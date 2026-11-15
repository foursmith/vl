package highlight

import (
	"context"
	"strings"
	"testing"
)

func TestGoHighlightPreservesRuneOffsets(t *testing.T) {
	text := "package main\n\t// 中文\nfunc main() { println(\"hello\") }\n"
	lines := Lines(context.Background(), "main.go", text)
	if len(lines) != 4 || len(lines[0]) == 0 || len(lines[1]) == 0 {
		t.Fatalf("missing highlighted tokens: %+v", lines)
	}
	if lines[1][0].Start != 1 {
		t.Fatalf("tab offset changed: %+v", lines[1])
	}
	if result := Lines(context.Background(), "unknown.elegit-unknown", "plain text"); result != nil {
		t.Fatal("unknown files should remain plain text")
	}
}

func BenchmarkSyntaxHighlight(b *testing.B) {
	text := strings.Repeat("var value = \"sample\" // 中文\n", 5000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Lines(context.Background(), "source.go", text)
	}
}

func TestGoModernDarkKeywordRoles(t *testing.T) {
	text := "package main\nfunc main() {\nvar n int\nif n == 0 { return }\nprintln(n)\n}\n"
	lines := Lines(context.Background(), "main.go", text)
	for _, check := range []struct {
		line, column int
		color        string
	}{
		{0, 0, "#569CD6"},  // package declaration
		{1, 0, "#569CD6"},  // func declaration
		{1, 5, "#DCDCAA"},  // function name
		{2, 0, "#569CD6"},  // var declaration
		{2, 6, "#4EC9B0"},  // int type
		{3, 0, "#C586C0"},  // if control flow
		{3, 12, "#C586C0"}, // return control flow
		{4, 0, "#DCDCAA"},  // builtin function
	} {
		found := ""
		for _, span := range lines[check.line] {
			if check.column >= span.Start && check.column < span.End {
				found = span.Color
				break
			}
		}
		if found != check.color {
			t.Errorf("line %d column %d: %s, want %s", check.line, check.column, found, check.color)
		}
	}
}

func TestGoImportUsesVSCodeControlScope(t *testing.T) {
	lines := Lines(context.Background(), "main.go", "import \"fmt\"\n")
	if len(lines) == 0 || len(lines[0]) == 0 || lines[0][0].Start != 0 || lines[0][0].Color != "#C586C0" {
		t.Fatalf("Go import lost keyword.control.import.go color: %+v", lines)
	}
}
