package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestQuickPositionCurrentAndOtherFile(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.snapshot.Files = []string{"file0.go", "file1.go"}
	m.openFile("file0.go")
	original := m.document()
	original.Edit("first\n中文value\nlast")
	m.sidebarVisible = false
	m.section = history
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: ":2:3"})
	drain(t, m, press(m, "enter"))
	line, col := original.Position()
	if line != 1 || col != 2 || m.sidebarVisible || m.sidebar || m.section != history || !original.Dirty() {
		t.Fatalf("current-file jump disturbed state: line=%d col=%d", line, col)
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, "file1.go"), []byte("first\nsecond\nthird"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: "file1.go:3:2"})
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != "file1.go" || rows[0].key != "Ln 3, Col 2" {
		t.Fatalf("location suffix filtered out file: %+v", rows)
	}
	l := m.pickerLayout(rows)
	_, cmd := m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
	drain(t, m, cmd)
	line, col = m.document().Position()
	if line != 2 || col != 1 || !strings.HasSuffix(m.document().Path, "file1.go") || !original.Dirty() || m.lastCommand != "" {
		t.Fatal("file jump lost position, unsaved document, or command history")
	}
}

func TestQuickPositionInvalidInputAndFailedOpen(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.snapshot.Files = []string{"file0.go", "missing.go"}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: ":1"})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].disabled == "" {
		t.Fatal("jump without a document must explain how to proceed")
	}
	m.openFile("file0.go")
	d := m.document()
	for _, query := range []string{":", ":0", ":2:0", ":2:x", ":1:2:3", ":9999999999999999999999999"} {
		m.openQuickFiles()
		m.Update(tea.PasteMsg{Content: query})
		press(m, "enter")
		if m.modal != "quick" || d.Cursor != 0 || m.notice == "" {
			t.Fatalf("invalid position closed picker or moved cursor: %q", query)
		}
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: "missing.go:100"})
	drain(t, m, press(m, "enter"))
	if m.document() != d || d.Cursor != 0 {
		t.Fatal("failed file open moved the previous document's cursor")
	}
}

func TestQuickPositionLiteralFilenameAndNarrowLayout(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	path := "part:2"
	if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("one\ntwo\nthree"), 0644); err != nil {
		t.Fatal(err)
	}
	m.snapshot.Files = []string{path}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: path})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != path || rows[0].key != "" {
		t.Fatal("literal filename was interpreted as a location")
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: path + ":3"})
	for _, width := range []int{30, 48, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) != width {
				t.Fatalf("position hint overflowed terminal width %d", width)
			}
		}
	}
	drain(t, m, press(m, "enter"))
	line, col := m.document().Position()
	if line != 2 || col != 0 {
		t.Fatalf("colon filename location: %d:%d", line, col)
	}
}
