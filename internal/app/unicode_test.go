package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestUnicodePaintingMouseAndHorizontalScrollShareBoundaries(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	family := "👨‍👩‍👧‍👦"
	d := m.document()
	d.Edit("A" + family + "e\u0301🇨🇳\tB")
	d.SetPosition(0, 1)
	rendered := m.editorLines()[0]
	found := false
	for _, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
		if cell.Content == family {
			found = true
		}
	}
	if !found {
		t.Fatal("renderer split the family grapheme")
	}
	if c := m.editorCursor(); c == nil || c.Shape != tea.CursorBlock || c.X != m.mainX()+8 {
		t.Fatalf("cursor did not align with emoji boundary: %+v", c)
	}
	for _, click := range []struct{ cell, col int }{{1, 1}, {2, 1}, {3, 1 + utf8.RuneCountInString(family)}, {4, 3 + utf8.RuneCountInString(family)}} {
		m.click(tea.MouseClickMsg{X: m.mainX() + 7 + click.cell, Y: editorContentRow, Button: tea.MouseLeft})
		if _, col := d.Position(); col != click.col {
			t.Fatalf("click column %d -> rune %d, want %d", click.cell, col, click.col)
		}
	}
	m.tabs[m.activeTab].left = 2
	clipped := plain(m.editorLines()[0])
	if strings.Contains(clipped, family) || strings.ContainsRune(clipped, '\u200d') {
		t.Fatal("horizontal clipping exposed a partial emoji")
	}
	if expanded := expandTabs(family + "\tX"); expanded != family+"  X" {
		t.Fatalf("tab after family: %q", expanded)
	}
}
func TestFindExpandsPartialGraphemeSelection(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Edit("e\u0301x")
	d.SetPosition(0, 0)
	m.find = "\u0301"
	m.findNext()
	if d.SelectedText() != "e\u0301" || d.Cursor != 2 {
		t.Fatal("find left selection/cursor inside combining cluster")
	}
}
