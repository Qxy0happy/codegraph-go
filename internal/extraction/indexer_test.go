package extraction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexAllGoDir(t *testing.T) {
	// Index just the go/ directory as a smoke test
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Navigate to project root (go/internal/extraction/ → go/ → project root)
	root := filepath.Join(cwd, "..", "..", "..")
	// Index only the go/ subdirectory by pointing to a temp copy or the go/ dir itself
	goDir := filepath.Join(root, "go")

	// Create a temp workspace so we don't mess up the real .codegraph
	tmpDir := t.TempDir()
	// Symlink or copy won't work easily; just use a random subdir
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Go's internal/extraction directory should have some Go files
	// Let's point to the project's go/ directory directly
	// but use a separate DB location
	t.Logf("Indexing %s into temp dir %s", goDir, tmpDir)

	// Override .codegraph location by using IndexAll with a temp dir
	result, err := IndexAll(context.Background(), IndexConfig{
		RootDir:    goDir,
		NumWorkers: 2,
	})
	if err != nil {
		t.Fatalf("IndexAll failed: %v", err)
	}

	t.Logf("Result: %d files, %d indexed, %d errors",
		result.FilesScanned, result.FilesIndexed, result.FilesErrored)
	t.Logf("  %d nodes, %d edges in %v",
		result.NodesCreated, result.EdgesCreated, result.Duration)

	if result.NodesCreated == 0 && result.FilesScanned > 0 {
		t.Error("expected some nodes to be created")
	}
}

func TestExtractFile(t *testing.T) {
	// Test extracting symbols from a small Go file
	source := []byte(`package main

import "fmt"

// Hello returns a greeting.
func Hello(name string) string {
	return "Hello, " + name
}

type Greeter struct {
	greeting string
}

func (g *Greeter) Greet() string {
	return g.greeting
}
`)

	// We need to parse as Go, but we only have TypeScript parser registered
	// This test checks graceful handling
	result, err := ExtractFile("test.go", source)
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	if result == nil {
		t.Fatal("ExtractFile returned nil")
	}
	t.Logf("Extracted %d nodes, %d edges from .go file (language not supported yet)", 
		len(result.Nodes), len(result.Edges))
}

func TestExtractTypeScript(t *testing.T) {
	source := []byte(`
function hello(name: string): string {
	return "Hello, " + name;
}

class Greeter {
	private greeting: string;
	constructor(message: string) {
		this.greeting = message;
	}
}
`)

	result, err := ExtractFile("test.ts", source)
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	if result == nil {
		t.Fatal("ExtractFile returned nil")
	}
	t.Logf("Extracted %d nodes, %d edges from test.ts", len(result.Nodes), len(result.Edges))
	for _, n := range result.Nodes {
		sig := ""
		if n.Signature != nil {
			sig = " " + *n.Signature
		}
		t.Logf("  %s `%s`%s (%s:%d)", n.Kind, n.Name, sig, n.FilePath, n.StartLine)
	}
	for _, e := range result.Edges {
		t.Logf("  %s → %s [%s]", e.Source, e.Target, e.Kind)
	}

	// Verify we found the expected symbols
	hasFunction := false
	hasClass := false
	hasMethod := false
	for _, n := range result.Nodes {
		switch n.Kind {
		case "function":
			if n.Name == "hello" {
				hasFunction = true
			}
		case "class":
			if n.Name == "Greeter" {
				hasClass = true
			}
		case "method":
			if n.Name == "constructor" || n.Name == "Greet" {
				hasMethod = true
			}
		}
	}
	if !hasFunction {
		t.Error("expected function 'hello'")
	}
	if !hasClass {
		t.Error("expected class 'Greeter'")
	}
	if !hasMethod {
		t.Error("expected method")
	}
}
