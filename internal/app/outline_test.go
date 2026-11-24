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
	press(m, "o")
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

func TestExplorerHeaderChevronsAlign(t *testing.T) {
	m := sidebarListWorkspace(t, explorer)
	m.height = 20
	if err := os.WriteFile(filepath.Join(m.repo.Root, "guide.md"), []byte("# Guide\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !m.openDocument("guide.md", false) {
		t.Fatal("could not open Markdown file")
	}
	root := m.repositoryHeader("", explorerRootKey)
	m.items = []item{root}
	m.sidebarCollapsed[root.key] = true
	m.explorerPanel = explorerPanelNone
	m.fileHistory = &fileHistoryState{path: "guide.md"}

	for _, monochrome := range []bool{false, true} {
		m.monochrome = monochrome
		lines := m.sidebarLinesWidth(40)
		outline, _ := m.outlineLayout()
		timeline, _ := m.fileHistoryLayout()
		glyph := m.treeChevron(false)
		want := strings.Index(plain(lines[headerHeight]), glyph)
		if want < 0 {
			t.Fatal("missing repository chevron")
		}
		for _, y := range []int{outline, timeline} {
			if got := strings.Index(plain(lines[y]), glyph); got != want {
				t.Fatalf("monochrome=%v: chevron column %d, want %d in %q", monochrome, got, want, plain(lines[y]))
			}
		}
	}
}

func TestKeyboardPreviewKeepsSelectedExplorerFileVisible(t *testing.T) {
	m := sidebarListWorkspace(t, explorer)
	root, err := filepath.EvalSymlinks(m.repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	m.repo.Root = root
	m.SetTerminalSize(100, 24)
	for name, source := range map[string]string{
		"short.txt": "Short\n",
		"long.md":   strings.Repeat("# Heading\n\n", 12),
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m.snapshot.NoRepository = true
	if !m.openDocument("short.txt", true) {
		t.Fatal("could not preview the short file")
	}
	drain(t, m, m.syncFileHistory())
	before := m.treeList().height
	last := before - 1
	m.items[last-1] = item{key: "short.txt", file: "short.txt"}
	m.items[last] = item{key: "long.md", file: "long.md"}
	m.selected = last - 1
	drain(t, m, press(m, "j"))
	if got := filepath.Base(m.document().Path); got != "long.md" || m.treeList().height >= before {
		t.Fatalf("preview did not open Outline: file=%s height=%d", got, m.treeList().height)
	}
	if y := m.sidebarSelectionY(); m.sidebarRowAt(y) != m.selected {
		t.Fatalf("expanded Outline covered the keyboard selection at row %d", y)
	}
}
