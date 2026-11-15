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
