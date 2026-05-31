package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/colbymchenry/codegraph-go/internal/types"
)

// ---------------------------------------------------------------------------
// codegraph_search
// ---------------------------------------------------------------------------

type searchArgs struct {
	Query string `json:"query"`
	Kind  string `json:"kind,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func (s *Server) handleSearch(raw json.RawMessage) (*toolCallResult, error) {
	var args searchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid search arguments: %w", err)
	}
	if args.Query == "" {
		return nil, fmt.Errorf("search requires a non-empty 'query'")
	}
	if args.Limit <= 0 || args.Limit > 100 {
		args.Limit = 10
	}

	// Search via FTS5
	nodes, err := s.db.SearchNodes(args.Query, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter by kind if specified
	if args.Kind != "" {
		var filtered []types.Node
		for _, n := range nodes {
			if string(n.Kind) == args.Kind {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	if len(nodes) == 0 {
		return &toolCallResult{
			Content: []contentItem{{Type: "text", Text: "No results found."}},
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d result(s):\n\n", len(nodes))
	for _, n := range nodes {
		fmt.Fprintf(&b, "  %s  `%s`  (%s)\n", n.Kind, n.Name, n.FilePath)
		fmt.Fprintf(&b, "         ID: %s\n", n.ID)
		if n.Signature != nil {
			fmt.Fprintf(&b, "         Signature: %s\n", *n.Signature)
		}
		fmt.Fprintf(&b, "         Lines %d-%d\n\n", n.StartLine, n.EndLine)
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_node
// ---------------------------------------------------------------------------

type nodeArgs struct {
	Symbol string `json:"symbol"`
}

func (s *Server) handleNode(raw json.RawMessage) (*toolCallResult, error) {
	var args nodeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid node arguments: %w", err)
	}
	if args.Symbol == "" {
		return nil, fmt.Errorf("node requires a non-empty 'symbol'")
	}

	node, err := s.db.GetNodeByID(args.Symbol)
	if err != nil {
		return nil, fmt.Errorf("node not found: %s", args.Symbol)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Node: %s\n", node.Name)
	fmt.Fprintf(&b, "  ID:           %s\n", node.ID)
	fmt.Fprintf(&b, "  Kind:         %s\n", node.Kind)
	fmt.Fprintf(&b, "  File:         %s\n", node.FilePath)
	fmt.Fprintf(&b, "  Language:     %s\n", node.Language)
	fmt.Fprintf(&b, "  Qualified:    %s\n", node.QualifiedName)
	if node.Signature != nil {
		fmt.Fprintf(&b, "  Signature:    %s\n", *node.Signature)
	}
	if node.Docstring != nil {
		fmt.Fprintf(&b, "  Docstring:    %s\n", *node.Docstring)
	}
	fmt.Fprintf(&b, "  Lines:        %d-%d\n", node.StartLine, node.EndLine)
	fmt.Fprintf(&b, "  Exported:     %t\n", node.IsExported)
	fmt.Fprintf(&b, "  Async:        %t\n", node.IsAsync)
	fmt.Fprintf(&b, "  Static:       %t\n", node.IsStatic)

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_callers
// ---------------------------------------------------------------------------

type callerArgs struct {
	Symbol string `json:"symbol"`
}

func (s *Server) handleCallers(raw json.RawMessage) (*toolCallResult, error) {
	var args callerArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid callers arguments: %w", err)
	}
	if args.Symbol == "" {
		return nil, fmt.Errorf("callers requires a non-empty 'symbol'")
	}

	node, err := s.db.GetNodeByID(args.Symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", args.Symbol)
	}

	edges, err := s.db.GetCallers(args.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get callers: %w", err)
	}

	if len(edges) == 0 {
		return &toolCallResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("No callers found for `%s`.", node.Name)}},
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Callers of `%s` (%d):\n\n", node.Name, len(edges))
	for _, e := range edges {
		caller, err := s.db.GetNodeByID(e.Source)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "  %s  `%s`  (%s:%d)\n", caller.Kind, caller.Name, caller.FilePath, e.Line)
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_callees
// ---------------------------------------------------------------------------

type calleeArgs struct {
	Symbol string `json:"symbol"`
}

func (s *Server) handleCallees(raw json.RawMessage) (*toolCallResult, error) {
	var args calleeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid callees arguments: %w", err)
	}
	if args.Symbol == "" {
		return nil, fmt.Errorf("callees requires a non-empty 'symbol'")
	}

	node, err := s.db.GetNodeByID(args.Symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", args.Symbol)
	}

	edges, err := s.db.GetCallees(args.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get callees: %w", err)
	}

	if len(edges) == 0 {
		return &toolCallResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("No callees found for `%s`.", node.Name)}},
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Callees of `%s` (%d):\n\n", node.Name, len(edges))
	for _, e := range edges {
		callee, err := s.db.GetNodeByID(e.Target)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "  %s  `%s`  (%s:%d)\n", callee.Kind, callee.Name, callee.FilePath, e.Line)
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_trace
// ---------------------------------------------------------------------------

type traceArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) handleTrace(raw json.RawMessage) (*toolCallResult, error) {
	var args traceArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid trace arguments: %w", err)
	}
	if args.From == "" || args.To == "" {
		return nil, fmt.Errorf("trace requires both 'from' and 'to'")
	}

	path, err := s.db.FindPath(args.From, args.To, nil)
	if err != nil {
		return nil, fmt.Errorf("trace failed: %w", err)
	}

	if path == nil {
		// No static path found - show both endpoints
		fromNode, _ := s.db.GetNodeByID(args.From)
		toNode, _ := s.db.GetNodeByID(args.To)

		var b strings.Builder
		fmt.Fprintf(&b, "No static call path found between `%s` and `%s`.\n", args.From, args.To)
		fmt.Fprintf(&b, "The chain probably breaks at dynamic dispatch.\n\n")
		if fromNode != nil {
			fmt.Fprintf(&b, "FROM: `%s` (%s:%d)\n", fromNode.Name, fromNode.FilePath, fromNode.StartLine)
		}
		if toNode != nil {
			fmt.Fprintf(&b, "TO: `%s` (%s:%d)\n", toNode.Name, toNode.FilePath, toNode.StartLine)
		}
		return &toolCallResult{
			Content: []contentItem{{Type: "text", Text: b.String()}},
		}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Call path (%d hops):\n\n", len(path.Steps))
	for i, step := range path.Steps {
		connector := "→"
		if i > 0 {
			fmt.Fprintf(&b, "  %s\n", connector)
		}
		fmt.Fprintf(&b, "  %d. %s  `%s`  (%s:%d)\n", i+1, step.Node.Kind, step.Node.Name, step.Node.FilePath, step.Node.StartLine)
		if step.Node.Signature != nil {
			fmt.Fprintf(&b, "     %s\n", *step.Node.Signature)
		}
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_explore (simple: search by query, return grouped by file)
// ---------------------------------------------------------------------------

type exploreArgs struct {
	Query string `json:"query"`
}

func (s *Server) handleExplore(raw json.RawMessage) (*toolCallResult, error) {
	var args exploreArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid explore arguments: %w", err)
	}
	if args.Query == "" {
		return nil, fmt.Errorf("explore requires a non-empty 'query'")
	}

	nodes, err := s.db.SearchNodes(args.Query, 50)
	if err != nil {
		return nil, fmt.Errorf("explore search failed: %w", err)
	}

	if len(nodes) == 0 {
		return &toolCallResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("No symbols found matching '%s'.", args.Query)}},
		}, nil
	}

	// Group by file
	byFile := make(map[string][]types.Node)
	for _, n := range nodes {
		byFile[n.FilePath] = append(byFile[n.FilePath], n)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Explore results for '%s' (%d symbols in %d files):\n\n", args.Query, len(nodes), len(byFile))
	for filePath, syms := range byFile {
		fmt.Fprintf(&b, "📄 %s\n", filePath)
		for _, n := range syms {
			extra := ""
			if n.Signature != nil {
				extra = " " + *n.Signature
			}
			fmt.Fprintf(&b, "   %s  `%s`%s  (L%d-%d)\n", n.Kind, n.Name, extra, n.StartLine, n.EndLine)
		}
		fmt.Fprintf(&b, "\n")
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}

// ---------------------------------------------------------------------------
// codegraph_context (search + related symbols)
// ---------------------------------------------------------------------------

type contextArgs struct {
	Task        string `json:"task"`
	IncludeCode *bool  `json:"includeCode,omitempty"`
}

func (s *Server) handleContext(raw json.RawMessage) (*toolCallResult, error) {
	var args contextArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid context arguments: %w", err)
	}
	if args.Task == "" {
		return nil, fmt.Errorf("context requires a non-empty 'task'")
	}

	includeCode := true
	if args.IncludeCode != nil {
		includeCode = *args.IncludeCode
	}

	// Extract the key symbols from the task description and search
	words := strings.Fields(args.Task)
	var symbols []types.Node
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) < 3 {
			continue
		}
		nodes, err := s.db.SearchNodes(w, 5)
		if err != nil {
			continue
		}
		symbols = append(symbols, nodes...)
		if len(symbols) >= 15 {
			break
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []types.Node
	for _, n := range symbols {
		if !seen[n.ID] {
			seen[n.ID] = true
			unique = append(unique, n)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Context for: %s\n\n", args.Task)

	if len(unique) == 0 {
		fmt.Fprintf(&b, "No relevant symbols found.\n\n")
		fmt.Fprintf(&b, "Try a more specific query with codegraph_search first.\n")
	} else {
		fmt.Fprintf(&b, "Related symbols (%d):\n", len(unique))
		for _, n := range unique {
			fmt.Fprintf(&b, "  %s  `%s`  (%s:%d)\n", n.Kind, n.Name, n.FilePath, n.StartLine)
			if includeCode {
				// Try to get callers/callees for this symbol
				callers, _ := s.db.GetCallers(n.ID)
				callees, _ := s.db.GetCallees(n.ID)
				if len(callers) > 0 {
					fmt.Fprintf(&b, "       callers: %d\n", len(callers))
				}
				if len(callees) > 0 {
					fmt.Fprintf(&b, "       callees: %d\n", len(callees))
				}
			}
		}
	}

	return &toolCallResult{
		Content: []contentItem{{Type: "text", Text: b.String()}},
	}, nil
}
