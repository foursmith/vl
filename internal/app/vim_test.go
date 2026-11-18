package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/editor"
	"strings"
	"testing"
)

func TestVimWorkbenchRouting(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "one two\nthree", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false
	if !strings.Contains(m.statusLine(), "NORMAL") {
		t.Fatal("default Vim mode not visible")
	}
	press(m, "i")
	press(m, "X")
	press(m, "esc")
	if m.sidebar || d.Text != "Xone two\nthree" || m.tabs[0].vim.InsertMode {
		t.Fatal("Escape did not stay in editor Normal mode")
	}
	press(m, "u")
	if d.Text != "one two\nthree" {
		t.Fatal(d.Text)
	}
	press(m, "U")
	if m.modal != "" || d.Text != "Xone two\nthree" {
		t.Fatal("U did not redo")
	}
	m.Update(tea.PasteMsg{Content: "ignored"})
	if strings.Contains(d.Text, "ignored") {
		t.Fatal("Normal mode accepted bracketed paste")
	}
	press(m, "/")
	if !m.fileFindFocused() || m.modal != "" {
		t.Fatal("search component did not open")
	}
	press(m, "word")
	if d.Text != "Xone two\nthree" || !strings.Contains(m.search.query.Value(), "word") {
		t.Fatal("Vim intercepted modal input")
	}
	press(m, "esc")
	m.sidebar = true
	press(m, "i")
	if m.tabs[0].vim.InsertMode {
		t.Fatal("Vim intercepted sidebar")
	}
	m.sidebar = false
	press(m, "Z")
	if d.Text != "Xone two\nthree" {
		t.Fatal("Normal mode accepted ordinary text")
	}
}

func TestVimVisualRenderingAndRouting(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "abc\n\ndef", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false

	press(m, "v")
	character := strings.Join(m.editorLines(), "\n")
	if d.SelectedText() != "a" || !strings.Contains(m.statusLine(), "VISUAL") {
		t.Fatal("character selection not active")
	}
	press(m, "V")
	line := strings.Join(m.editorLines(), "\n")
	if d.SelectedText() != "abc\n" || character == line || !strings.Contains(m.statusLine(), "VISUAL LINE") {
		t.Fatal("mode switch reused cached selection rendering")
	}
	press(m, "j")
	press(m, "d")
	if d.Text != "def" || m.tabs[0].vim.VisualMode {
		t.Fatal("line selection delete failed")
	}
	press(m, "u")
	if d.Text != "abc\n\ndef" {
		t.Fatal("visual undo failed")
	}
	press(m, "v")
	press(m, "esc")
	if m.sidebar || d.Anchor != -1 {
		t.Fatal("Escape left editor or selection active")
	}
	press(m, "v")
	m.positionEditor(m.mainX()+8, editorContentRow, false)
	if m.tabs[0].vim.VisualMode || d.Anchor != -1 {
		t.Fatal("mouse reposition left a stale Visual anchor")
	}
}

func TestVimWordObjectsThroughWorkbench(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "alpha beta gamma", Anchor: -1}
	d.SetPosition(0, 8)
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false

	press(m, "d")
	press(m, "a")
	if d.Text != "alpha beta gamma" || m.tabs[0].vim.InsertMode {
		t.Fatal("object prefix edited text or entered Insert")
	}
	handled, cmd := m.vimKey("w")
	if !handled || d.Text != "alpha gamma" || cmd != nil || m.vimRegister != "beta " {
		t.Fatal("daw did not delete into the unnamed register")
	}
	press(m, "u")
	if d.Text != "alpha beta gamma" {
		t.Fatal("daw undo failed")
	}
	d.SetPosition(0, 8)
	press(m, "c")
	press(m, "i")
	press(m, "w")
	press(m, "X")
	press(m, "esc")
	if d.Text != "alpha X gamma" {
		t.Fatal("ciw failed:", d.Text)
	}
}

func TestVimColonReusesQuickPosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")

	d := m.document()
	d.Edit("first\n中文value\nthird")
	d.SetPosition(0, 2)
	press(m, ":")
	if m.modal != "quick" || m.input.Value() != ":" {
		t.Fatal("colon did not open existing file picker in position mode")
	}
	press(m, "enter")
	if m.modal != "quick" || d.Cursor != 2 {
		t.Fatal("empty line number moved cursor or closed picker")
	}
	m.Update(tea.PasteMsg{Content: "2:3"})
	drain(t, m, press(m, "enter"))
	row, col := d.Position()
	if row != 1 || col != 2 || m.modal != "" || m.sidebar || m.document() != d || !d.Dirty() {
		t.Fatalf("jump changed state: %d:%d", row, col)
	}
	before := d.Cursor
	press(m, ":")
	press(m, "esc")
	if d.Cursor != before || m.sidebar {
		t.Fatal("cancel moved cursor or focus")
	}
	press(m, "i")
	press(m, ":")
	if m.modal != "" || !strings.Contains(d.Text, ":") {
		t.Fatal("Insert mode colon opened picker")
	}
	press(m, "esc")
	press(m, "u")
	if d.Text != "first\n中文value\nthird" {
		t.Fatal("jump disturbed undo history")
	}
}

func TestVimFlashTargetsAndRepeatThroughWorkbench(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "a:n/a:n", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false

	for _, tt := range []struct {
		query  string
		cursor int
	}{{":", 1}, {"n", 2}, {"/", 3}} {
		press(m, "f")
		press(m, tt.query)
		press(m, "enter")
		if m.modal != "" || m.flash != nil || d.Cursor != tt.cursor {
			t.Fatal("Flash input opened a conflicting modal")
		}
	}
	d.Edit("one two")
	d.SetPosition(0, 0)
	press(m, "i")
	m.Update(tea.PasteMsg{Content: "你好"})
	press(m, "esc")
	press(m, "w")
	press(m, ".")
	if d.Text != "你好one 你好two" || m.tabs[0].vim.InsertMode {
		t.Fatal("repeat after bracketed paste:", d.Text)
	}
	press(m, "u")
	if d.Text != "你好one two" {
		t.Fatal("repeat was not one undo step:", d.Text)
	}
}

func TestVimChangeSurroundThroughWorkbench(t *testing.T) {
	m := flashModel(t, "(one) (two)")
	m.document().SetPosition(0, 2)
	for _, key := range []string{"c", "s", "("} {
		press(m, key)
	}
	if m.document().Text != "(one) (two)" || !m.tabs[0].vim.CommandPending() || m.flash != nil {
		t.Fatal("surround prefix changed text or opened Flash")
	}
	press(m, "[")
	if m.document().Text != "[one] (two)" || m.tabs[0].vim.InsertMode {
		t.Fatal("cs([ failed")
	}
	m.document().SetPosition(0, 8)
	press(m, ".")
	if m.document().Text != "[one] [two]" {
		t.Fatal("surround repeat failed")
	}
	press(m, "u")
	if m.document().Text != "[one] (two)" {
		t.Fatal("surround repeat undo failed")
	}
}

func TestVimSurroundThroughWorkbench(t *testing.T) {
	m := flashModel(t, "one two\nnext\n")
	for _, key := range []string{"y", "s", "i", "w", "\""} {
		press(m, key)
	}
	if m.document().Text != "\"one\" two\nnext\n" {
		t.Fatal("word surround failed")
	}
	for _, key := range []string{"d", "s", "\""} {
		press(m, key)
	}
	if m.document().Text != "one two\nnext\n" {
		t.Fatal("delete surround failed")
	}
	m.document().SetPosition(0, 0)
	press(m, "V")
	press(m, "S")
	press(m, "<")
	if m.document().Text != "<one two>\nnext\n" || m.tabs[0].vim.CommandPending() {
		t.Fatal("angle surround failed:", m.document().Text)
	}
	press(m, "u")
	if m.document().Text != "one two\nnext\n" {
		t.Fatal("angle surround undo failed")
	}
	m.document().SetPosition(0, 0)
	press(m, "y")
	press(m, "s")
	press(m, "f")
	if m.flash == nil || m.flash.operator != "ys" {
		t.Fatal("ysf did not open Flash")
	}
	m.Update(tea.PasteMsg{Content: "two"})
	press(m, "enter")
	press(m, "]")
	if m.document().Text != "[one t]wo\nnext\n" {
		t.Fatal("ysf surround failed:", m.document().Text)
	}
}
