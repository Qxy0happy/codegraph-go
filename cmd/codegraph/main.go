package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Qxy0happy/codegraph-go/internal/extraction"
	"github.com/Qxy0happy/codegraph-go/internal/graph"
	"github.com/Qxy0happy/codegraph-go/internal/mcp"
	"github.com/Qxy0happy/codegraph-go/internal/sync"
	"github.com/Qxy0happy/codegraph-go/internal/types"
	"github.com/Qxy0happy/codegraph-go/internal/watcher"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "serve", "server":
		runServer()
	case "init", "index":
		runIndex()
	case "watch":
		runWatch()
	case "sync":
		runSync()
	case "install-hooks":
		runInstallHooks()
	case "status":
		runStatus()
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `CodeGraph-Go — Go port of CodeGraph

Usage:
  codegraph-go init           Index the current project
  codegraph-go serve          Start MCP server (stdio JSON-RPC)
  codegraph-go watch          Watch files and auto-reindex
  codegraph-go sync           Re-index changed files (git diff)
  codegraph-go install-hooks  Install git hooks for auto-sync
  codegraph-go status         Show database stats

`)
}

func runIndex() {
	root, err := findCodegraphRoot()
	if err != nil {
		// No .codegraph yet — use cwd
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Indexing %s...\n", root)

	result, err := extraction.IndexAll(context.Background(), extraction.IndexConfig{
		RootDir:    root,
		NumWorkers: 0, // auto = GOMAXPROCS
		Verbose:    true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Indexing failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone! %d files scanned, %d indexed, %d errors\n",
		result.FilesScanned, result.FilesIndexed, result.FilesErrored)
	fmt.Printf("  %d nodes, %d edges created in %v\n",
		result.NodesCreated, result.EdgesCreated, result.Duration)
}

func runWatch() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Watching %s...\n", root)
	if err := watcher.Watch(ctx, watcher.WatchConfig{
		RootDir:  root,
		Debounce: 500 * time.Millisecond,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		os.Exit(1)
	}
}

func runSync() {
	root, err := findCodegraphRoot()
	if err != nil {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	result, err := sync.SyncChanged(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sync failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Synced %d files, %d changed, %d nodes, %d edges\n",
		result.FilesChecked, result.FilesChanged,
		result.NodesUpdated, result.EdgesUpdated)
}

func runInstallHooks() {
	root, err := findCodegraphRoot()
	if err != nil {
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Installing CodeGraph hooks in %s...\n", root)
	if err := sync.InstallHooks(root); err != nil {
		fmt.Fprintf(os.Stderr, "Install hooks failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Hooks installed.")
}

func runServer() {
	root, err := findCodegraphRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(root, ".codegraph", "codegraph.db")
	db, err := graph.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	server := mcp.NewServer(db)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runStatus() {
	root, err := findCodegraphRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(root, ".codegraph", "codegraph.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No CodeGraph database found at %s\n", dbPath)
		fmt.Fprintf(os.Stderr, "Run 'codegraph init -i' first to index a project.\n")
		os.Exit(1)
	}

	db, err := graph.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	nodeCount, _ := db.GetNodeCount()
	edgeCount, _ := db.GetEdgeCount()
	fmt.Printf("CodeGraph Database: %s\n", dbPath)
	fmt.Printf("  Nodes: %d\n", nodeCount)
	fmt.Printf("  Edges: %d\n\n", edgeCount)

	fmt.Printf("Node kinds:\n")
	for _, kindStr := range []string{
		"file", "function", "method", "class", "interface",
		"struct", "enum", "variable", "constant", "import",
	} {
		nodes, err := db.GetNodesByKind(types.NodeKind(kindStr))
		if err != nil {
			continue
		}
		if len(nodes) > 0 {
			fmt.Printf("  %-12s %d\n", kindStr+":", len(nodes))
		}
	}

	if nodeCount > 0 {
		fmt.Printf("\nReady to serve: codegraph-go serve\n")
	}
}

func findCodegraphRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".codegraph")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .codegraph directory found in any parent")
		}
		dir = parent
	}
}
