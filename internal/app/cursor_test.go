package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
)

func TestEditorCursorModePositionAndVisibility(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "a\t中e\u0301", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab, m.sidebar = 0, false
	d.SetPosition(0, 5)
	m.tabs[0].left = 2
	check := func() {
		t.Helper()
		c := m.View().Cursor
		if c == nil || c.Shape != tea.CursorBar || c.X != m.mainX()+12 || c.Y != editorContentRow {
			t.Fatalf("insertion cursor: %+v", c)
		}
		for _, cell := range uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0] {
			if cell.Style.Attrs&uv.AttrReverse != 0 {
				t.Fatal("block remains under insertion cursor")
			}
		}
	}
	check()
	m.vimEnabled = true
	if c := m.View().Cursor; c == nil || c.Shape != tea.CursorBlock || c.Blink {
		t.Fatalf("Vim navigation cursor: %+v", c)
	}
	m.tabs[0].vim.InsertMode = true
	check()
	m.sidebar = true
	if m.View().Cursor != nil {
		t.Fatal("sidebar retained editor caret")
	}
	m.sidebar = false
	m.modal = "find"
	if m.editorCursor() != nil {
		t.Fatal("modal retained editor caret")
	}
	m.modal = ""
	m.tabs[0].left = 100
	if m.editorCursor() != nil {
		t.Fatal("offscreen caret is visible")
	}
}
