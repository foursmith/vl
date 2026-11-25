package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
)

func TestInputUsesExplicitBindings(t *testing.T) {
	input := newInput()
	input.Focus()
	input.SetValue("one two")
	input.buffer.SelectRange(3, 3)
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeyBackspace, Mod: tea.ModAlt},
		{Code: tea.KeyDelete, Mod: tea.ModCtrl},
		{Code: 'h', Mod: tea.ModCtrl},
	} {
		input.update(msg, nil)
		if input.Value() != "one two" || input.Position() != 3 || input.buffer.Anchor != -1 {
			t.Fatalf("%s changed input to %q at %d with anchor %d", msg, input.Value(), input.Position(), input.buffer.Anchor)
		}
	}
	input.update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl}, nil)
	if input.Position() != 0 {
		t.Fatalf("Ctrl+Left moved the cursor to %d, want 0", input.Position())
	}
	input.update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl | tea.ModShift}, nil)
	if got := input.buffer.SelectedText(); got != "one " {
		t.Fatalf("Ctrl+Shift+Right selected %q, want %q", got, "one ")
	}
}

func TestInputEditsWholeGraphemes(t *testing.T) {
	input := newInput()
	input.Focus()
	input.SetValue("中e\u0301👩‍💻")
	input.update(tea.KeyPressMsg{Code: tea.KeyLeft}, nil)
	input.update(tea.KeyPressMsg{Code: tea.KeyBackspace}, nil)
	if got := input.Value(); got != "中👩‍💻" {
		t.Fatalf("Backspace left %q", got)
	}
	input.update(tea.KeyPressMsg{Code: tea.KeyDelete}, nil)
	if got := input.Value(); got != "中" {
		t.Fatalf("Delete left %q", got)
	}
	input.SetValue("中\uFFFD")
	input.update(tea.PasteMsg{Content: "\uFFFD"}, nil)
	if got := input.Value(); got != "中\uFFFD\uFFFD" {
		t.Fatalf("replacing and pasting input left %q", got)
	}
	input.SetValue(strings.Repeat("a", inputRuneLimit-1))
	input.update(tea.PasteMsg{Content: "e\u0301"}, nil)
	if len(input.Value()) != inputRuneLimit-1 {
		t.Fatal("pasting at the input limit split a grapheme")
	}
}

func TestTextEditingShortcutsFollowInputFocus(t *testing.T) {
	for _, test := range []struct {
		name  string
		modal string
		find  bool
		focus int
	}{
		{name: "picker", modal: "quick"},
		{name: "commit message", modal: "commit"},
		{name: "file name", modal: "newFile"},
		{name: "workspace query", focus: 0},
		{name: "workspace replacement", focus: 1},
		{name: "workspace include", focus: 3},
		{name: "workspace exclude", focus: 4},
		{name: "file query", find: true, focus: 0},
		{name: "file replacement", find: true, focus: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := newHoverRenderModel(t)
			var input *textInput
			switch {
			case test.modal != "":
				m.prompt(test.modal, "Prompt", "")
				input = &m.input
			case test.find:
				m.tabs = []tab{{doc: &editor.Document{Buffer: editor.Buffer{Text: "example", Anchor: -1}}}}
				m.activeTab = 0
				m.openFileReplace()
				m.focusFileFind(test.focus)
				input = m.findWidget.field()
			default:
				m.openSearchAt(test.focus)
				m.search.replace, m.search.details = true, true
				input = m.searchField(test.focus)
			}
			expect := func(want string) {
				t.Helper()
				if got := input.Value(); got != want {
					t.Fatalf("input = %q, want %q", got, want)
				}
			}
			input.SetValue("例text")
			input.buffer.SelectRange(1, 1)
			m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModCtrl})
			expect("text")
			press(m, "ctrl+z")
			expect("例text")
			m.Update(tea.KeyPressMsg{Code: tea.KeyDelete, Mod: tea.ModCtrl})
			expect("例")
			press(m, "ctrl+z")
			expect("例text")
			press(m, "ctrl+a")
			if start, end := input.buffer.Selection(); start != 0 || end != 5 {
				t.Fatalf("selection = [%d, %d)", start, end)
			}
			if press(m, "ctrl+c") == nil {
				t.Fatal("copy did not request the clipboard")
			}
			expect("例text")
			if press(m, "ctrl+x") == nil {
				t.Fatal("cut did not request the clipboard")
			}
			expect("例text")
			// Deliver clipboard results without accessing the user's clipboard.
			m.Update(clipboardMsg{text: "例text", apply: input.clipboardTarget(true)})
			expect("")
			press(m, "ctrl+z")
			expect("例text")
			m.Update(tea.KeyPressMsg{Code: 'Z', Mod: tea.ModCtrl | tea.ModShift})
			expect("")
			press(m, "x")
			press(m, "ctrl+a")
			if press(m, "ctrl+v") == nil {
				t.Fatal("paste did not request the clipboard")
			}
			m.Update(clipboardMsg{text: "new\ntext", apply: input.clipboardTarget(false)})
			expect("new text")
			press(m, "ctrl+z")
			expect("x")
			m.Update(tea.PasteMsg{Content: "terminal paste"})
			expect("terminal paste")
			press(m, "ctrl+z")
			expect("x")
			if test.find && test.focus == 0 && m.findWidget.query.Value() != "x" {
				t.Fatalf("file search still uses %q after undo", m.findWidget.query.Value())
			}
			if test.modal == "" && !test.find && test.focus == 0 && m.search.options.Query != "x" {
				t.Fatalf("workspace search still uses %q after undo", m.search.options.Query)
			}
		})
	}
}

func TestInputSelectionAndStaleClipboardAfterSwitchingFields(t *testing.T) {
	m := newHoverRenderModel(t)
	m.openSearch()
	m.search.replace = true
	field := &m.search.query
	field.SetValue("中e\u0301")
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	if start, end := field.buffer.Selection(); start != 1 || end != 3 {
		t.Fatalf("selection = [%d, %d), want the last grapheme", start, end)
	}
	m.monochrome = true
	cells := uv.NewStyledString(m.inputView(*field, inputArea{width: 10})).Lines(ansi.GraphemeWidth)[0]
	for _, cell := range cells {
		if (cell.Style.Attrs&uv.AttrReverse != 0) != (cell.Content == "e\u0301") {
			t.Fatalf("unexpected selection highlight on %q", cell.Content)
		}
	}
	press(m, "x")
	if got := field.Value(); got != "中x" {
		t.Fatalf("replacing selection produced %q", got)
	}
	press(m, "ctrl+v")
	late := clipboardMsg{text: "late", apply: field.clipboardTarget(false)}
	m.focusSearch(1)
	m.Update(late)
	m.focusSearch(0)
	m.Update(late)
	if field.Value() != "中x" || m.search.replacement.Value() != "" {
		t.Fatal("a delayed paste changed a field after its focus changed")
	}
}
