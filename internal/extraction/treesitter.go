// Package extraction provides tree-sitter based code parsing for the
// CodeGraph knowledge graph. Uses smacker/go-tree-sitter (CGo) for the
// tree-sitter C library and compiles grammar sources via CGo subpackages.
package extraction

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	golang "github.com/smacker/go-tree-sitter/golang"
	ts "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Node represents a parsed syntax tree node with Go-friendly fields.
type Node struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
	Children  []Node `json:"children,omitempty"`
}

// languageMap maps language names to their GetLanguage functions.
type languageFunc func() *sitter.Language

// languageMap maps language names to their grammar providers.
var languages = map[string]languageFunc{
	"typescript": ts.GetLanguage,
	"go":         golang.GetLanguage,
}

// Parse parses source code for the given language and returns the CST.
func Parse(source []byte, langName string) (*Node, error) {
	getLang, ok := languages[langName]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", langName)
	}

	root, err := sitter.ParseCtx(context.Background(), source, getLang())
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return walkNode(root, source), nil
}

func walkNode(node *sitter.Node, source []byte) *Node {
	n := &Node{
		Kind:      node.Type(),
		StartLine: int(node.StartPoint().Row) + 1,
		StartCol:  int(node.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		EndCol:    int(node.EndPoint().Column),
	}
	n.Name = extractName(node, source)

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		n.Children = append(n.Children, *walkNode(child, source))
	}
	return n
}

func extractName(node *sitter.Node, source []byte) string {
	for _, field := range []string{"name", "Name", "property", "identifier"} {
		if c := node.ChildByFieldName(field); c != nil {
			return c.Content(source)
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		c := node.NamedChild(i)
		if c == nil {
			continue
		}
		typ := c.Type()
		if strings.Contains(typ, "identifier") || typ == "constant" {
			return c.Content(source)
		}
	}
	return ""
}

// DebugPrint prints the AST tree for debugging.
func DebugPrint(node *Node, indent int) {
	prefix := strings.Repeat("  ", indent)
	name := ""
	if node.Name != "" {
		name = " \"" + node.Name + "\""
	}
	fmt.Printf("%s%s%s (L%d:%d-%d:%d)\n", prefix, node.Kind, name,
		node.StartLine, node.StartCol, node.EndLine, node.EndCol)
	for _, child := range node.Children {
		DebugPrint(&child, indent+1)
	}
}
