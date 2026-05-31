// Package watcher provides file system watching for auto-reindexing.
package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/colbymchenry/codegraph-go/internal/extraction"
	"github.com/colbymchenry/codegraph-go/internal/graph"
)

// WatchConfig controls the file watcher behavior.
type WatchConfig struct {
	RootDir  string // Project root
	Debounce time.Duration
}

// Watch starts a file watcher that re-indexes changed source files.
// Blocks until ctx is cancelled or a fatal error occurs.
func Watch(ctx context.Context, cfg WatchConfig) error {
	if cfg.Debounce <= 0 {
		cfg.Debounce = 500 * time.Millisecond
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watcher: %w", err)
	}
	defer watcher.Close()

	// Walk the project tree and add directories
	if err := filepath.WalkDir(cfg.RootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == "node_modules" || name == ".codegraph" ||
			name == "dist" || name == "build" || name == "target" || name == "__pycache__" ||
			strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	}); err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	log.Printf("Watching %s for changes...\n", cfg.RootDir)

	// Debounce: batch file changes within the debounce window
	var pending map[string]struct{}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isSourceFile(event.Name) {
				continue
			}

			// Debounce: collect events, fire after quiet period
			if pending == nil {
				pending = make(map[string]struct{})
				time.AfterFunc(cfg.Debounce, func() {
					processChanges(ctx, cfg.RootDir, pending)
					pending = nil
				})
			}
			pending[event.Name] = struct{}{}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watch error: %v\n", err)

		case <-ctx.Done():
			return nil
		}
	}
}

// processChanges re-indexes all changed files.
func processChanges(ctx context.Context, rootDir string, files map[string]struct{}) {
	db, err := graph.OpenRW(filepath.Join(rootDir, ".codegraph", "codegraph.db"))
	if err != nil {
		log.Printf("Open DB: %v\n", err)
		return
	}
	defer db.Close()

	for path := range files {
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			continue
		}

		source, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		result, err := extraction.ExtractFile(rel, source)
		if err != nil || result == nil {
			continue
		}

		for _, n := range result.Nodes {
			db.InsertNode(n)
		}
		for _, e := range result.Edges {
			db.InsertEdge(e)
		}
		log.Printf("  Re-indexed %s (%d nodes, %d edges)\n", rel, len(result.Nodes), len(result.Edges))
	}
}

func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".ts", ".tsx", ".js", ".mjs", ".cjs", ".jsx",
		".go", ".py", ".rs", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".php", ".rb", ".swift", ".kt", ".dart",
		".scala", ".lua", ".m", ".mm", ".yml", ".yaml",
		".svelte", ".vue":
		return true
	}
	return false
}
