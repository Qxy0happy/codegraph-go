# CodeGraph-Go

Go port of [CodeGraph](https://github.com/colbymchenry/codegraph) — a local-first semantic code intelligence engine.

Indexes source code into a SQLite knowledge graph and serves it via MCP (Model Context Protocol) for AI agents to query.

## Quick Start

```bash
# Build (requires zig cc for CGo)
CC="zig cc" CGO_ENABLED=1 go build -o codegraph-go ./cmd/codegraph/

# Index a project
./codegraph-go init

# Start MCP server
./codegraph-go serve

# Watch for changes and auto-reindex
./codegraph-go watch
```

## Commands

| Command | Description |
|---|---|
| `init` | Index the current project |
| `serve` | Start MCP stdio server |
| `watch` | Watch files and auto-reindex |
| `sync` | Re-index changed files (git diff) |
| `install-hooks` | Install git hooks for auto-sync |
| `status` | Show database stats |

## Architecture

```
Source files → tree-sitter (CGo) → Node/Edge extraction → SQLite (FTS5)
                                                                   ↓
AI Agent ← MCP stdio ← Context builder ← Graph traversal ← DB queries
```

- **tree-sitter** via `smacker/go-tree-sitter` (CGo compiled with zig cc)
- **SQLite** via `modernc.org/sqlite` (pure Go, FTS5 full-text search)
- **MCP** JSON-RPC 2.0 over stdio
- **Parallel indexing** with goroutine pool (`runtime.NumCPU()` workers)
- **Supported languages:** TypeScript, Go (extensible via LanguageExtractor)

## Current Status

Fully functional for querying existing CodeGraph databases and indexing new projects.
