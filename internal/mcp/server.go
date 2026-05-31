// Package mcp implements a JSON-RPC 2.0 MCP server over stdio.
//
// Model Context Protocol (MCP) enables AI agents to discover and call
// tools. This server provides the CodeGraph tool surface: search, trace,
// explore, callers/callees, context, and impact analysis.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Qxy0happy/codegraph-go/internal/graph"
)

// Server is an MCP server that provides code intelligence tools.
type Server struct {
	db     *graph.DB
	reader *bufio.Scanner
	writer io.Writer
}

// NewServer creates a new MCP server backed by a CodeGraph database.
// Reads JSON-RPC from stdin and writes responses to stdout.
func NewServer(db *graph.DB) *Server {
	return NewServerWithIO(db, os.Stdin, os.Stdout)
}

// NewServerWithIO creates an MCP server with custom I/O streams (for testing).
func NewServerWithIO(db *graph.DB, r io.Reader, w io.Writer) *Server {
	return &Server{
		db:     db,
		reader: bufio.NewScanner(r),
		writer: w,
	}
}

// Run starts the MCP server loop, reading JSON-RPC messages from stdin.
func (s *Server) Run() error {
	log.SetOutput(os.Stderr) // logs go to stderr, JSON-RPC to stdout

	for s.reader.Scan() {
		line := s.reader.Text()
		if line == "" {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			continue
		}

		s.handle(req)
	}
	return s.reader.Err()
}

// ---------------------------------------------------------------------------
// JSON-RPC types
// ---------------------------------------------------------------------------

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

// MCP tool definition
type toolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// MCP tool call result content item
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ---------------------------------------------------------------------------
// Request dispatch
// ---------------------------------------------------------------------------

func (s *Server) handle(req jsonrpcRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// No response needed
	case "tools/list":
		s.handleToolList(req)
	case "tools/call":
		s.handleToolCall(req)
	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleInitialize(req jsonrpcRequest) {
	s.sendResult(req.ID, map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "codegraph-go",
			"version": "0.1.0",
		},
	})
}

func (s *Server) handleToolList(req jsonrpcRequest) {
	s.sendResult(req.ID, map[string]interface{}{
		"tools": s.toolDefinitions(),
	})
}

func (s *Server) handleToolCall(req jsonrpcRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params", nil)
		return
	}

	var result *toolCallResult
	var err error

	switch params.Name {
	case "codegraph_search":
		result, err = s.handleSearch(params.Arguments)
	case "codegraph_context":
		result, err = s.handleContext(params.Arguments)
	case "codegraph_node":
		result, err = s.handleNode(params.Arguments)
	case "codegraph_explore":
		result, err = s.handleExplore(params.Arguments)
	case "codegraph_trace":
		result, err = s.handleTrace(params.Arguments)
	case "codegraph_callers":
		result, err = s.handleCallers(params.Arguments)
	case "codegraph_callees":
		result, err = s.handleCallees(params.Arguments)
	default:
		s.sendError(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
		return
	}

	if err != nil {
		s.sendResult(req.ID, toolCallResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		})
		return
	}
	s.sendResult(req.ID, result)
}

// ---------------------------------------------------------------------------
// Send helpers
// ---------------------------------------------------------------------------

func (s *Server) sendResult(id *json.RawMessage, result interface{}) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(s.writer, string(b))
}

func (s *Server) sendError(id *json.RawMessage, code int, message string, data interface{}) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(s.writer, string(b))
}

// ---------------------------------------------------------------------------
// Tool definitions
// ---------------------------------------------------------------------------

func (s *Server) toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "codegraph_search",
			Description: "Search for symbols by name or pattern across the codebase. Returns matching nodes with file locations.\n\nParameters:\n- query (required): Symbol name or pattern (supports FTS5 syntax like prefix matching)\n- kind (optional): Filter by node kind (function, class, method, etc.)\n- limit (optional): Maximum results (default 10)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Symbol name or search query"},
					"kind":  map[string]interface{}{"type": "string", "description": "Filter by node kind"},
					"limit": map[string]interface{}{"type": "number", "description": "Max results (default 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "codegraph_node",
			Description: "Get detailed information about a single symbol by its node ID.\n\nParameters:\n- symbol (required): Node ID of the symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{"type": "string", "description": "Node ID"},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "codegraph_callers",
			Description: "Find all callers of a function or method (what calls it).\n\nParameters:\n- symbol (required): Node ID of the function/method",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{"type": "string", "description": "Node ID"},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "codegraph_callees",
			Description: "Find all functions/methods called by a given function or method.\n\nParameters:\n- symbol (required): Node ID of the function/method",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{"type": "string", "description": "Node ID"},
				},
				"required": []string{"symbol"},
			},
		},
		{
			Name:        "codegraph_trace",
			Description: "Find a call path between two symbols — how does <from> reach <to>?\n\nParameters:\n- from (required): Starting symbol node ID\n- to (required): Target symbol node ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from": map[string]interface{}{"type": "string", "description": "Starting node ID"},
					"to":   map[string]interface{}{"type": "string", "description": "Target node ID"},
				},
				"required": []string{"from", "to"},
			},
		},
		{
			Name:        "codegraph_explore",
			Description: "Explore several related symbols, showing their source code grouped by file.\n\nParameters:\n- query (required): Symbol/file names to explore",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Symbol names, file names"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "codegraph_context",
			Description: "Build comprehensive context for a task — searches symbols, returns related definitions, callers, and callees combined.\n\nParameters:\n- task (required): Description of the task or feature to build context for\n- includeCode (optional): Include source code (default true)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task":        map[string]interface{}{"type": "string", "description": "Task description"},
					"includeCode": map[string]interface{}{"type": "boolean", "description": "Include source code"},
				},
				"required": []string{"task"},
			},
		},
	}
}
