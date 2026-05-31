package extraction

import (
	"testing"
)

func TestParseTypeScript(t *testing.T) {
	testSource := `
function hello(name: string): string {
	return "Hello, " + name;
}

class Greeter {
	private greeting: string;

	constructor(message: string) {
		this.greeting = message;
	}

	public greet(): string {
		return this.greeting;
	}
}

export const PI = 3.14159;
`

	root, err := Parse([]byte(testSource), "typescript")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if root == nil {
		t.Fatal("Parse returned nil")
	}
	t.Logf("Root node: %s (L%d:%d-%d:%d)", root.Kind, root.StartLine, root.StartCol, root.EndLine, root.EndCol)
	t.Logf("Children: %d", len(root.Children))

	// Verify expected structure
	foundFunction := false
	foundClass := false

	var findNodes func(n *Node, depth int)
	findNodes = func(n *Node, depth int) {
		prefix := ""
		for i := 0; i < depth; i++ {
			prefix += "  "
		}
		name := ""
		if n.Name != "" {
			name = " \"" + n.Name + "\""
		}
		t.Logf("%s%s%s (L%d:%d)", prefix, n.Kind, name, n.StartLine, n.StartCol)

		switch n.Kind {
		case "function_declaration":
			foundFunction = true
			if n.Name != "hello" {
				t.Errorf("expected function name 'hello', got '%s'", n.Name)
			}
		case "class_declaration":
			foundClass = true
			if n.Name != "Greeter" {
				t.Errorf("expected class name 'Greeter', got '%s'", n.Name)
			}
		}

		for _, child := range n.Children {
			findNodes(&child, depth+1)
		}
	}
	findNodes(root, 0)

	if !foundFunction {
		t.Error("expected function_declaration not found")
	}
	if !foundClass {
		t.Error("expected class_declaration not found")
	}
}

func TestDebugPrint(t *testing.T) {
	source := []byte(`function add(a: number, b: number): number {
	return a + b;
}`)

	root, err := Parse(source, "typescript")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	DebugPrint(root, 0)
}
