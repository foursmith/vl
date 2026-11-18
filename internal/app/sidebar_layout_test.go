package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
)

func TestSidebarDragBothSidesAndBounds(t *testing.T) {
	for _, right := range []bool{false, true} {
		m := tabWorkspace(t, 1)
		m.openFile("file0.go")
		m.SetSidebarLayout(30, right)
		selected, focus, doc := m.selected, m.sidebar, m.document()
		m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: 0, Button: tea.MouseLeft})
		if !m.resizingSidebar {
			t.Fatal("divider click did not start resizing")
		}
		x := 47
		if right {
			x = m.width - 48
		}
		m.Update(tea.MouseMotionMsg{X: x, Y: 4, Button: tea.MouseLeft})
		if m.sideWidth() != 47 || m.viewport.Width() != m.mainWidth() {
			t.Fatalf("drag did not resize panes: %d", m.sideWidth())
		}
		m.Update(tea.MouseReleaseMsg{X: x, Y: 4, Button: tea.MouseLeft})
		m.Update(tea.MouseMotionMsg{X: 0, Y: 4})
		if m.resizingSidebar || m.sideWidth() != 47 || m.selected != selected || m.sidebar != focus || m.document() != doc {
			t.Fatal("drag changed selection/focus/document or continued after release")
		}
		for _, x := range []int{-100, 1000} {
			m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: 5, Button: tea.MouseLeft})
			m.Update(tea.MouseMotionMsg{X: x, Y: 5, Button: tea.MouseLeft})
			if m.sideWidth() < 28 || m.mainWidth() < 40 {
				t.Fatal("drag violated pane minimum widths")
			}
			m.Update(tea.MouseReleaseMsg{X: x, Y: 5, Button: tea.MouseLeft})
		}
		m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: 5, Button: tea.MouseLeft})
		m.Update(tea.BlurMsg{})
		m.Update(tea.MouseMotionMsg{X: 50, Y: 5, Button: tea.MouseLeft})
		if m.resizingSidebar {
			t.Fatal("blur did not cancel drag")
		}
	}
}

func TestSidebarRightGeometryAndMouseTargets(t *testing.T) {
	m := explorerWorkspace(t)
	m.SetSidebarLayout(38, true)
	m.openFile("README.md")
	for _, width := range []int{30, 69, 70, 120, 180} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		for y, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) != width {
				t.Fatalf("width %d row %d: %q", width, y, plain(line))
			}
		}
		if m.sideShown() {
			if m.mainX() != 0 || m.sidebarAt(0) || m.sidebarAt(m.dividerX()) || !m.sidebarAt(m.sideX()) {
				t.Fatal("right-side hit regions overlap")
			}
		} else if m.sideX() != 0 || m.mainWidth() != width {
			t.Fatal("narrow layout did not use full terminal")
		}
	}
	if m.sideWidth() != 38 {
		t.Fatal("temporary terminal shrink lost preferred width")
	}
	m.Update(tea.MouseClickMsg{X: 7, Y: editorContentRow, Button: tea.MouseLeft})
	line, col := m.document().Position()
	if m.sidebar || line != 0 || col != 0 {
		t.Fatal("right sidebar broke editor click coordinates")
	}
	for _, nav := range m.navigation() {
		if nav.section == changes {
			_, cmd := m.Update(tea.MouseClickMsg{X: m.sideX() + nav.x + 1, Y: 0, Button: tea.MouseLeft})
			drain(t, m, cmd)
		}
	}
	if m.section != changes {
		t.Fatal("right sidebar navigation was routed to editor tabs")
	}
	a, y := hoverAction(t, m, "cmd/main.go", "stage")
	if m.hoveredChangeRow() != m.selected || m.hoverHint() != a.label {
		t.Fatal("right sidebar hover coordinates missed")
	}
	_, cmd := m.Update(tea.MouseClickMsg{X: m.sideX() + a.x + 1, Y: y, Button: tea.MouseLeft})
	drain(t, m, cmd)
	if m.current() == nil || !m.current().staged || m.current().change.Path != "cmd/main.go" {
		t.Fatal("right sidebar stage button missed")
	}
	for _, slot := range m.tabLayout() {
		if slot.control == 0 && slot.index == m.activeTab {
			m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + slot.width - 3, Y: tabRow, Button: tea.MouseLeft})
			break
		}
	}
	if m.document() != nil {
		t.Fatal("right layout tab close missed")
	}
	press(m, "ctrl+l")
	if m.sideShown() || m.mainWidth() != m.width {
		t.Fatal("hidden sidebar still takes space")
	}
	press(m, "ctrl+l")
	if !m.sideShown() || !m.sidebarRight || m.sideWidth() != 38 {
		t.Fatal("toggle lost sidebar layout")
	}
	// Wheel events must scroll the sidebar without changing selection or preview.
	for i := 0; i < 50; i++ {
		m.items = append(m.items, item{label: "file"})
	}
	selected, doc := m.selected, m.document()
	m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: 5, Button: tea.MouseWheelDown})
	if m.sideTop == 0 || m.selected != selected || m.document() != doc {
		t.Fatal("right sidebar wheel changed target or did not scroll")
	}
}

func TestSidebarLayoutPaletteAndDragCancellation(t *testing.T) {
	m := tabWorkspace(t, 0)
	for _, id := range []string{"view.sidebarRight", "view.sidebarLeft", "view.sidebarResetWidth", "view.sidebar"} {
		m.SetSidebarLayout(45, false)
		m.openPalette()
		query := id
		if id == "view.sidebar" {
			query = "View: Toggle Sidebar"
		}
		m.input.SetValue(">" + query)
		press(m, "enter")
		if m.modal != "" {
			t.Fatal("command did not dismiss palette")
		}
		if id == "view.sidebar" && (m.sidebarVisible || m.sidebar) {
			t.Fatal("toggle command did not hide the sidebar")
		}
		if id == "view.sidebarRight" && !m.sidebarRight {
			t.Fatal("right command failed")
		}
		if id == "view.sidebarLeft" && m.sidebarRight {
			t.Fatal("left command failed")
		}
		if id == "view.sidebarResetWidth" && m.sidebarWidth != 0 {
			t.Fatal("reset failed")
		}
	}
	press(m, "ctrl+l")
	m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: 3, Button: tea.MouseLeft})
	m.openPalette()
	if m.resizingSidebar {
		t.Fatal("modal did not cancel drag")
	}
	press(m, "esc")
	m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: 3, Button: tea.MouseLeft})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	if m.resizingSidebar {
		t.Fatal("terminal resize did not cancel drag")
	}
}

func TestLayoutRestoresPreferencesAcrossWorkspaces(t *testing.T) {
	m := newHoverRenderModel(t)
	want := config.Layout{SidebarWidth: 45, SidebarRight: true, Theme: "light_modern", LightTheme: "light_modern", LastCommand: "view.sidebar"}
	m.SetLayout(want)
	want = m.Layout()
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})
	path := filepath.Join(t.TempDir(), "layout.json")
	if err := config.SaveLayout(path, m.Layout()); err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadLayout(path)
	if err != nil {
		t.Fatal(err)
	}
	next := newHoverRenderModel(t)
	next.workspaces = []string{next.repo.Root}
	next.SetLayout(saved)
	if got := next.Layout(); got != want || !next.colorTheme.IsLight() {
		t.Fatalf("restored layout: %+v", got)
	}
}
