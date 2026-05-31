// Package sync provides git integration for incremental re-indexing.
package sync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Qxy0happy/codegraph-go/internal/extraction"
	"github.com/Qxy0happy/codegraph-go/internal/graph"
)

// SyncResult summarizes a sync operation.
type SyncResult struct {
	FilesChecked int
	FilesChanged int
	NodesUpdated int
	EdgesUpdated int
}

// SyncChanged re-indexes files that have changed since the last commit.
func SyncChanged(rootDir string) (*SyncResult, error) {
	db, err := graph.OpenRW(filepath.Join(rootDir, ".codegraph", "codegraph.db"))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Get changed files from git
	changedFiles, err := getGitChangedFiles(rootDir)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	if len(changedFiles) == 0 {
		return &SyncResult{}, nil
	}

	result := &SyncResult{FilesChecked: len(changedFiles)}

	for _, relPath := range changedFiles {
		fullPath := filepath.Join(rootDir, relPath)

		source, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		extracted, err := extraction.ExtractFile(relPath, source)
		if err != nil || extracted == nil {
			continue
		}

		result.FilesChanged++
		result.NodesUpdated += len(extracted.Nodes)
		result.EdgesUpdated += len(extracted.Edges)

		for _, n := range extracted.Nodes {
			db.InsertNode(n)
		}
		for _, e := range extracted.Edges {
			db.InsertEdge(e)
		}
	}

	return result, nil
}

// InstallHooks installs CodeGraph git hooks into the project.
func InstallHooks(rootDir string) error {
	hooksDir := filepath.Join(rootDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("hooks dir: %w", err)
	}

	hooks := []struct {
		name    string
		content string
	}{
		{
			name: "post-merge",
			content: `#!/bin/sh
# CodeGraph: re-index after pull/merge
exec codegraph-go sync
`,
		},
		{
			name: "post-commit",
			content: `#!/bin/sh
# CodeGraph: re-index after commit
exec codegraph-go sync
`,
		},
		{
			name: "pre-commit",
			content: `#!/bin/sh
# CodeGraph: ensure the graph is up-to-date before commit
codegraph-go sync > /dev/null 2>&1
`,
		},
	}

	for _, h := range hooks {
		path := filepath.Join(hooksDir, h.name)
		if err := os.WriteFile(path, []byte(h.content), 0755); err != nil {
			return fmt.Errorf("write %s hook: %w", h.name, err)
		}
		fmt.Printf("  Installed %s hook\n", h.name)
	}

	return nil
}

// getGitChangedFiles returns files changed since the last commit.
// Falls back to files changed in the working tree.
func getGitChangedFiles(rootDir string) ([]string, error) {
	// Try: files changed in the last commit
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=ACM", "HEAD~1..HEAD")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return filterSourceFiles(strings.Fields(string(out)))
	}

	// Fallback: staged + unstaged changes
	cmd = exec.Command("git", "status", "--porcelain", "--no-renames")
	cmd.Dir = rootDir
	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 4 {
			continue
		}
		// Status format: "XY filename"
		status := line[:2]
		filename := strings.TrimSpace(line[2:])
		if strings.Contains(status, "?") {
			files = append(files, filename) // untracked
		} else if !strings.Contains(status, "D") {
			files = append(files, filename) // modified or added
		}
	}

	return filterSourceFiles(files)
}

func filterSourceFiles(files []string) ([]string, error) {
	var out []string
	exts := extraction.SourceExtensions
	for _, f := range files {
		// Skip dir entries (trailing /)
		if strings.HasSuffix(f, "/") {
			continue
		}
		ext := filepath.Ext(f)
		if exts[ext] {
			out = append(out, f)
		}
	}
	return out, nil
}

// ensure exec is imported (used by exec.Command)
var _ = bytes.Buffer{}
