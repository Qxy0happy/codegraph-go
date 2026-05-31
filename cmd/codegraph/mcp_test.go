package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colbymchenry/codegraph-go/internal/graph"
	"github.com/colbymchenry/codegraph-go/internal/mcp"
)

func findRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".codegraph")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func openDB(t *testing.T) *graph.DB {
	t.Helper()
	root := findRoot()
	dbPath := filepath.Join(root, ".codegraph", "codegraph.db")
	db, err := graph.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMCPServerMinimal(t *testing.T) {
	// Minimal: just test initialize
	db := openDB(t)
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
`
	var stdout bytes.Buffer
	server := mcp.NewServerWithIO(db, strings.NewReader(req), &stdout)
	server.Run()

	output := stdout.String()
	if output == "" {
		t.Fatal("empty output from server")
	}
	t.Logf("Output: %s", output)

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  map[string]interface{} `json:"result,omitempty"`
		Error   *struct { Code int; Message string } `json:"error,omitempty"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\n%s", err, output)
	}
	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %d %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result == nil {
		t.Errorf("expected result, got error: %+v", resp.Error)
	}
}

func TestMCPServerFull(t *testing.T) {
	db := openDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/list"}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"codegraph_search","arguments":{"query":"Open","limit":3}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"codegraph_node","arguments":{"symbol":"file:go/internal/graph/db.go"}}}
`

	stdinR := strings.NewReader(requests)
	var stdout bytes.Buffer

	server := mcp.NewServerWithIO(db, stdinR, &stdout)
	err := server.Run()
	if err != nil {
		t.Fatalf("server run: %v", err)
	}

	// Parse each line as JSON-RPC response
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 4 {
		t.Errorf("expected ≥4 responses, got %d", len(lines))
		t.Logf("Full output:\n%s", stdout.String())
		return
	}

	for i, line := range lines {
		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Errorf("line %d: invalid JSON: %v\n  %s", i, err, line)
			continue
		}
		if resp.JSONRPC != "2.0" {
			t.Errorf("line %d: expected jsonrpc 2.0, got %s", i, resp.JSONRPC)
		}
		if resp.Error != nil {
			t.Errorf("line %d: error: %d %s", i, resp.Error.Code, resp.Error.Message)
		}
	}

	t.Logf("Full output (%d lines):\n%s", len(lines), stdout.String())
}

func TestMCPToolSearch(t *testing.T) {
	db := openDB(t)

	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"codegraph_search","arguments":{"query":"indexAll","limit":5}}}
`

	stdinR := strings.NewReader(requests)
	var stdout bytes.Buffer
	server := mcp.NewServerWithIO(db, stdinR, &stdout)
	server.Run()

	output := stdout.String()
	if !strings.Contains(strings.ToLower(output), "indexall") {
		t.Errorf("expected search results for 'indexAll', got:\n%s", output)
	}
}

func TestMCPToolCallers(t *testing.T) {
	db := openDB(t)

	// First find a node ID via search
	searchReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"codegraph_search","arguments":{"query":"requestParse","limit":3}}}
`
	var stdoutBuf bytes.Buffer
	server := mcp.NewServerWithIO(db, strings.NewReader(searchReq), &stdoutBuf)
	server.Run()

}

func TestNodeCount(t *testing.T) {
	db := openDB(t)
	count, err := db.GetNodeCount()
	if err != nil {
		t.Fatalf("get node count: %v", err)
	}
	if count == 0 {
		t.Error("expected non-zero node count")
	}
	t.Logf("Node count: %d", count)
}

func TestSearch(t *testing.T) {
	db := openDB(t)
	nodes, err := db.SearchNodes("indexAll", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected results for 'indexAll'")
	}
	t.Logf("Found %d results for 'indexAll'", len(nodes))
	for _, n := range nodes {
		t.Logf("  %s  `%s`  (%s:%d)", n.Kind, n.Name, n.FilePath, n.StartLine)
	}
}

func TestCallers(t *testing.T) {
	db := openDB(t)
	nodes, err := db.SearchNodes("requestParse", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(nodes) == 0 {
		t.Skip("no 'requestParse' symbol found")
	}

	edges, err := db.GetCallers(nodes[0].ID)
	if err != nil {
		t.Fatalf("get callers: %v", err)
	}
	t.Logf("Callers of %s (`%s`): %d", nodes[0].Name, nodes[0].ID, len(edges))
	for _, e := range edges {
		caller, _ := db.GetNodeByID(e.Source)
		if caller != nil {
			t.Logf("  %s `%s`", caller.Kind, caller.Name)
		}
	}
}

func TestBFS(t *testing.T) {
	db := openDB(t)

	// Find the `indexAll` method in the orchestrator (not the wrapper in src/index.ts)
	nodes, err := db.SearchNodes("requestParse", 3)
	if err != nil || len(nodes) == 0 {
		t.Skip("no 'requestParse' symbol found")
	}

	// BFS traversal: find callees
	steps, err := db.GetCalleesBFS(nodes[0].ID, 2)
	if err != nil {
		t.Fatalf("BFS callees: %v", err)
	}
	t.Logf("BFS callees of '%s' (depth 2): %d steps", nodes[0].Name, len(steps))
	for _, s := range steps {
		t.Logf("  [%s] `%s` (%s:%d)", s.Edge.Kind, s.Node.Name, s.Node.FilePath, s.Node.StartLine)
	}

	// Also test findPath if both symbols exist
	if len(nodes) >= 2 {
		path, err := db.FindPath(nodes[0].ID, nodes[1].ID, nil)
		if err != nil {
			t.Logf("FindPath error (expected for unrelated symbols): %v", err)
		}
		if path != nil {
			t.Logf("Path found: %d hops", len(path.Steps))
		}
	}

}
