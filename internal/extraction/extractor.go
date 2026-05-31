package extraction

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Qxy0happy/codegraph-go/internal/types"
)

// Extractor extracts CodeGraph symbols and edges from source code.
// This is a simplified version of the TypeScript LanguageExtractor pattern.
type Extractor struct {
	FilePath string
	Language string
	Source   []byte

	nodes []types.Node
	edges []types.Edge

	// Stack of parent node IDs for building contains edges
	scope []string
}

// ExtractionResult holds the output of a single file extraction.
type ExtractionResult struct {
	Nodes []types.Node
	Edges []types.Edge
	File  types.File
}

// ExtractFile extracts symbols and edges from source code.
func ExtractFile(filePath string, source []byte) (*ExtractionResult, error) {
	lang := DetectLanguage(filePath)
	if lang == "" {
		return &ExtractionResult{}, nil
	}

	// Parse with tree-sitter
	root, err := Parse(source, lang)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	ex := &Extractor{
		FilePath: filePath,
		Language: lang,
		Source:   source,
	}

	// Create file node
	fileNode := types.Node{
		ID:            fmt.Sprintf("file:%s", filePath),
		Kind:          types.NodeKindFile,
		Name:          filepath.Base(filePath),
		QualifiedName: filePath,
		FilePath:      filePath,
		Language:      types.Language(lang),
		StartLine:     1,
		EndLine:       root.EndLine,
		UpdatedAt:     now(),
	}
	ex.nodes = append(ex.nodes, fileNode)
	ex.scope = append(ex.scope, fileNode.ID)

	// Visit top-level children
	for _, child := range root.Children {
		ex.visitNode(&child)
	}

	return &ExtractionResult{
		Nodes: ex.nodes,
		Edges: ex.edges,
		File: types.File{
			Path:      filePath,
			Language:  lang,
			NodeCount: len(ex.nodes),
		},
	}, nil
}

// visitNode dispatches on CST node type and extracts symbols.
func (ex *Extractor) visitNode(node *Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case "function_declaration":
		ex.extractFunction(node)
	case "method_definition":
		ex.extractMethod(node)
	case "class_declaration", "abstract_class_declaration":
		ex.extractClass(node)
	case "interface_declaration":
		ex.extractInterface(node)
	case "enum_declaration":
		ex.extractEnum(node)
	case "method_declaration":
		ex.extractMethod(node)
	case "import_statement", "import_declaration":
		ex.extractImport(node)
	case "call_expression":
		ex.extractCall(node)
	case "lexical_declaration":
		ex.extractVariable(node)
	case "variable_declaration":
		ex.extractVariable(node)
	case "export_statement":
		ex.visitChildren(node)
		return
	case "program", "statement_block", "class_body", "interface_body", "source_file":
		ex.visitChildren(node)
		return
	}
}

func (ex *Extractor) visitChildren(node *Node) {
	for i := range node.Children {
		ex.visitNode(&node.Children[i])
	}
}

func (ex *Extractor) extractFunction(node *Node) {
	if node.Name == "" {
		ex.visitChildren(node)
		return
	}

	kind := types.NodeKindFunction
	sig := extractSignature(node, ex.Source)

	fn := ex.makeNode(kind, node.Name, node, sig)
	if fn == nil {
		return
	}
	ex.pushScope(fn.ID)
	ex.visitChildren(node)
	ex.popScope()
}

func (ex *Extractor) extractMethod(node *Node) {
	if node.Name == "" {
		ex.visitChildren(node)
		return
	}

	// Check if inside a class
	if !ex.isInsideClass() {
		// Top-level method in TS/JS (e.g., object literal) - treat as function
		ex.extractFunction(node)
		return
	}

	sig := extractSignature(node, ex.Source)
	method := ex.makeNode(types.NodeKindMethod, node.Name, node, sig)
	if method == nil {
		return
	}
	ex.pushScope(method.ID)
	ex.visitChildren(node)
	ex.popScope()
}

func (ex *Extractor) extractClass(node *Node) {
	if node.Name == "" {
		ex.visitChildren(node)
		return
	}

	cls := ex.makeNode(types.NodeKindClass, node.Name, node, "")
	if cls == nil {
		return
	}
	ex.pushScope(cls.ID)
	ex.visitChildren(node)
	ex.popScope()
}

func (ex *Extractor) extractInterface(node *Node) {
	if node.Name == "" {
		ex.visitChildren(node)
		return
	}
	iface := ex.makeNode(types.NodeKindInterface, node.Name, node, "")
	if iface == nil {
		return
	}
}

func (ex *Extractor) extractEnum(node *Node) {
	if node.Name == "" {
		ex.visitChildren(node)
		return
	}
	en := ex.makeNode(types.NodeKindEnum, node.Name, node, "")
	if en == nil {
		return
	}
	ex.pushScope(en.ID)
	ex.visitChildren(node)
	ex.popScope()
}

func (ex *Extractor) extractImport(node *Node) {
	// Extract the module name from the import statement
	src := string(ex.Source)
	// Simple extraction: find the string between quotes
	// A more robust version would walk the CST children
	for _, child := range node.Children {
		if child.Kind == "string" || child.Kind == "string_fragment" {
			moduleName := child.Name
			if moduleName == "" {
				// Extract from source
				start := findPos(src, child.StartLine, child.StartCol)
				end := findPos(src, child.EndLine, child.EndCol)
				if start >= 0 && end > start {
					moduleName = strings.Trim(src[start:end], "'\"")
				}
			}
			if moduleName != "" {
				imp := ex.makeNode(types.NodeKindImport, moduleName, node, "")
				_ = imp
			}
		}
	}
}

func (ex *Extractor) extractCall(node *Node) {
	// Create a calls edge from the current scope to the called function
	// The callee is typically the first child that has a name
	for _, child := range node.Children {
		if child.Name != "" {
			callerID := ex.currentScope()
			if callerID != "" {
				ex.edges = append(ex.edges, types.Edge{
					Source:  callerID,
					Target:  fmt.Sprintf("call:%s", child.Name),
					Kind:    types.EdgeCalls,
				})
			}
			break
		}
	}
}

func (ex *Extractor) extractVariable(node *Node) {
	// Check all variable declarators for names
	for _, child := range node.Children {
		if child.Kind == "variable_declarator" && child.Name != "" {
			isConst := false
			for _, syntactical := range node.Children {
				if syntactical.Kind == "const" {
					isConst = true
					break
				}
			}
			kind := types.NodeKindVariable
			if isConst {
				kind = types.NodeKindConstant
			}
			ex.makeNode(kind, child.Name, &child, "")
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (ex *Extractor) makeNode(kind types.NodeKind, name string, node *Node, signature string) *types.Node {
	if name == "" {
		return nil
	}

	id := fmt.Sprintf("%s:%s:%s", kind, ex.FilePath, name)

	n := &types.Node{
		ID:        id,
		Kind:      kind,
		Name:      name,
		FilePath:  ex.FilePath,
		Language:  types.Language(ex.Language),
		StartLine: node.StartLine,
		EndLine:   node.EndLine,
		StartColumn: node.StartCol,
		EndColumn: node.EndCol,
		UpdatedAt: now(),
	}

	if signature != "" {
		s := signature
		n.Signature = &s
	}

	// Build qualified name from scope
	var parts []string
	for _, sid := range ex.scope {
		for _, pn := range ex.nodes {
			if pn.ID == sid && pn.Kind != types.NodeKindFile {
				parts = append(parts, pn.Name)
			}
		}
	}
	parts = append(parts, name)
	n.QualifiedName = strings.Join(parts, "::")

	ex.nodes = append(ex.nodes, *n)

	// Add contains edge from parent scope
	if parent := ex.currentScope(); parent != "" {
		ex.edges = append(ex.edges, types.Edge{
			Source: parent,
			Target: id,
			Kind:   types.EdgeContains,
		})
	}

	return n
}

func (ex *Extractor) pushScope(id string) {
	ex.scope = append(ex.scope, id)
}

func (ex *Extractor) popScope() {
	if len(ex.scope) > 0 {
		ex.scope = ex.scope[:len(ex.scope)-1]
	}
}

func (ex *Extractor) currentScope() string {
	if len(ex.scope) == 0 {
		return ""
	}
	return ex.scope[len(ex.scope)-1]
}

func (ex *Extractor) isInsideClass() bool {
	for i := len(ex.scope) - 1; i >= 0; i-- {
		id := ex.scope[i]
		for _, n := range ex.nodes {
			if n.ID == id && n.Kind == types.NodeKindClass {
				return true
			}
		}
	}
	return false
}

// extractSignature extracts a function/method signature from source text.
func extractSignature(node *Node, source []byte) string {
	if node.StartLine == node.EndLine {
		line := extractLine(source, node.StartLine)
		if line != "" {
			// Trim to just the signature (before the body brace)
			if idx := strings.Index(line, "{"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
			}
			return line
		}
	}
	return ""
}

func extractLine(source []byte, lineNum int) string {
	lines := strings.Split(string(source), "\n")
	if lineNum >= 1 && lineNum <= len(lines) {
		return strings.TrimSpace(lines[lineNum-1])
	}
	return ""
}

func findPos(source string, line, col int) int {
	lines := strings.Split(source, "\n")
	if line < 1 || line > len(lines) {
		return -1
	}
	offset := 0
	for i := 0; i < line-1; i++ {
		offset += len(lines[i]) + 1
	}
	return offset + col
}

func now() int64 {
	return time.Now().UnixMilli()
}
