package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestWorkbenchFocusShortcutAndRemovedBindings(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	m.openFile("file0.go")
	m.openFile("file1.go")
	for _, key := range []tea.KeyPressMsg{
		{Code: 'p', Mod: tea.ModCtrl | tea.ModShift},
		{Code: 'h', Mod: tea.ModCtrl},
		{Code: tea.KeyF1},
		{Code: 'f', Mod: tea.ModCtrl | tea.ModShift},
		{Code: tea.KeyF10, Mod: tea.ModShift},
		{Code: tea.KeyLeft, Mod: tea.ModAlt},
		{Code: tea.KeyRight, Mod: tea.ModAlt},
		{Code: tea.KeyF6},
	} {
		m.Update(key)
		if m.modal != "" || m.contextMenu != nil || m.sidebar || len(m.tabs) != 2 || m.activeTab != 1 {
			t.Fatalf("%s changed the dialog, focus, or active tab", key.String())
		}
	}
	position := statusPart(t, m, "position")
	m.Update(tea.MouseClickMsg{X: position.x + 1, Y: m.height - 1, Button: tea.MouseLeft})
	if m.modal != "" {
		t.Fatal("clicking the cursor position opened a dialog")
	}

	m.openSearch()
	focusKey := tea.KeyPressMsg{Code: tea.KeyF2}
	m.Update(focusKey)
	if m.sidebar {
		t.Fatal("F2 did not move focus out of workspace Search")
	}
	m.openFileFind()
	m.Update(focusKey)
	if !m.sidebar || m.fileFindFocused() {
		t.Fatal("F2 did not move focus from Find to the sidebar")
	}
}

func TestTreeVimNavigationAndFileOperators(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	for _, k := range []string{"g", "g", "2", "j"} {
		drain(t, m, press(m, k))
	}
	if m.selected != 2 {
		t.Fatalf("2j selected %d", m.selected)
	}
	press(m, "G")
	if m.selected != len(m.items)-1 {
		t.Fatal("G")
	}
	press(m, "3")
	press(m, "G")
	if m.selected != 2 {
		t.Fatal("3G")
	}
	drain(t, m, press(m, "l"))
	if !m.expanded["cmd"] {
		t.Fatal("l did not expand folder")
	}
	drain(t, m, press(m, "h"))
	if !m.current().header || !m.expanded["cmd"] {
		t.Fatal("h should select the workspace root and leave cmd expanded")
	}
	drain(t, m, press(m, "2"))
	drain(t, m, press(m, "j"))
	drain(t, m, press(m, "l"))
	if m.expanded["cmd"] {
		t.Fatal("l did not collapse the expanded folder")
	}
	for i, e := range m.items {
		if e.file == "README.md" {
			m.selected = i
		}
	}
	press(m, "y")
	press(m, "y")
	press(m, "p")
	if m.modal != "pasteFile" {
		t.Fatal("yy p did not prompt")
	}
	m.input.SetValue("copied.md")
	drain(t, m, press(m, "enter"))
	data, err := os.ReadFile(filepath.Join(m.repo.Root, "copied.md"))
	if err != nil || string(data) != "# Workspace\n" {
		t.Fatalf("copy: %q %v", data, err)
	}
	for i, e := range m.items {
		if e.file == "copied.md" {
			m.selected = i
		}
	}
	press(m, "d")
	press(m, "d")
	if m.modal != "confirm" {
		t.Fatal("dd bypassed confirmation")
	}
	press(m, "esc")
	if _, err = os.Stat(filepath.Join(m.repo.Root, "copied.md")); err != nil {
		t.Fatal("cancel deleted file")
	}
	// Unfinished operators cannot survive a focus switch.
	press(m, "d")
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	press(m, "d")
	if m.modal != "" {
		t.Fatal("unfinished dd crossed focus switch")
	}
}

func TestHalfPageScrollInAllEditorModes(t *testing.T) {
	for _, mode := range []string{"normal", "i", "v", "V"} {
		t.Run(mode, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 25})
			m.openFile("file0.go")
			tab := &m.tabs[m.activeTab]
			d := tab.doc
			original := strings.Repeat("    text\n", 100)
			d.Text = original
			if mode != "normal" {
				press(m, mode)
			}
			for _, step := range []struct {
				key  string
				want int
			}{{"ctrl+d", m.contentHeight() / 2}, {"ctrl+u", 0}} {
				press(m, step.key)
				line, _ := d.Position()
				if line != step.want || tab.editorDisplayTop() != step.want || d.Text != original {
					t.Fatalf("%s: cursor=%d scroll=%d, want %d with original text", step.key, line, tab.editorDisplayTop(), step.want)
				}
				if tab.vim.InsertMode != (mode == "i") || tab.vim.VisualMode != (mode == "v" || mode == "V") {
					t.Fatalf("%s changed editor mode", step.key)
				}
				if tab.vim.VisualMode {
					start, _ := d.Selection()
					if start != 0 {
						t.Fatalf("%s moved the selection anchor", step.key)
					}
				}
			}
		})
	}
}

func TestDiffVimNavigationAndShortcutPriority(t *testing.T) {
	m := reviewWorkspace(t, false)
	press(m, "3")
	press(m, "j")
	if row, _ := m.review.Position(); row != 3 {
		t.Fatal("3j did not move the read-only cursor")
	}
	press(m, "G")
	if row, _ := m.review.Position(); row != len(m.review.Lines())-1 || m.reviewCursor() == nil {
		t.Fatal("G did not reveal the last source line")
	}
	press(m, "g")
	press(m, "g")
	if row, _ := m.review.Position(); row != 0 {
		t.Fatal("gg")
	}
	split := m.split
	press(m, "ctrl+d")
	if row, _ := m.review.Position(); row == 0 || m.split != split || !m.review.ReadOnly {
		t.Fatal("Ctrl+D must scroll the read-only diff")
	}
	press(m, "ctrl+u")
	if row, _ := m.review.Position(); row != 0 {
		t.Fatal("Ctrl+U must scroll back to the first row")
	}
	for _, action := range m.commandActions() {
		if action.id == string(commandDiffLayout) {
			if action.key != "" {
				t.Fatalf("Toggle Inline View still advertises %q", action.key)
			}
			action.run()
			break
		}
	}
	if m.split == split || !m.review.ReadOnly {
		t.Fatal("Toggle Inline View must still change the layout from the palette")
	}
	press(m, "ctrl+f")
	if !m.fileFindFocused() || m.modal != "" {
		t.Fatal("Ctrl+F must open the shared find widget")
	}
}

func TestVimFileOperatorsProtectDirtyFilesAndOtherSections(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.openFile("README.md")
	m.sidebar = true
	for i, e := range m.items {
		if e.file == "README.md" {
			m.selected = i
		}
	}
	m.document().Insert("unsaved")
	press(m, "y")
	press(m, "y")
	if m.explorerYankPath != "" {
		t.Fatal("copied dirty file")
	}
	press(m, "d")
	press(m, "d")
	if m.modal != "" {
		t.Fatal("allowed dirty file deletion")
	}
	m.document().Undo()
	drain(t, m, m.selectSection(changes))
	selectChanged(t, m, "README.md", false)
	press(m, "d")
	press(m, "d")
	if m.modal != "confirm" || len(m.confirmationActions) != 2 || m.confirmationActions[0].label != m.tr("Discard") || m.busy {
		t.Fatal("dd should only open the Git discard confirmation in Changes")
	}
}

func TestMenuVimCountsAndEdges(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.contextMenu = &contextMenu{actions: []action{{label: "a"}, {label: "b"}, {label: "c"}, {label: "d"}}}
	press(m, "2")
	press(m, "j")
	if m.contextMenu.selected != 2 {
		t.Fatal("menu 2j")
	}
	press(m, "g")
	press(m, "g")
	if m.contextMenu.selected != 0 {
		t.Fatal("menu gg")
	}
	press(m, "G")
	if m.contextMenu.selected != 3 {
		t.Fatal("menu G")
	}
	press(m, "h")
	if m.contextMenu != nil {
		t.Fatal("menu h")
	}
}
