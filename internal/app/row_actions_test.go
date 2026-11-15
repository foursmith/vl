package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func hoverAction(t *testing.T, m *Model, path, id string) (rowAction, int) {
	t.Helper()
	index := -1
	for i, row := range m.items {
		if row.change != nil && row.change.Path == path {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("missing changed file %s", path)
	}
	m.selected = index
	m.ensureSidebar()
	y := sidebarHeaderRows + index - m.sideTop
	for _, action := range m.changeRowActions(m.items[index], m.sideWidth()) {
		if action.id == id {
			m.Update(tea.MouseMotionMsg{X: m.sideX() + action.x + 1, Y: y})
			return action, y
		}
	}
	t.Fatalf("missing %s action", id)
	return rowAction{}, 0
}

func TestHoverActionsAppearWithoutChangingSelection(t *testing.T) {
	m := explorerWorkspace(t)
	m.nerd = false
	drain(t, m, m.selectSection(changes))
	selected := m.selected
	index := -1
	for i, row := range m.items {
		if row.change != nil && row.change.Path == "cmd/main.go" {
			index = i
			break
		}
	}
	y := sidebarHeaderRows + index - m.sideTop
	if strings.Contains(plain(m.sidebarLines()[y]), m.symbol(symbolOpen)) {
		t.Fatal("actions visible before hover")
	}
	m.Update(tea.MouseMotionMsg{X: 7, Y: y})
	if m.selected != selected || !strings.Contains(plain(m.sidebarLines()[y]), m.symbol(symbolOpen)) || !strings.Contains(plain(m.sidebarLines()[y]), m.symbol(symbolDiscard)) {
		t.Fatal("hover changed selection or did not reveal actions")
	}
	if m.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("terminal does not report hover events")
	}
	for _, line := range m.sidebarLines() {
		if ansi.StringWidth(line) != m.sideWidth() {
			t.Fatal("hover row overflowed sidebar")
		}
	}
	m.Update(tea.MouseMotionMsg{X: m.mainX() + 2, Y: y})
	if strings.Contains(plain(m.sidebarLines()[y]), m.symbol(symbolOpen)) {
		t.Fatal("actions remained after leaving row")
	}
}

func TestHoverStageUnstageAndOpenUseClickedFile(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	a, y := hoverAction(t, m, "cmd/main.go", "stage")
	_, cmd := m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("stage did not schedule work")
	}
	drain(t, m, cmd)
	if m.current() == nil || !m.current().staged || m.current().change.Path != "cmd/main.go" {
		t.Fatal("hover stage lost target identity")
	}
	for _, action := range m.changeRowActions(*m.current(), m.sideWidth()) {
		if action.id == "discard" {
			t.Fatal("index row exposes a worktree discard action")
		}
	}
	a, y = hoverAction(t, m, "cmd/main.go", "unstage")
	_, cmd = m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	drain(t, m, cmd)
	if m.current() == nil || m.current().staged {
		t.Fatal("hover unstage failed")
	}
	a, y = hoverAction(t, m, "cmd/main.go", "open")
	_, cmd = m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	drain(t, m, cmd)
	if !m.showEditor || m.document() == nil || filepath.Base(m.document().Path) != "main.go" {
		t.Fatal("open action did not open file")
	}
	if m.section != explorer || m.sidebar || !m.expanded["cmd"] || m.current() == nil || m.current().file != "cmd/main.go" {
		t.Fatal("open action did not reveal the clicked file in Explorer while keeping editor focus")
	}
}

func TestHoverDiscardRequiresConfirmationAndHonorsCancel(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	path := filepath.Join(m.repo.Root, "cmd/main.go")
	a, y := hoverAction(t, m, "cmd/main.go", "discard")
	m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	if m.modal != "confirm" || !strings.Contains(m.confirmationDetail, "无法恢复") || m.busy {
		t.Fatal("untracked deletion did not require explicit confirmation")
	}
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancel removed untracked file")
	}
	a, y = hoverAction(t, m, "cmd/main.go", "discard")
	m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	drain(t, m, press(m, "enter"))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("confirmed discard did not remove file: %v", err)
	}
}

func TestHoverActionsDisableDuringGitOperation(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	a, y := hoverAction(t, m, "cmd/main.go", "stage")
	m.busy = true
	_, cmd := m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
	if cmd != nil || !strings.Contains(m.notice, "等待") {
		t.Fatal("busy hover button started another operation")
	}
}
