package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foursmith/vl/internal/editor"
)

func TestMarkdownNewline(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
	}{
		{"unordered", "- 中文|", "- 中文\n- |"},
		{"nested ordered", "1. Parent\n   9) Nested|", "1. Parent\n   9) Nested\n   10) |"},
		{"task", "> - [X] Done|", "> - [X] Done\n> - [ ] |"},
		{"quote", "> > Text|", "> > Text\n> > |"},
		{"exit list", "- One\n- |", "- One\n|"},
		{"exit quoted task", "> - [ ] |", "> |"},
		{"exit inner quote", "> > |", "> |"},
		{"mid-line", "- 一|二", "- 一\n|二"},
		{"fenced code", "```text\n- example|\n```", "```text\n- example\n|\n```"},
		{"quoted fenced code", "> ~~~\n> - example|\n> ~~~", "> ~~~\n> - example\n|\n> ~~~"},
		{"indented code", "    - example|", "    - example\n|"},
		{"after fence", "```\ntext\n```\n- item|", "```\ntext\n```\n- item\n- |"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, rest, _ := strings.Cut(tc.before, "|")
			d := &editor.Document{Path: "file.md"}
			d.Text = before + rest
			d.SetPosition(strings.Count(before, "\n"), len([]rune(before[strings.LastIndexByte(before, '\n')+1:])))
			if !continueMarkdownLine(d) {
				d.Insert("\n")
			}
			cursor := d.ByteOffset(d.Cursor)
			if got := d.Text[:cursor] + "|" + d.Text[cursor:]; got != tc.after {
				t.Fatalf("after Enter = %q, want %q", got, tc.after)
			}
			d.Undo()
			if d.Text != before+rest {
				t.Fatalf("after undo = %q", d.Text)
			}
		})
	}
}

func TestMarkdownEnterFromEditor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.md")
	if err := os.WriteFile(path, []byte("- 中文"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	d := m.document()
	press(m, "A")
	press(m, "enter")
	if got := d.Text; got != "- 中文\n- " {
		t.Fatalf("Vim Insert Enter = %q", got)
	}
	press(m, "esc")
	press(m, "u")
	if got := d.Text; got != "- 中文" {
		t.Fatalf("Vim undo = %q", got)
	}

	m.setVimDisabled(true)
	d.SetPosition(0, d.LineRuneCount(0))
	press(m, "enter")
	press(m, "enter")
	if got := d.Text; got != "- 中文\n" {
		t.Fatalf("after exiting the list = %q", got)
	}
}
