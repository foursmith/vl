package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/editor"
)

func TestVimSystemClipboardShortcuts(t *testing.T) {
	m := flashModel(t, "one two")
	d := m.document()
	press(m, "v")
	press(m, "e")
	_, cut := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModSuper})
	if cut != nil {
		t.Fatal("Super+X requested the system clipboard")
	}
	_, cut = m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cut == nil || d.SelectedText() != "one" || d.Text != "one two" {
		t.Fatal("cut did not retain the selection while writing to the clipboard")
	}
	m.Update(clipboardMsg{text: "one", apply: documentClipboardTarget(d, true)})
	if d.Text != " two" || m.tabs[0].vim.VisualMode {
		t.Fatal("cut did not remove the selection and exit Visual mode")
	}

	_, paste := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModSuper})
	if paste != nil {
		t.Fatal("Super+V requested the system clipboard")
	}
	_, paste = m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	if paste == nil {
		t.Fatal("paste did not request the system clipboard")
	}
	m.Update(clipboardMsg{text: "one", apply: documentClipboardTarget(d, false)})
	if d.Text != "one two" {
		t.Fatalf("direct clipboard paste produced %q", d.Text)
	}
	d.SetPosition(0, 0)
	press(m, "d")
	m.Update(tea.PasteMsg{Content: "p\n猫"})
	if d.Text != "p\n猫one two" || m.tabs[0].vim.CommandPending() {
		t.Fatal("terminal paste in Normal mode did not insert literal text and clear the pending command")
	}
}

func TestDocumentClipboardDoesNotFollowFocusOrSelection(t *testing.T) {
	m := flashModel(t, "same same")
	d := m.document()
	d.SelectRange(0, 4)
	cut := clipboardMsg{text: "same", apply: documentClipboardTarget(d, true)}
	d.SelectRange(5, 9)
	m.Update(cut)
	if d.Text != "same same" || d.SelectedText() != "same" {
		t.Fatal("a delayed cut removed a different occurrence of the selected text")
	}
	paste := clipboardMsg{text: "late", apply: documentClipboardTarget(d, false)}
	m.openFileFind()
	m.Update(paste)
	if d.Text != "same same" {
		t.Fatal("a delayed paste changed the document while Find was focused")
	}
}

func TestToggleVimModeFromCommandPalette(t *testing.T) {
	m := flashModel(t, "one two")
	m.editorOnly = true
	path := filepath.Join(t.TempDir(), "vl.config.json")
	m.SetPreferencesStore(config.NewPreferencesStore(path))
	m.autoSwitchInputMethod = config.InputMethodSwitch{Enable: true, Cmd: []string{"im-select"}}
	d := m.document()
	press(m, "w")
	press(m, "v")
	toggle := func() {
		t.Helper()
		press(m, "ctrl+p")
		press(m, "Toggle Vim Mode")
		drain(t, m, press(m, "enter"))
	}
	toggle()
	if m.Preferences().Vim.Enabled() || m.wantsEnglishInput() || strings.Contains(plain(m.statusLine()), "VISUAL") {
		t.Fatal("disabling Vim left modal editor state active")
	}
	preferences, err := config.LoadPreferences(path)
	if err != nil || preferences.Vim.Enabled() {
		t.Fatalf("saved Vim preferences = %+v, %v", preferences.Vim, err)
	}
	if cursor := m.View().Cursor; cursor == nil || cursor.Shape != tea.CursorBar {
		t.Fatalf("direct editing cursor = %+v", cursor)
	}
	if got := d.SelectedText(); got != "t" {
		t.Fatalf("disabling Vim changed the character selection to %q", got)
	}
	press(m, "i")
	press(m, " ")
	press(m, "v")
	if got := d.Text; got != "one i vwo" {
		t.Fatalf("direct typing produced %q", got)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	if got := d.SelectedText(); got != "v" {
		t.Fatalf("Shift+Left selected %q", got)
	}
	press(m, "esc")
	if d.SelectedText() != "" {
		t.Fatal("Escape retained the selection")
	}
	press(m, "ctrl+a")
	if d.SelectedText() != d.Text {
		t.Fatal("Ctrl+A did not select all in direct editing")
	}
	toggle()
	if !m.Preferences().Vim.Enabled() || !strings.Contains(plain(m.statusLine()), "NORMAL") {
		t.Fatal("enabling Vim did not restore Normal mode")
	}
	press(m, "i")
	if !m.tabs[m.activeTab].vim.InsertMode || d.Text != "one i vwo" {
		t.Fatal("enabling Vim did not restore modal keystrokes")
	}
}

func TestVimDisabledReadOnlyDiffAndPathCompletion(t *testing.T) {
	m := reviewWorkspace(t, false)
	m.SetPreferences(config.Preferences{Vim: config.VimPreferences{Enable: new(false)}})
	m.focusEditor()
	original := m.review.Text
	press(m, "ctrl+d")
	if row, _ := m.review.Position(); row == 0 {
		t.Fatal("Ctrl+D did not scroll the diff with Vim disabled")
	}
	press(m, "ctrl+u")
	if row, _ := m.review.Position(); row != 0 {
		t.Fatal("Ctrl+U did not return to the start of the diff with Vim disabled")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	if m.review.SelectedText() == "" || m.tabs[m.activeTab].vim.VisualMode {
		t.Fatal("read-only diff did not select text directly")
	}
	press(m, "v")
	press(m, " ")
	if m.review.Text != original || strings.Contains(plain(m.statusLine()), "NORMAL") {
		t.Fatal("read-only diff entered Vim mode or modified text")
	}
	press(m, "]")
	if m.diffHunk != 1 {
		t.Fatal("diff hunk navigation depended on Vim mode")
	}

	m = flashModel(t, "")
	m.SetPreferences(config.Preferences{Vim: config.VimPreferences{Enable: new(false)}})
	m.focusEditor()
	m.document().Path = filepath.Join(m.repo.Root, "example.go")
	press(m, "./")
	if m.pathCompletionDocument() != m.document() {
		t.Fatal("direct typing did not allow path completion")
	}
}

func TestWorkbenchSelectAllBeforeVim(t *testing.T) {
	const text = "one two\n最后一行\n"
	for _, test := range []struct {
		name, start string
		visual      bool
		insert      bool
	}{
		{name: "normal operator", start: "d"},
		{name: "visual", start: "v", visual: true},
		{name: "visual line", start: "V", visual: true},
		{name: "insert", start: "i", insert: true},
		{name: "flash", start: "f"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := flashModel(t, text)
			d := m.document()
			d.SetPosition(0, 4)
			press(m, test.start)
			press(m, "ctrl+a")
			v := &m.tabs[m.activeTab].vim
			if d.SelectedText() != text || v.VisualMode != test.visual || v.InsertMode != test.insert ||
				v.CommandPending() || m.flash != nil {
				t.Fatalf("select all left mode %q, Flash = %v, selection = %q", v.Label(), m.flash != nil, d.SelectedText())
			}
			if press(m, "ctrl+c") == nil || press(m, "ctrl+x") == nil {
				t.Fatal("workbench shortcut did not request the system clipboard")
			}
		})
	}

	m := reviewWorkspace(t, false)
	press(m, "ctrl+a")
	if m.diffSelectedText() != m.review.Text || press(m, "ctrl+c") == nil {
		t.Fatal("read-only diff did not expose its entire text selection for copying")
	}
	for _, shortcut := range []string{"ctrl+x", "ctrl+v", "ctrl+z"} {
		if press(m, shortcut) != nil {
			t.Fatalf("%s requested an edit on a read-only diff", shortcut)
		}
	}
}

func TestVimUndoRedoShortcuts(t *testing.T) {
	undo := tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
	for _, code := range []rune{'z', 'Z'} {
		redo := tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl | tea.ModShift}
		t.Run(redo.String(), func(t *testing.T) {
			m := flashModel(t, "one")
			d := m.document()
			press(m, "i")
			press(m, "X")
			m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModSuper})
			if d.Text != "Xone" {
				t.Fatalf("Super+Z changed the text to %q", d.Text)
			}
			m.Update(undo)
			if d.Text != "one" || !m.tabs[0].vim.InsertMode {
				t.Fatalf("undo in Insert mode produced %q", d.Text)
			}
			m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
			if d.Text != "one" {
				t.Fatalf("Ctrl+Y unexpectedly redid the edit: %q", d.Text)
			}
			m.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModSuper | tea.ModShift})
			if d.Text != "one" {
				t.Fatalf("Super+Shift+Z changed the text to %q", d.Text)
			}
			m.Update(redo)
			if d.Text != "Xone" || !m.tabs[0].vim.InsertMode {
				t.Fatalf("redo in Insert mode produced %q", d.Text)
			}

			press(m, "esc")
			press(m, "d")
			m.Update(undo)
			if d.Text != "one" || m.tabs[0].vim.CommandPending() {
				t.Fatalf("undo in Normal mode produced %q or left an operator pending", d.Text)
			}
		})
	}
}

func TestVimWorkbenchRouting(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Buffer: editor.Buffer{Text: "one two\nthree", Anchor: -1}}
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
	press(m, "/")
	if !m.fileFindFocused() || m.modal != "" {
		t.Fatal("search component did not open")
	}
	press(m, "word")
	if d.Text != "Xone two\nthree" || !strings.Contains(m.findWidget.query.Value(), "word") {
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
	d := &editor.Document{Buffer: editor.Buffer{Text: "abc\n\ndef", Anchor: -1}}
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
	d := &editor.Document{Buffer: editor.Buffer{Text: "alpha beta gamma", Anchor: -1}}
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

func TestVimChineseWordsThroughWorkbench(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(dir, "extra")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	dictionary := []byte("中文 1755 nz\n分词 19 n\n测试 2444 v\n")
	if err := os.WriteFile(filepath.Join(dir, "dict.txt.big"), dictionary, 0600); err != nil {
		t.Fatal(err)
	}
	m := flashModel(t, "中文分词测试 foo")
	d := m.document()
	press(m, "w")
	if d.Cursor != 7 {
		t.Fatalf("default word boundary at %d", d.Cursor)
	}
	m.SetPreferences(config.Preferences{Vim: config.VimPreferences{EnableJieba: true}})
	d.SetPosition(0, 0)
	press(m, "w")
	if d.Cursor != 2 {
		t.Fatalf("Chinese word boundary at %d", d.Cursor)
	}
	for _, key := range []string{"d", "i", "w"} {
		press(m, key)
	}
	if d.Text != "中文测试 foo" || m.vimRegister != "分词" {
		t.Fatalf("text = %q, register = %q", d.Text, m.vimRegister)
	}

	m.setVimDisabled(true)
	x, y := m.mainX()+editorGutterWidth+4, m.contentRow()
	clickText(t, m, x, y)
	clickText(t, m, x, y)
	if got := d.SelectedText(); got != "测试" {
		t.Fatalf("double-click with Vim mode off selected %q", got)
	}
	m.prompt("newFile", "Prompt", "中文测试")
	l := m.dialogLayout()
	x, y = l.x+8, l.y+2
	clickText(t, m, x, y)
	clickText(t, m, x, y)
	if start, end := m.input.buffer.Selection(); start != 2 || end != 4 {
		t.Fatalf("input double-click selected [%d, %d)", start, end)
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
	d := &editor.Document{Buffer: editor.Buffer{Text: "a:n/a:n", Anchor: -1}}
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
	// Han text and the adjacent identifier are separate words.
	press(m, "2")
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
