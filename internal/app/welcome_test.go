package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestWelcomeFitsTerminalAndSidebarLayouts(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.sidebar = false
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(38, right)
		for _, size := range []struct{ w, h int }{{30, 8}, {69, 12}, {70, 8}, {100, 18}, {120, 36}} {
			m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) != size.h {
				t.Fatalf("%dx%d: got %d rows", size.w, size.h, len(lines))
			}
			for y, line := range lines {
				if ansi.StringWidth(line) != size.w {
					t.Fatalf("%dx%d row %d width %d", size.w, size.h, y, ansi.StringWidth(line))
				}
			}
			content := ansi.Strip(strings.Join(m.mainLines(), "\n"))
			for _, a := range welcomeActions {
				key := m.commandShortcutHint(a.command)
				if !strings.Contains(content, a.label) || !strings.Contains(content, key) {
					t.Fatalf("welcome shortcut missing in %dx%d: %s", size.w, size.h, a.label)
				}
			}
			_, _, _, logoRows := m.welcomeLayout()
			if logoRows == 1 && !strings.Contains(content, "vl") {
				t.Fatal("compact logo missing")
			}
		}
	}
}

func TestWelcomeClicksUseRenderedCoordinates(t *testing.T) {
	for _, right := range []bool{false, true} {
		for i, a := range welcomeActions {
			m := tabWorkspace(t, 0)
			m.SetSidebarLayout(35, right)
			x, y, _, logoRows := m.welcomeLayout()
			_, cmd := m.Update(tea.MouseClickMsg{X: m.mainX() + x + 1, Y: editorContentRow + y + logoRows + m.welcomeGap(logoRows) + i, Button: tea.MouseLeft})
			switch a.command {
			case "commands":
				if m.modal != "quick" || !m.commandPicker() {
					t.Fatal("commands link missed")
				}
			case "files":
				if m.modal != "quick" || m.commandPicker() {
					t.Fatal("file search link missed")
				}
			case "recent":
				if m.modal != "recent" {
					t.Fatal("recent workspace link missed")
				}
			case "quit":
				if cmd == nil {
					t.Fatal("quit link missed")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("quit link did not quit")
				}
			}
		}
	}
}

func TestWelcomeArtStaysStillWhenTerminalReportsCellSize(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 48})
	before := m.View().Content
	if !strings.Contains(before, m.tr("Drag to rotate")) {
		t.Fatal("test layout did not show the 3D logo")
	}
	for _, size := range []uv.CellSizeEvent{{Width: 8, Height: 20}, {Width: 12, Height: 20}} {
		m.Update(size)
		if m.cellWidth != size.Width || m.cellHeight != size.Height {
			t.Fatal("pixel dimensions were not recorded")
		}
		if m.View().Content != before {
			t.Fatal("pixel-size report changed the welcome frame")
		}
	}
}

func TestWelcomeArtDragRotatesUntilRelease(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 132, Height: 34})
	_, y, _, height := m.welcomeLayout()
	x := m.mainX() + m.mainWidth()/2
	y += m.contentRow() + height/2
	before := m.View().Content
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x + 12, Y: y + 3, Button: tea.MouseLeft})
	rotated := m.View().Content
	if !m.welcomeOrbit.dragging || m.welcomeOrbit.yaw == 0 || m.welcomeOrbit.pitch == 0 || rotated == before {
		t.Fatal("drag did not render a new orbit angle")
	}
	m.Update(tea.MouseReleaseMsg{X: x + 12, Y: y + 3, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x + 18, Y: y + 5, Button: tea.MouseNone})
	if m.welcomeOrbit.dragging || m.View().Content != rotated {
		t.Fatal("art continued to rotate after release")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.View().Content == rotated {
		t.Fatal("focused art did not rotate with the keyboard")
	}
}

func TestWelcomeDoesNotReplaceOpenOrLoadingContent(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.diffLoading = true
	showDiffFixture(m)
	m.diffPath = "file0.go"
	if m.welcomeVisible() || strings.Contains(plain(strings.Join(m.mainLines(), "\n")), "Loading diff") {
		t.Fatal("pending diff shows welcome or a loading placeholder")
	}
	toolbar := plain(m.contentToolbar())
	if !strings.Contains(toolbar, "file0.go") || strings.Contains(toolbar, "Loading") {
		t.Fatal("pending diff hides its path behind a loading placeholder")
	}
	m.diffLoading = false
	showDiffFixture(m)
	if m.welcomeVisible() {
		t.Fatal("welcome replaced empty diff")
	}
	m.closeTab()
	if err := os.WriteFile(filepath.Join(m.repo.Root, "empty.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	m.openFile("empty.txt")
	if m.welcomeVisible() {
		t.Fatal("welcome replaced empty editor")
	}
	m.closeTab()
	if !m.welcomeVisible() {
		t.Fatal("last tab close did not restore welcome")
	}
}
