package extraction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Qxy0happy/codegraph-go/internal/graph"
	"github.com/Qxy0happy/codegraph-go/internal/types"
)

// IndexConfig controls indexer behavior.
type IndexConfig struct {
	RootDir    string // Project root directory
	NumWorkers int    // Number of parallel parser goroutines (0 = GOMAXPROCS)
	Verbose    bool   // Print progress
}

// IndexResult summarizes an indexing run.
type IndexResult struct {
	FilesScanned int
	FilesIndexed int
	FilesErrored int
	NodesCreated int
	EdgesCreated int
	Duration     time.Duration
}

// IndexAll scans, parses, and indexes all source files in a project.
// Uses a goroutine pool for parallel tree-sitter parsing.
func IndexAll(ctx context.Context, cfg IndexConfig) (*IndexResult, error) {
	start := time.Now()

	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = runtime.NumCPU()
	}
	if cfg.RootDir == "" {
		var err error
		cfg.RootDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
	}

	// Phase 1: Scan for source files
	if cfg.Verbose {
		fmt.Printf("Scanning %s...\n", cfg.RootDir)
	}

	files, err := scanFiles(cfg.RootDir)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	if len(files) == 0 {
		return &IndexResult{FilesScanned: 0}, nil
	}
	if cfg.Verbose {
		fmt.Printf("Found %d source files\n", len(files))
	}

	// Phase 2: Parse files in parallel using goroutine pool
	fileChan := make(chan string, len(files))
	resultChan := make(chan *ExtractionResult, cfg.NumWorkers*2)
	errChan := make(chan error, cfg.NumWorkers)

	var wg sync.WaitGroup

	// Start parser workers
	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go parseWorker(ctx, fileChan, resultChan, &wg)
	}

	// Send files to workers (closing fileChan signals workers to stop)
	go func() {
		for _, f := range files {
			select {
			case fileChan <- f:
			case <-ctx.Done():
				return
			}
		}
		close(fileChan)
	}()

	// Close resultChan when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []*ExtractionResult
	for r := range resultChan {
		results = append(results, r)
	}

	// Check for worker errors (non-blocking)
	select {
	case err := <-errChan:
		if err != nil {
			return nil, err
		}
	default:
	}

	// Phase 3: Write to SQLite
	if cfg.Verbose {
		fmt.Printf("Writing %d results to database...\n", len(results))
	}

	db, err := openOrCreateDB(cfg.RootDir)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	totalNodes := 0
	totalEdges := 0
	indexed := 0
	errored := 0

	for _, r := range results {
		if r == nil {
			errored++
			continue
		}
		if len(r.Nodes) > 0 || len(r.Edges) > 0 {
			if err := storeResult(db, r); err != nil {
				if cfg.Verbose {
					fmt.Printf("  Failed to store %s: %v\n", r.File.Path, err)
				}
				errored++
				continue
			}
			indexed++
		}
		totalNodes += len(r.Nodes)
		totalEdges += len(r.Edges)
	}

	return &IndexResult{
		FilesScanned: len(files),
		FilesIndexed: indexed,
		FilesErrored: errored,
		NodesCreated: totalNodes,
		EdgesCreated: totalEdges,
		Duration:     time.Since(start),
	}, nil
}

// parseWorker reads file paths from fileChan, parses them, and sends
// results to resultChan. Multiple workers run in parallel.
func parseWorker(ctx context.Context, fileChan <-chan string, resultChan chan<- *ExtractionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		source, err := os.ReadFile(filePath)
		if err != nil {
			resultChan <- &ExtractionResult{
				File: types.File{Path: filePath},
			}
			continue
		}

		result, err := ExtractFile(filePath, source)
		if err != nil {
			resultChan <- &ExtractionResult{
				File: types.File{Path: filePath},
			}
			continue
		}

		resultChan <- result
	}
}

// scanFiles recursively finds all source files in rootDir.
func scanFiles(rootDir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if d.IsDir() {
			name := d.Name()
			// Skip common non-source directories
			if name == "node_modules" || name == ".git" || name == ".codegraph" ||
				name == "dist" || name == "build" || name == "target" ||
				name == ".venv" || name == "venv" || name == "__pycache__" ||
				name == "vendor" || name == ".next" || name == ".nuxt" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if SourceExtensions[filepath.Ext(path)] {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// openOrCreateDB opens (or creates) the CodeGraph database for writing.
func openOrCreateDB(rootDir string) (*graph.DB, error) {
	dbDir := filepath.Join(rootDir, ".codegraph")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("create .codegraph dir: %w", err)
	}

	dbPath := filepath.Join(dbDir, "codegraph.db")
	return graph.OpenRW(dbPath)
}

// storeResult writes extraction results to the database.
func storeResult(db *graph.DB, result *ExtractionResult) error {
	// Write nodes
	for _, n := range result.Nodes {
		if err := db.InsertNode(n); err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}
	// Write edges
	for _, e := range result.Edges {
		if err := db.InsertEdge(e); err != nil {
			return fmt.Errorf("insert edge: %w", err)
		}
	}
	// Write file record
	if err := db.UpsertFile(result.File); err != nil {
		return fmt.Errorf("upsert file: %w", err)
	}
	return nil
}

var _ = time.Second // suppress unused import error during dev
