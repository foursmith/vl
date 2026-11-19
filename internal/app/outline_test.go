package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestExplorerOutlineNavigation(t *testing.T) {
	m := sidebarListWorkspace(t, explorer)
	m.height = 12
	const source = "Title\n=====\n\n## **安装**\n\n```md\n# ignored\n```\n\n### Linux\n\n## Usage\n\n# Credits\n"
	if err := os.WriteFile(filepath.Join(m.repo.Root, "guide.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if !m.openDocument("guide.md", true) {
		t.Fatal("could not open Markdown preview")
	}
	tab := &m.tabs[m.activeTab]
	_, l := m.outlineLayout()
	want := []outlineEntry{{"Title", 0, 0}, {"安装", 3, 1}, {"Linux", 9, 2}, {"Usage", 11, 1}, {"Credits", 13, 0}}
	if !slices.Equal(tab.markdown.headings, want) {
		t.Fatalf("outline = %#v, want %#v", tab.markdown.headings, want)
	}
	if frame := plain(strings.Join(m.sidebarLines(), "\n")); !strings.Contains(frame, m.tr("Outline")) || !strings.Contains(frame, "│ # 安装") || !strings.Contains(frame, "▎ # Title") {
		t.Fatalf("outline missing from sidebar:\n%s", frame)
	}
	x := m.sideX() + 2
	m.Update(tea.MouseWheelMsg{X: x, Y: l.start, Button: tea.MouseWheelDown})
	if tab.markdown.outlineTop == 0 || m.sideTop != 0 || m.sidebarRowAt(l.start) != -1 {
		t.Fatal("outline row interfered with the file tree")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	press(m, "j")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if line, _ := m.document().Position(); line != 3 || m.sidebar {
		t.Fatalf("keyboard jump left cursor at line %d, sidebar focus = %v", line, m.sidebar)
	}
	m.Update(tea.MouseClickMsg{X: x, Y: l.start + 2, Button: tea.MouseLeft})
	if line, _ := m.document().Position(); line != 9 {
		t.Fatalf("mouse jump left cursor at line %d", line)
	}
	middle := m.contentRow() + m.contentHeight()/2
	if cursor := m.editorCursor(); cursor == nil || cursor.Position.Y != middle {
		t.Fatalf("mouse jump did not center cursor on viewport row %d: %v", middle, cursor)
	}
	m.Update(tea.MouseClickMsg{X: x, Y: l.start - 1, Button: tea.MouseLeft})
	if _, outline := m.outlineLayout(); outline.height != 0 {
		t.Fatal("outline did not collapse")
	}
}

func TestExplorerOutlineOnlyShowsForMarkdown(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	if header, _ := m.outlineLayout(); header != m.bodyHeight() {
		t.Fatal("outline is visible without a file")
	}
	for _, path := range []string{"README.MARKDOWN", "file0.go"} {
		if path != "file0.go" {
			if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("# Heading\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if !m.openDocument(path, false) {
			t.Fatalf("could not open %s", path)
		}
		header, _ := m.outlineLayout()
		if visible := header < m.bodyHeight(); visible != (path == "README.MARKDOWN") {
			t.Fatalf("outline visible = %v for %s", visible, path)
		}
	}
}
