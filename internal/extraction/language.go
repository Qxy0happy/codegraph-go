package extraction

import (
	"path/filepath"
	"strings"
)

// DetectLanguage determines the programming language from a file path.
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".py", ".pyw":
		return "python"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php", ".module", ".inc", ".theme":
		return "php"
	case ".rb", ".rake":
		return "ruby"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".dart":
		return "dart"
	case ".scala", ".sc":
		return "scala"
	case ".lua":
		return "lua"
	case ".luau":
		return "luau"
	case ".m", ".mm":
		return "objc"
	case ".yml", ".yaml":
		return "yaml"
	case ".svelte":
		return "svelte"
	case ".vue":
		return "vue"
	default:
		return ""
	}
}

// SourceExtensions returns a set of recognized source file extensions.
var SourceExtensions = map[string]bool{
	".ts": true, ".tsx": true,
	".js": true, ".mjs": true, ".cjs": true, ".jsx": true,
	".py": true, ".pyw": true,
	".go": true,
	".rs": true,
	".java": true,
	".c": true, ".h": true,
	".cpp": true, ".cc": true, ".cxx": true, ".hpp": true, ".hxx": true,
	".cs": true,
	".php": true, ".module": true, ".inc": true, ".theme": true,
	".rb": true, ".rake": true,
	".swift": true,
	".kt": true, ".kts": true,
	".dart": true,
	".scala": true, ".sc": true,
	".lua": true, ".luau": true,
	".m": true, ".mm": true,
	".yml": true, ".yaml": true,
	".svelte": true, ".vue": true,
}
