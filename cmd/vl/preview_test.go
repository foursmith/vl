package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

func TestPreviewWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "-source.go")
	source := "package main\r\n\t// 中文例\r\n\tvar a = 123\r\nvar hidden = 0"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	status := runPreview([]string{"--skip=1", "--lines=2", "--width=8", "--tab-size=2", "--color=always", "--", path}, &out, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if got, want := ansi.Strip(out.String()), "  // 中\n  var a \n"; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}

func TestPreviewStripsFileControls(t *testing.T) {
	if got, want := previewText("\x1b[2J\tcode\u009b", "  "), "[2J  code"; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}

func TestPreviewHighlightsFirstTypeScriptLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ts")
	source := "import process from \"node:process\"\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	args := []string{"--lines=1", "--color=always", "--theme=dark_modern", path}
	if status := runPreview(args, &out, &stderr); status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(out.String(), "\x1b[38;2;197;134;192mimport") || ansi.Strip(out.String()) != source {
		t.Fatalf("first line lost syntax highlighting: %q", out.String())
	}
}

func TestPreviewLineNumbersAndWrap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "text.txt")
	source := strings.Repeat("\n", 998) + "ABC中文\nlast\nextra\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"clip", []string{"--width=12"}, " 999  ABC中\n1000  last\n"},
		{"wrap", []string{"--width=12", "--wrap"}, " 999  ABC中\n      文\n1000  last\n"},
		{"narrow", []string{"--width=4", "--wrap"}, "ABC\n中文\nlast\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"--skip=998", "--lines=2", "--line-numbers", "--color=never"}
			args = append(args, test.args...)
			var out, stderr bytes.Buffer
			if status := runPreview(append(args, path), &out, &stderr); status != 0 {
				t.Fatalf("status %d: %s", status, stderr.String())
			}
			if out.String() != test.want {
				t.Fatalf("output %q, want %q", out.String(), test.want)
			}
		})
	}
}

func TestPreviewWrapPreservesSyntaxWithoutColoringContinuationGutter(t *testing.T) {
	var out bytes.Buffer
	opt := previewOptions{width: 7, wrap: true, colors: theme.Builtin("dark_modern")}
	if err := opt.writeLine(&out, "\x1b[48;2;1;2;3mABCD\x1b[m", 1, 3); err != nil {
		t.Fatal(err)
	}
	first, rest, _ := strings.Cut(out.String(), "\n")
	if got := ansi.Strip(out.String()); got != "  1  AB\n     CD\n" {
		t.Fatalf("output %q", got)
	}
	if !strings.HasSuffix(first, "AB"+ansi.ResetStyle) || !strings.HasPrefix(rest, "     \x1b[48;2;1;2;3mCD") {
		t.Fatalf("wrapping did not isolate the continuation gutter: %q", out.String())
	}
}

func TestPreviewWindowAfterLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.txt")
	source := strings.Repeat("x", 3*previewLineBytes) + "\n\nlast"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	opt := previewOptions{skip: 1, lines: 2, tabSize: 4}
	if err := writePreview(&out, path, opt); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "\nlast\n"; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}
