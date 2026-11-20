package highlight

import (
	"context"
	"strings"
	"testing"

	"github.com/foursmith/vl/internal/theme"
)

func TestGoHighlightPreservesRuneOffsets(t *testing.T) {
	text := "package main\n\t// 中文\nfunc main() { println(\"hello\") }\n"
	lines := highlightOnce(context.Background(), "main.go", text, theme.Builtin("dark_modern"))
	if len(lines) != 4 || len(lines[0]) == 0 || len(lines[1]) == 0 {
		t.Fatalf("missing highlighted tokens: %+v", lines)
	}
	if lines[1][0].Start != 1 {
		t.Fatalf("tab offset changed: %+v", lines[1])
	}
	if result := highlightOnce(context.Background(), "unknown.vl-unknown", "plain text", theme.Builtin("dark_modern")); result != nil {
		t.Fatal("unknown files should remain plain text")
	}
}

func BenchmarkSyntaxHighlight(b *testing.B) {
	text := strings.Repeat("var value = \"sample\" // 中文\n", 5000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		referenceLines(context.Background(), "source.go", text, theme.Builtin("dark_modern"))
	}
}

func TestGoModernDarkKeywordRoles(t *testing.T) {
	text := "package main\nfunc main() {\nvar n int\nif n == 0 { return }\nprintln(n)\n}\n"
	lines := highlightOnce(context.Background(), "main.go", text, theme.Builtin("dark_modern"))
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
	lines := highlightOnce(context.Background(), "main.go", "import \"fmt\"\n", theme.Builtin("dark_modern"))
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
	return Span{Color: theme.Builtin("dark_modern").Palette().Foreground}
}

func TestTextMateCarriesMultilineStateAndRuneOffsets(t *testing.T) {
	text := "package main\n/* comment\n中文 */ var n = `first\n中文`\nreturn\n"
	lines := highlightOnce(context.Background(), "main.go", text, theme.Builtin("dark_modern"))
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
	if theme.Builtin("dark_modern").Palette().Foreground != "#CCCCCC" {
		t.Fatal("Dark Modern did not override Dark VS foreground")
	}
	for _, check := range []struct {
		scopes []string
		color  string
		font   theme.FontStyle
	}{
		{[]string{"source.go", "keyword.function.go"}, "#569CD6", 0},
		{[]string{"source.go", "keyword.control.go"}, "#C586C0", 0},
		{[]string{"source.go", "entity.name.function.go"}, "#DCDCAA", 0},
		{[]string{"source.go", "storage.type.numeric.go"}, "#4EC9B0", 0},
		{[]string{"text.html.markdown", "markup.bold.markdown"}, "#569CD6", theme.Bold},
		{[]string{"text.html.markdown", "markup.italic.markdown"}, "#C586C0", theme.Italic},
		{[]string{"source.powershell", "variable.other.member"}, "#DCDCAA", 0},
		{[]string{"source.go", "variable.other.member"}, "#9CDCFE", 0},
		{[]string{"source.css", "entity.other.attribute-name.class"}, "#D7BA7D", 0},
		{[]string{"text.html.markdown", "string", "meta.image.inline.markdown"}, "#D4D4D4", 0},
	} {
		got := theme.Builtin("dark_modern").Resolve(check.scopes)
		if got.Color != check.color || got.Font != check.font {
			t.Errorf("%v: %+v, want %s font %v", check.scopes, got, check.color, check.font)
		}
	}
	lines := highlightOnce(context.Background(), "README.md", "**bold** and *italic*\n", theme.Builtin("dark_modern"))
	if styleAt(lines, 0, 2).Font&theme.Bold == 0 || styleAt(lines, 0, 14).Font&theme.Italic == 0 {
		t.Fatalf("font styles missing from spans: %+v", lines)
	}
}

func TestTextMateLimitsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if highlightOnce(ctx, "main.go", "package main", theme.Builtin("dark_modern")) != nil {
		t.Fatal("cancelled work ran")
	}
	if highlightOnce(context.Background(), "main.go", strings.Repeat("x", MaxBytes+1), theme.Builtin("dark_modern")) != nil {
		t.Fatal("oversized document tokenized")
	}
	text := "package main\n" + strings.Repeat("x", 20_001) + "\nvar n int\n"
	lines := highlightOnce(context.Background(), "main.go", text, theme.Builtin("dark_modern"))
	if len(lines) != 4 || len(lines[0]) == 0 || len(lines[1]) != 0 || len(lines[2]) != 0 {
		t.Fatal("line limit lost prefix or propagated incomplete state")
	}
}

func TestLightModernInheritedSyntaxAndFonts(t *testing.T) {
	if theme.Builtin("light_modern").Palette().Foreground != "#3B3B3B" {
		t.Fatal("Light Modern foreground did not override inherited theme")
	}
	lines := highlightOnce(context.Background(), "main.go", "package main\nfunc main() {\n// 中文\nvar n int\nif n == 0 { return }\n}\n", theme.Builtin("light_modern"))
	for _, check := range []struct {
		line, col int
		color     string
	}{
		{0, 0, "#0000FF"}, {1, 5, "#795E26"}, {2, 0, "#008000"}, {3, 6, "#267F99"}, {4, 0, "#AF00DB"},
	} {
		found := theme.Builtin("light_modern").Palette().Foreground
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
	markdown := highlightOnce(context.Background(), "README.md", "**bold** and *italic*\n", theme.Builtin("light_modern"))
	if styleAt(markdown, 0, 2).Font&theme.Bold == 0 || styleAt(markdown, 0, 14).Font&theme.Italic == 0 {
		t.Fatal("light syntax lost font styles")
	}
}

func highlightOnce(ctx context.Context, path, text string, colors *theme.Theme) [][]Span {
	var session Session
	return session.Lines(ctx, Language(path), text, colors)
}
