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
	m.tabs[0].vim.InsertMode = true
	check()
	m.tabs[0].vim.InsertMode = false
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
	m.modal = "confirm"
	if m.editorCursor() != nil {
		t.Fatal("modal retained editor caret")
	}
	m.modal = ""
	m.tabs[0].left = 100
	if m.editorCursor() != nil {
		t.Fatal("offscreen caret is visible")
	}
}

func TestSearchInputsUseNativeBarCursor(t *testing.T) {
	m := newHoverRenderModel(t)
	m.openSearch()
	m.search.replace, m.search.details = true, true
	layout := m.searchPanelLayout()
	for _, tt := range []struct{ focus, row int }{{0, layout.query}, {1, layout.replacement}, {3, layout.include}, {4, layout.exclude}} {
		m.focusSearch(tt.focus)
		m.searchField(tt.focus).SetValue("中a")
		c := m.View().Cursor
		if c == nil || c.Shape != tea.CursorBar || c.X != m.sideX()+searchFieldInset+1+3 || c.Y != tt.row {
			t.Fatalf("search focus %d: %+v", tt.focus, c)
		}
	}
	m.tabs = []tab{{doc: &editor.Document{Text: "text", Anchor: -1}}}
	m.activeTab = 0
	m.openFileFind()
	for _, focus := range []int{0, 1} {
		m.focusFileFind(focus)
		m.searchField(focus).SetValue("中a")
		m.searchField(focus).CursorEnd()
		row := fileFindQueryRow
		if focus == 1 {
			row = fileReplaceRow
		}
		c := m.View().Cursor
		if c == nil || c.Shape != tea.CursorBar || c.X != m.fileFindX()+searchFieldInset+4 || c.Y != m.contentRow()+row {
			t.Fatalf("file find focus %d: %+v", focus, c)
		}
	}
	m.prompt("quick", "Search files", "中a")
	l := m.pickerLayout(m.filteredActions())
	if c := m.View().Cursor; c == nil || c.Shape != tea.CursorBar || c.X != l.x+5 || c.Y != l.y+1 {
		t.Fatalf("quick search cursor: %+v", c)
	}
}

func TestInputCursorTracksScrolledUnicode(t *testing.T) {
	m := newHoverRenderModel(t)
	m.input.Focus()
	m.input.Prompt = " "
	m.input.SetWidth(4)
	for _, tt := range []struct {
		text string
		x    int
	}{{"", 11}, {"中a", 14}, {"abcdef", 15}, {"中中中", 15}, {"e\u0301abc", 15}} {
		m.input.SetValue(tt.text)
		m.input.CursorEnd()
		view := m.inputView(m.input, 10, 2)
		if c := m.inputCaret; c == nil || c.Shape != tea.CursorBar || c.X != tt.x || c.Y != 2 {
			t.Fatalf("%q: %+v", tt.text, c)
		}
		for _, cell := range uv.NewStyledString(view).Lines(ansi.GraphemeWidth)[0] {
			if cell.Style.Attrs&uv.AttrReverse != 0 {
				t.Fatal("virtual block remains under native cursor")
			}
		}
	}
}
