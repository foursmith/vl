package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestQuickFileRevealsTreeAndKeepsEditorFocus(t *testing.T) {
	m := explorerWorkspace(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 8})
	path := "cmd/nested/main.go"
	if err := os.MkdirAll(filepath.Join(m.repo.Root, "cmd/nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.section = changes
	m.sidebarVisible = false
	m.rootCollapsed = true
	m.snapshot.Files = []string{path}
	m.openQuickFiles()
	drain(t, m, press(m, "enter"))
	if m.section != explorer || !m.sidebarVisible || m.sidebar || m.rootCollapsed || !m.expanded["cmd"] || !m.expanded["cmd/nested"] {
		t.Fatal("file search did not reveal ancestors while preserving editor focus")
	}
	if entry := m.current(); entry == nil || entry.file != path {
		t.Fatalf("tree selection: %+v", entry)
	}
	if m.selected < m.sideTop || m.selected >= m.sideTop+m.bodyHeight()-sidebarHeaderRows {
		t.Fatal("revealed file is outside the sidebar viewport")
	}
	// A later navigation must win over an in-flight reveal.
	cmd := m.openFileInExplorer(path)
	m.selectSection(history)
	drain(t, m, cmd)
	if m.section != history || m.revealFile != "" {
		t.Fatal("late tree response restored the old reveal")
	}
}

func TestFailedFileOpenDoesNotRevealPreviousDocument(t *testing.T) {
	m := explorerWorkspace(t)
	m.openFile("README.md")
	doc := m.document()
	drain(t, m, m.selectSection(changes))
	if err := os.WriteFile(filepath.Join(m.repo.Root, "binary.dat"), []byte{0, 1, 2}, 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"missing.go", "binary.dat"} {
		if cmd := m.openFileInExplorer(path); cmd != nil || m.section != changes || m.document() != doc || m.revealFile != "" {
			t.Fatalf("failed open changed Explorer state: %s", path)
		}
	}
}

func TestQuickPickerShortcutsAndFileSelection(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.snapshot.Files = []string{"file0.go", "file1.go"}
	press(m, "ctrl+p")
	if m.modal != "quick" || m.input.Prompt != "" {
		t.Fatal("Ctrl+P must open file search")
	}
	m.input.SetValue("file1")
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "file1.go" {
		t.Fatalf("file filtering: %+v", rows)
	}
	m.View()
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 1, Button: tea.MouseLeft})
	if m.modal != "quick" {
		t.Fatal("clicking input dismissed file search")
	}
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
	if m.modal != "" || m.document() == nil || !strings.HasSuffix(m.document().Path, "file1.go") {
		t.Fatal("file result click did not open the filtered file")
	}
	m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl | tea.ModShift})
	if m.modal != "quick" || m.input.Prompt != "" || m.input.Value() != ">" || len(m.palette) != len(m.actions()) {
		t.Fatal("Ctrl+Shift+P must open commands from editor focus")
	}
}

func TestQuickPickerGeometryAndScrolledClick(t *testing.T) {
	m := tabWorkspace(t, 0)
	for _, width := range []int{30, 48, 120} {
		for _, height := range []int{8, 24} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: height})
			m.openPalette()
			m.selectPickerIndex(len(m.palette) - 1)
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != width {
					t.Fatalf("picker width at %dx%d: %d", width, height, ansi.StringWidth(line))
				}
			}
		}
	}
	executed := false
	m.palette[len(m.palette)-1] = action{label: "Last command", run: func() tea.Cmd { executed = true; return nil }}
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3 + l.count - 1, Button: tea.MouseLeft})
	if !executed || m.modal != "" {
		t.Fatal("scrolled picker click executed the wrong command")
	}
}

func TestCommandPaletteAlphabeticalOrderAndLastUsed(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	checkSorted := func(rows []action) {
		t.Helper()
		for i := 1; i < len(rows); i++ {
			if strings.ToLower(rows[i-1].label) > strings.ToLower(rows[i].label) {
				t.Fatalf("commands out of order: %q before %q", rows[i-1].label, rows[i].label)
			}
		}
	}
	m.SetLastCommand("removed.command")
	m.openPalette()
	checkSorted(m.filteredActions())
	m.input.SetValue(">View: Toggle Sidebar")
	m.selectPickerChoice()
	if m.LastCommand() != "view.sidebar" {
		t.Fatalf("executed command was not remembered: %q", m.LastCommand())
	}
	m.openPalette()
	rows := m.filteredActions()
	if rows[0].id != "view.sidebar" {
		t.Fatalf("last command not first: %q", rows[0].id)
	}
	checkSorted(rows[1:])
	m.input.SetValue(">file.save")
	m.selectPickerChoice()
	if m.LastCommand() != "view.sidebar" {
		t.Fatal("disabled command replaced last used command")
	}
	m.snapshot.Files = []string{"file0.go"}
	m.openQuickFiles()
	m.input.SetValue("file0.go")
	m.selectPickerChoice()
	if m.LastCommand() != "view.sidebar" {
		t.Fatal("file picker selection replaced last used command")
	}
}

func TestQuickPickerSwitchesWithEditableCommandPrefix(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.snapshot.Files = []string{"file0.go"}
	m.openPalette()
	m.selectPickerIndex(5)
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	rows := m.filteredActions()
	if m.modal != "quick" || m.input.Value() != "" || m.input.Prompt != "" || m.pickerList.Index() != 0 || len(rows) != 1 || rows[0].id != "file0.go" {
		t.Fatal("deleting > did not switch to file search and reset selection")
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "file0"})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "file0.go" {
		t.Fatal("typing did not filter file results")
	}
	drain(t, m, press(m, "enter"))
	if m.document() == nil || m.LastCommand() != "" {
		t.Fatal("file selection did not open the file or changed command history")
	}
	m.openQuickFiles()
	m.Update(tea.KeyPressMsg{Code: '>', Text: ">"})
	if !m.commandPicker() || len(m.filteredActions()) != len(m.actions()) {
		t.Fatal("typing > did not switch to commands")
	}
	m.Update(tea.PasteMsg{Content: "View: Toggle Sidebar"})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "view.sidebar" {
		t.Fatal("command prefix was included in the search query")
	}
	drain(t, m, press(m, "enter"))
	if m.LastCommand() != "view.sidebar" {
		t.Fatal("command selection did not update history")
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: ">View: Toggle Sidebar"})
	if rows := m.filteredActions(); !m.commandPicker() || len(rows) != 1 || rows[0].id != "view.sidebar" {
		t.Fatal("pasting > did not switch modes")
	}
	press(m, "ctrl+p")
	if m.commandPicker() || m.input.Value() != "" {
		t.Fatal("Ctrl+P did not reset to file search")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !m.commandPicker() || m.input.Value() != ">" {
		t.Fatal("F1 did not reset to command search")
	}
}
