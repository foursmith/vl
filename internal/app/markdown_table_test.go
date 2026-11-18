package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMarkdownTableLayoutUsesSourceCoordinates(t *testing.T) {
	const source = "|Name|Value|\n|---|---:|\n|中文|1|\n|👩‍💻 é|2|\n|a\\|b|3|\n\n```md\n|x|y|\n|---|---|\n|longer|z|\n```\n"
	path := filepath.Join(t.TempDir(), "table.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	d := m.document()
	check := func() int {
		t.Helper()
		lines := m.editorLines()
		separator := -1
		for i := 0; i < 5; i++ {
			line := ansi.Strip(lines[i])[7:]
			last := strings.LastIndexByte(line, '|')
			middle := strings.LastIndexByte(line[:last], '|')
			pos := ansi.StringWidth(line[:middle])
			if separator >= 0 && pos != separator {
				t.Fatalf("row %d separator at %d, want %d in %q", i, pos, separator, line)
			}
			separator = pos
		}
		if line := ansi.Strip(lines[7])[7:]; !strings.HasPrefix(line, "|x|y|") {
			t.Fatalf("code block changed visually: %q", line)
		}
		return separator
	}
	before := check()
	x := before + 7
	m.positionEditor(x, 2, false)
	if line, col := d.Position(); line != 2 || col != 3 {
		t.Fatalf("clicked separator mapped to (%d, %d)", line, col)
	}
	if cursor := m.editorCursor(); cursor == nil || cursor.X != x {
		t.Fatalf("cursor not drawn on separator: %v", cursor)
	}
	m.positionEditor(x, 3, true)
	want := "|1|\n|👩‍💻 é"
	if got := d.SelectedText(); got != want {
		t.Fatalf("selected %q, want original text %q", got, want)
	}
	if d.Dirty() || d.Text != source {
		t.Fatal("display layout changed document")
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != source {
		t.Fatalf("save changed Markdown: %q, %v", data, err)
	}

	row := d.Lines()[4]
	d.SetPosition(4, utf8.RuneCountInString(row[:strings.LastIndex(row, "|3")]))
	d.Insert(strings.Repeat("寬", 24))
	if after := check(); after != before+48 {
		t.Fatalf("edit moved separator from %d to %d", before, after)
	}
	m.Update(tea.WindowSizeMsg{Width: 26, Height: 20})
	m.positionEditor(8, 0, false)
	tab := &m.tabs[m.activeTab]
	tab.left = 0
	m.scrollHorizontal(200)
	padding := tab.tableRows()[0]
	header := displayWidth(d.Lines()[0], 0, padding)
	if tab.left+19 < header {
		t.Fatalf("horizontal scroll hides end of the projected header: left %d, header %d", tab.left, header)
	}
}
