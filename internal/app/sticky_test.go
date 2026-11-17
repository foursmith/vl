package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestStickyCodeScopesIgnoreCommentsAndStrings(t *testing.T) {
	text := "package p\nfunc outer() {\n // { fake\n s := `}\n{`\n if ready {\n  work()\n  more()\n }\n}\nfunc next() {\n done()\n}\n"
	scopes := codeScopes("test.go", text)
	for _, tc := range []struct {
		top  int
		want []int
	}{
		{0, nil}, {1, nil}, {6, []int{1, 5}}, {8, []int{1}}, {9, nil}, {11, []int{10}},
	} {
		if got := stickyRows(scopes, tc.top, 5); !slices.Equal(got, tc.want) {
			t.Fatalf("top %d: got %v want %v; scopes %v", tc.top, got, tc.want, scopes)
		}
	}
	if got := codeScopes("notes.txt", text); len(got) != 0 {
		t.Fatal("plain text gained code context")
	}
}

func TestStickyPythonScopeEndsAtDedent(t *testing.T) {
	scopes := codeScopes("test.py", "class C:\n def method(self):\n  if ready:\n   work()\n   more()\n\ndef next():\n pass\n")
	if got := stickyRows(scopes, 3, 5); !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatal(got)
	}
	if got := stickyRows(scopes, 6, 5); len(got) != 0 {
		t.Fatal("previous scope survived dedent", got)
	}
}

func TestStickyEditorPaintingHitTestingAndInvalidation(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	d := m.document()
	d.Edit("func outer() {\n if ready {\n" + strings.Repeat("  work()\n", 30) + " }\n}\n")
	d.SetPosition(15, 0)
	m.tabs[0].top = 10
	rows := m.editorLines()
	if !strings.Contains(ansi.Strip(rows[0]), "func outer") || !strings.Contains(ansi.Strip(rows[1]), "if ready") {
		t.Fatal("missing nested context", rows[:2])
	}
	for _, painted := range [][]string{rows, m.editorLines()} {
		for y, row := range painted {
			for _, cell := range uv.NewStyledString(row).Lines(ansi.GraphemeWidth)[0] {
				if (cell.Style.Underline != 0) != (y == 1) {
					t.Fatal("sticky separator missing or leaked into another row", y)
				}
			}
		}
	}
	if m.editorRowAt(0) != 0 || m.editorRowAt(1) != 1 || m.editorRowAt(2) != 12 {
		t.Fatal("paint and hit test disagree")
	}
	m.positionEditor(m.mainX()+9, editorContentRow+2, false)
	if line, _ := d.Position(); line != 12 {
		t.Fatal("click selected wrong source line", line)
	}
	m.positionEditor(m.mainX()+7, editorContentRow, false)
	if line, _ := d.Position(); line != 0 || m.tabs[0].top != 0 {
		t.Fatal("sticky click did not reveal declaration")
	}
	m.tabs[0].top = 10
	d.Edit(strings.Repeat("plain\n", 40))
	if got := m.editorStickyRows(); len(got) != 0 {
		t.Fatal("stale scope after edit", got)
	}
}

func stickyTreeModel(t *testing.T) *Model {
	t.Helper()
	m := tabWorkspace(t, 0)
	m.section = explorer
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	m.items = []item{{label: "root", folder: "root", key: "root"}, {label: "nested", folder: "root/nested", key: "nested", depth: 1}}
	for i := 0; i < 30; i++ {
		m.items = append(m.items, item{label: fmt.Sprintf("file%d", i), file: fmt.Sprintf("root/nested/file%d", i), depth: 2})
	}
	m.items = append(m.items, item{label: "sibling", folder: "sibling", key: "sibling"}, item{label: "child", file: "sibling/child", depth: 1})
	return m
}

func TestStickyTreePaintingHoverClickAndSiblingBoundary(t *testing.T) {
	m := stickyTreeModel(t)
	m.sideTop = 8
	header := headerHeight
	if got := m.sidebarStickyRows(); !slices.Equal(got, []int{0, 1}) {
		t.Fatal(got)
	}
	rows := m.sidebarLines()
	if !strings.Contains(ansi.Strip(rows[header]), "root") || !strings.Contains(ansi.Strip(rows[header+1]), "nested") {
		t.Fatal("missing parent directories")
	}
	m.hoverX, m.hoverY = m.sideX()+3, header+1
	if m.hoveredSidebarRow() != 1 || m.sidebarRowAt(header+2) != 10 {
		t.Fatal("tree hit testing differs from painting")
	}
	m.expanded["root/nested"] = true
	m.clickSidebar(tea.MouseClickMsg{X: m.sideX() + 3, Y: header + 1, Button: tea.MouseLeft})
	if m.selected != 1 || m.expanded["root/nested"] {
		t.Fatal("sticky folder click hit a covered file")
	}
	m.sideTop = len(m.items) - 2
	if got := m.sidebarStickyRows(); len(got) != 0 {
		t.Fatal("old parent survived sibling boundary", got)
	}
	m.sideTop = len(m.items) - 1
	if got := m.sidebarStickyRows(); !slices.Equal(got, []int{len(m.items) - 2}) {
		t.Fatal("sibling failed to pin", got)
	}
}

func TestStickyRowsKeepKeyboardTargetsVisibleAndRespectSmallWindows(t *testing.T) {
	m := stickyTreeModel(t)
	m.sideTop = 8
	m.selected = 9
	m.ensureSidebar()
	if m.sidebarRowAt(headerHeight+m.selected-m.sideTop) != m.selected {
		t.Fatal("selection hidden behind sticky parents")
	}
	m = tabWorkspace(t, 1)
	m.openFile("file0.go")
	d := m.document()
	d.Edit("func outer() {\n if ready {\n" + strings.Repeat("  work()\n", 30) + " }\n}\n")
	m.tabs[0].top = 10
	d.SetPosition(11, 0)
	m.ensureEditor()
	line, _ := d.Position()
	if m.editorRowAt(line-m.tabs[0].top) != line {
		t.Fatal("cursor hidden behind sticky code")
	}
	for _, height := range []int{4, 5, 8, 20} {
		m.Update(tea.WindowSizeMsg{Width: 100, Height: height})
		if len(m.editorStickyRows()) >= m.contentHeight() {
			t.Fatal("sticky rows consumed viewport")
		}
		for _, row := range m.editorLines() {
			if ansi.StringWidth(row) != m.mainWidth() {
				t.Fatal("incorrect editor width")
			}
		}
	}
}

func TestStickyChangesGroupsRetainRepositoryAndStageIdentity(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = changes
	m.items = []item{
		{label: "repo", header: true, repoHeader: true},
		{label: "STAGED", header: true, depth: 1, staged: true},
		{label: "src", folder: "src", depth: 1, staged: true},
	}
	for i := 0; i < 20; i++ {
		m.items = append(m.items, item{label: "staged.go", depth: 2, staged: true})
	}
	boundary := len(m.items)
	m.items = append(m.items, item{label: "CHANGES", header: true, depth: 1}, item{label: "worktree.go", depth: 1})
	m.sideTop = 5
	if got := m.sidebarStickyRows(); !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatal("missing repo/group/folder context", got)
	}
	m.sideTop = boundary
	if got := m.sidebarStickyRows(); !slices.Equal(got, []int{0, boundary}) {
		t.Fatal("staged context leaked into worktree group", got)
	}
	m.sideTop = 5
	m.openContextMenu(m.sideX()+5, headerHeight+2)
	if m.selected != 2 || !m.current().staged || m.current().folder != "src" {
		t.Fatal("context menu lost pinned folder identity")
	}
	// Rebuilding the visible tree must invalidate its cached scope ranges.
	m.items = append([]item(nil), m.items[boundary:]...)
	m.sideTop = 0
	if got := m.sidebarStickyRows(); len(got) != 0 {
		t.Fatal("stale parents after refresh", got)
	}
}
