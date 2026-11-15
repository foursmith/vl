package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
				if !strings.Contains(content, a.label) || !strings.Contains(content, a.key) {
					t.Fatalf("welcome shortcut missing in %dx%d: %s", size.w, size.h, a.label)
				}
			}
			_, _, _, logoRows := m.welcomeLayout()
			if logoRows == 1 && !strings.Contains(content, "ELEGIT") {
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
			m.Update(tea.MouseClickMsg{X: m.mainX() + x + 1, Y: editorContentRow + y + logoRows + 1 + i, Button: tea.MouseLeft})
			switch a.command {
			case "commands":
				if m.modal != "palette" {
					t.Fatal("commands link missed")
				}
			case "files":
				if m.modal != "files" {
					t.Fatal("file search link missed")
				}
			case "recent":
				if m.modal != "recent" {
					t.Fatal("recent folders link missed")
				}
			case "changes":
				if m.section != changes || !m.sidebar {
					t.Fatal("source control link missed")
				}
			}
		}
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
	if !strings.Contains(toolbar, "file0.go") || strings.Contains(toolbar, "载入中") {
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
