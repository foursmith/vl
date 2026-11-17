package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func chooseContextAction(t *testing.T, m *Model, label string) tea.Cmd {
	t.Helper()
	if m.contextMenu == nil {
		t.Fatal("menu not open")
	}
	for i, a := range m.contextMenu.actions {
		if a.label == label {
			m.contextMenu.selected = i
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return cmd
		}
	}
	t.Fatalf("missing action %q", label)
	return nil
}

func TestContextMenuPreservesSelectionAndConsumesInput(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.document().SelectAll()
	selection := m.document().SelectedText()
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	if m.contextMenu == nil || m.document().SelectedText() != selection {
		t.Fatal("right click changed text selection")
	}
	before := m.document().Text
	press(m, "x")
	m.Update(tea.PasteMsg{Content: "unexpected paste"})
	if m.document().Text != before {
		t.Fatal("menu input reached editor")
	}
	m.Update(tea.MouseMotionMsg{X: m.mainX() + 5, Y: 4})
	if m.hoverOnly {
		t.Fatal("menu hover reuses stale frame")
	}
	press(m, "esc")
	if m.contextMenu != nil {
		t.Fatal("escape did not dismiss menu")
	}
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	m.Update(tea.MouseClickMsg{X: 0, Y: m.height - 1, Button: tea.MouseLeft})
	if m.contextMenu != nil || m.sidebar {
		t.Fatal("outside click leaked through")
	}
}

func TestContextMenuFitsAndScrolls(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.width, m.height = 30, 8
	m.sidebar = false
	m.resize()
	m.Update(tea.MouseClickMsg{X: 29, Y: 6, Button: tea.MouseRight})
	if m.contextMenu == nil {
		t.Fatal("menu missing at edge")
	}
	press(m, "end")
	l := m.contextMenuLayout()
	if l.x < 0 || l.y < 0 || l.x+l.width > m.width || l.y+l.count+2 > m.height || m.contextMenu.top == 0 {
		t.Fatalf("invalid layout: %+v", l)
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatalf("menu frame overflow: %q", plain(line))
		}
	}
	press(m, "esc")
	press(m, "shift+f10")
	if m.contextMenu == nil {
		t.Fatal("keyboard menu did not open")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	if m.contextMenu != nil {
		t.Fatal("resize retained menu")
	}
}

func TestContextMenuTargetsClickedTabAndConfirmsDirtyClose(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.openFile("file0.go")
	first := m.document()
	first.Insert("dirty")
	m.openFile("file1.go")
	slot := m.tabLayout()[0]
	m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseRight})
	if m.document() != first {
		t.Fatal("menu targets active rather than clicked tab")
	}
	chooseContextAction(t, m, "Close")
	if m.contextMenu != nil || m.modal != "close" || len(m.tabs) != 2 {
		t.Fatal("dirty close bypassed confirmation")
	}
}

func TestContextMenuChangeActionsAndStaleTargets(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	_, y := hoverAction(t, m, "cmd/main.go", "stage")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	drain(t, m, chooseContextAction(t, m, "Stage Changes"))
	if m.current() == nil || !m.current().staged || m.current().change.Path != "cmd/main.go" {
		t.Fatal("stage targeted wrong row")
	}
	_, y = hoverAction(t, m, "README.md", "discard")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	chooseContextAction(t, m, "Delete Untracked File")
	if m.modal != "confirm" {
		t.Fatal("discard bypassed confirmation")
	}
	m.modal = ""
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	m.items = nil
	if cmd := chooseContextAction(t, m, "Stage Changes"); cmd != nil {
		t.Fatal("stale menu scheduled mutation")
	}
	if !strings.Contains(m.notice, "Target changed") {
		t.Fatal("stale target not explained")
	}
}

func TestContextMenuMouseAndDisabledActions(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	chooseContextAction(t, m, "Copy")
	if m.contextMenu == nil || m.notice != "Select text first" {
		t.Fatal("disabled action executed or lost its explanation")
	}
	for i, a := range m.contextMenu.actions {
		if a.label != "Select All" {
			continue
		}
		l := m.contextMenuLayout()
		m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 1 + i, Button: tea.MouseLeft})
		if m.contextMenu != nil || m.document().SelectedText() == "" {
			t.Fatal("mouse did not execute menu action")
		}
		return
	}
	t.Fatal("select all missing")
}
