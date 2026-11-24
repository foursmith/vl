package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
)

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
				m.tabs = []tab{{doc: &editor.Document{Text: "example", Anchor: -1}}}
				m.activeTab = 0
				m.openFileReplace()
				m.focusFileFind(test.focus)
				input = m.searchField(test.focus)
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
			press(m, "ctrl+a")
			if start, end := input.selection(); start != 0 || end != 5 {
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
			if test.modal == "" && test.focus == 0 && m.findOptions.Query != "x" {
				t.Fatalf("search still uses %q after undo", m.findOptions.Query)
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
	field.CursorEnd()
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	if start, end := field.selection(); start != 1 || end != 3 {
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
