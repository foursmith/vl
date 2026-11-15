package highlight

import (
	"context"
	"strings"
	"testing"
)

func TestGoHighlightPreservesRuneOffsets(t *testing.T) {
	text := "package main\n\t// 中文\nfunc main() { println(\"hello\") }\n"
	lines := Lines(context.Background(), "main.go", text, false)
	if len(lines) != 4 || len(lines[0]) == 0 || len(lines[1]) == 0 {
		t.Fatalf("missing highlighted tokens: %+v", lines)
	}
	if lines[1][0].Start != 1 {
		t.Fatalf("tab offset changed: %+v", lines[1])
	}
	if result := Lines(context.Background(), "unknown.elegit-unknown", "plain text", false); result != nil {
		t.Fatal("unknown files should remain plain text")
	}
}

func BenchmarkSyntaxHighlight(b *testing.B) {
	text := strings.Repeat("var value = \"sample\" // 中文\n", 5000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Lines(context.Background(), "source.go", text, false)
	}
}

func TestGoModernDarkKeywordRoles(t *testing.T) {
	text := "package main\nfunc main() {\nvar n int\nif n == 0 { return }\nprintln(n)\n}\n"
	lines := Lines(context.Background(), "main.go", text, false)
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
	lines := Lines(context.Background(), "main.go", "import \"fmt\"\n", false)
	if len(lines) == 0 || len(lines[0]) == 0 || lines[0][0].Start != 0 || lines[0][0].Color != "#C586C0" {
		t.Fatalf("Go import lost keyword.control.import.go color: %+v", lines)
	}
}

func styleAt(lines [][]Span, line, column int) Span {
	for _, span := range lines[line] {
		if column >= span.Start && column < span.End {
			return span
		}
	}
	return Span{Color: darkModernTheme.foreground}
}

func TestTextMateCarriesMultilineStateAndRuneOffsets(t *testing.T) {
	text := "package main\n/* comment\n中文 */ var n = `first\n中文`\nreturn\n"
	lines := Lines(context.Background(), "main.go", text, false)
	for _, check := range []struct {
		line, col int
		color     string
	}{
		{1, 0, "#6A9955"}, {2, 0, "#6A9955"}, {2, 6, "#569CD6"}, {2, 14, "#CE9178"}, {3, 0, "#CE9178"}, {4, 0, "#C586C0"},
	} {
		if got := styleAt(lines, check.line, check.col).Color; got != check.color {
			t.Errorf("%d:%d: %s, want %s", check.line, check.col, got, check.color)
		}
	}
	for i, line := range strings.Split(text, "\n") {
		length := len([]rune(line))
		for _, span := range lines[i] {
			if span.Start < 0 || span.End > length || span.End <= span.Start {
				t.Fatalf("invalid span on %q: %+v", line, span)
			}
		}
	}
}

func TestTextMateInheritedThemeAndFonts(t *testing.T) {
	if darkModernTheme.foreground != "#CCCCCC" {
		t.Fatal("Dark Modern did not override Dark VS foreground")
	}
	for _, check := range []struct {
		scopes []string
		color  string
		font   FontStyle
	}{
		{[]string{"source.go", "keyword.function.go"}, "#569CD6", 0},
		{[]string{"source.go", "keyword.control.go"}, "#C586C0", 0},
		{[]string{"source.go", "entity.name.function.go"}, "#DCDCAA", 0},
		{[]string{"source.go", "storage.type.numeric.go"}, "#4EC9B0", 0},
		{[]string{"text.html.markdown", "markup.bold.markdown"}, "#569CD6", Bold},
		{[]string{"text.html.markdown", "markup.italic.markdown"}, "#C586C0", Italic},
		{[]string{"source.powershell", "variable.other.member"}, "#DCDCAA", 0},
		{[]string{"source.go", "variable.other.member"}, "#9CDCFE", 0},
		{[]string{"source.css", "entity.other.attribute-name.class"}, "#D7BA7D", 0},
		{[]string{"text.html.markdown", "string", "meta.image.inline.markdown"}, "#D4D4D4", 0},
	} {
		got := darkModernTheme.resolve(check.scopes)
		if got.color != check.color || got.font != check.font {
			t.Errorf("%v: %+v, want %s font %v", check.scopes, got, check.color, check.font)
		}
	}
	lines := Lines(context.Background(), "README.md", "**bold** and *italic*\n", false)
	if styleAt(lines, 0, 2).Font&Bold == 0 || styleAt(lines, 0, 14).Font&Italic == 0 {
		t.Fatalf("font styles missing from spans: %+v", lines)
	}
}

func TestTextMateLimitsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Lines(ctx, "main.go", "package main", false) != nil {
		t.Fatal("cancelled work ran")
	}
	if Lines(context.Background(), "main.go", strings.Repeat("x", MaxBytes+1), false) != nil {
		t.Fatal("oversized document tokenized")
	}
	text := "package main\n" + strings.Repeat("x", 20_001) + "\nvar n int\n"
	lines := Lines(context.Background(), "main.go", text, false)
	if len(lines) != 4 || len(lines[0]) == 0 || len(lines[1]) != 0 || len(lines[2]) != 0 {
		t.Fatal("line limit lost prefix or propagated incomplete state")
	}
}

func TestLightModernInheritedSyntaxAndFonts(t *testing.T) {
	if lightModernTheme.foreground != "#3B3B3B" {
		t.Fatal("Light Modern foreground did not override inherited theme")
	}
	lines := Lines(context.Background(), "main.go", "package main\nfunc main() {\n// 中文\nvar n int\nif n == 0 { return }\n}\n", true)
	for _, check := range []struct {
		line, col int
		color     string
	}{
		{0, 0, "#0000FF"}, {1, 5, "#795E26"}, {2, 0, "#008000"}, {3, 6, "#267F99"}, {4, 0, "#AF00DB"},
	} {
		found := lightModernTheme.foreground
		for _, span := range lines[check.line] {
			if check.col >= span.Start && check.col < span.End {
				found = span.Color
				break
			}
		}
		if found != check.color {
			t.Errorf("%d:%d: %s, want %s", check.line, check.col, found, check.color)
		}
	}
	markdown := Lines(context.Background(), "README.md", "**bold** and *italic*\n", true)
	if styleAt(markdown, 0, 2).Font&Bold == 0 || styleAt(markdown, 0, 14).Font&Italic == 0 {
		t.Fatal("light syntax lost font styles")
	}
}
