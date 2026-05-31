// Package types defines the core types for the CodeGraph knowledge graph.
package types

// NodeKind represents the type of a code symbol.
type NodeKind string

const (
	NodeKindFile       NodeKind = "file"
	NodeKindModule     NodeKind = "module"
	NodeKindClass      NodeKind = "class"
	NodeKindStruct     NodeKind = "struct"
	NodeKindInterface  NodeKind = "interface"
	NodeKindTrait      NodeKind = "trait"
	NodeKindProtocol   NodeKind = "protocol"
	NodeKindFunction   NodeKind = "function"
	NodeKindMethod     NodeKind = "method"
	NodeKindProperty   NodeKind = "property"
	NodeKindField      NodeKind = "field"
	NodeKindVariable   NodeKind = "variable"
	NodeKindConstant   NodeKind = "constant"
	NodeKindEnum       NodeKind = "enum"
	NodeKindEnumMember NodeKind = "enum_member"
	NodeKindTypeAlias  NodeKind = "type_alias"
	NodeKindNamespace  NodeKind = "namespace"
	NodeKindParameter  NodeKind = "parameter"
	NodeKindImport     NodeKind = "import"
	NodeKindExport     NodeKind = "export"
	NodeKindRoute      NodeKind = "route"
	NodeKindComponent  NodeKind = "component"
)

// EdgeKind represents the type of relationship between nodes.
type EdgeKind string

const (
	EdgeContains      EdgeKind = "contains"
	EdgeCalls         EdgeKind = "calls"
	EdgeImports       EdgeKind = "imports"
	EdgeExports       EdgeKind = "exports"
	EdgeExtends       EdgeKind = "extends"
	EdgeImplements    EdgeKind = "implements"
	EdgeReferences    EdgeKind = "references"
	EdgeTypeOf        EdgeKind = "type_of"
	EdgeReturns       EdgeKind = "returns"
	EdgeInstantiates  EdgeKind = "instantiates"
	EdgeOverrides     EdgeKind = "overrides"
	EdgeDecorates     EdgeKind = "decorates"
)

// Language represents a supported programming language.
type Language string

const (
	LangTypeScript   Language = "typescript"
	LangJavaScript   Language = "javascript"
	LangTSX          Language = "tsx"
	LangJSX          Language = "jsx"
	LangPython       Language = "python"
	LangGo           Language = "go"
	LangRust         Language = "rust"
	LangJava         Language = "java"
	LangC            Language = "c"
	LangCpp          Language = "cpp"
	LangCSharp       Language = "csharp"
	LangPHP          Language = "php"
	LangRuby         Language = "ruby"
	LangSwift        Language = "swift"
	LangKotlin       Language = "kotlin"
	LangDart         Language = "dart"
	LangSvelte       Language = "svelte"
	LangVue          Language = "vue"
	LangLiquid       Language = "liquid"
	LangPascal       Language = "pascal"
	LangScala        Language = "scala"
	LangLua          Language = "lua"
	LangLuau         Language = "luau"
	LangObjC         Language = "objc"
	LangYAML         Language = "yaml"
	LangTwig         Language = "twig"
	LangXML          Language = "xml"
	LangProperties   Language = "properties"
	LangUnknown      Language = "unknown"
)

// Node represents a code symbol in the knowledge graph.
type Node struct {
	ID             string     `json:"id"`
	Kind           NodeKind   `json:"kind"`
	Name           string     `json:"name"`
	QualifiedName  string     `json:"qualifiedName"`
	FilePath       string     `json:"filePath"`
	Language       Language   `json:"language"`
	StartLine      int        `json:"startLine"`
	EndLine        int        `json:"endLine"`
	StartColumn    int        `json:"startColumn"`
	EndColumn      int        `json:"endColumn"`
	Docstring      *string    `json:"docstring,omitempty"`
	Signature      *string    `json:"signature,omitempty"`
	Visibility     *string    `json:"visibility,omitempty"`
	IsExported     bool       `json:"isExported"`
	IsAsync        bool       `json:"isAsync"`
	IsStatic       bool       `json:"isStatic"`
	IsAbstract     bool       `json:"isAbstract"`
	Decorators     []string   `json:"decorators,omitempty"`
	TypeParameters []string   `json:"typeParameters,omitempty"`
	UpdatedAt      int64      `json:"updatedAt"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	ID         int64     `json:"id"`
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	Kind       EdgeKind  `json:"kind"`
	Metadata   *string   `json:"metadata,omitempty"`
	Line       *int      `json:"line,omitempty"`
	Col        *int      `json:"col,omitempty"`
	Provenance *string   `json:"provenance,omitempty"`
}

// File represents a tracked source file.
type File struct {
	Path        string `json:"path"`
	ContentHash string `json:"contentHash"`
	Language    string `json:"language"`
	Size        int64  `json:"size"`
	ModifiedAt  int64  `json:"modifiedAt"`
	IndexedAt   int64  `json:"indexedAt"`
	NodeCount   int    `json:"nodeCount"`
}

// UnresolvedRef represents a reference that needs resolution.
type UnresolvedRef struct {
	ID            int64   `json:"id"`
	FromNodeID    string  `json:"fromNodeId"`
	ReferenceName string  `json:"referenceName"`
	ReferenceKind string  `json:"referenceKind"`
	Line          int     `json:"line"`
	Col           int     `json:"col"`
	Candidates    *string `json:"candidates,omitempty"`
	FilePath      string  `json:"filePath"`
	Language      string  `json:"language"`
}

// Subgraph represents a slice of the graph returned by traversal.
type Subgraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Node       Node    `json:"node"`
	Relevance  float64 `json:"relevance"`
	Snippet    string  `json:"snippet,omitempty"`
}
