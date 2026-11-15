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
	if m.modal != "files" || m.input.Prompt != "" {
		t.Fatal("Ctrl+P must open file search")
	}
	m.input.SetValue("file1")
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "file1.go" {
		t.Fatalf("file filtering: %+v", rows)
	}
	m.View()
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 1, Button: tea.MouseLeft})
	if m.modal != "files" {
		t.Fatal("clicking input dismissed file search")
	}
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
	if m.modal != "" || m.document() == nil || !strings.HasSuffix(m.document().Path, "file1.go") {
		t.Fatal("file result click did not open the filtered file")
	}
	m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl | tea.ModShift})
	if m.modal != "palette" || m.input.Prompt != "> " || len(m.palette) != len(m.actions()) {
		t.Fatal("Ctrl+Shift+P must open commands from editor focus")
	}
}

func TestQuickPickerGeometryAndScrolledClick(t *testing.T) {
	m := tabWorkspace(t, 0)
	for _, width := range []int{30, 48, 120} {
		for _, height := range []int{8, 24} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: height})
			m.openPalette()
			m.choice = len(m.palette) - 1
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
