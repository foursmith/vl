package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/files"
)

func TestEditorWheelStopsAtFileBoundariesWithoutMovingCursor(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	d := m.document()
	for _, lines := range []int{1, m.contentHeight(), 50} {
		d.Edit(strings.Repeat("line\n", lines-1) + "last")
		d.SetPosition(0, 0)
		m.ensureEditor()
		for i := 0; i < 100; i++ {
			m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelDown})
		}
		want := max(0, lines-m.contentHeight())
		if m.tabs[m.activeTab].top != want {
			t.Fatalf("%d lines: scroll top %d, want %d", lines, m.tabs[m.activeTab].top, want)
		}
		if d.Cursor != 0 {
			t.Fatal("wheel scrolling moved the editing cursor")
		}
		if !strings.Contains(plain(strings.Join(m.editorLines(), "\n")), "last") {
			t.Fatal("last line scrolled out of the viewport")
		}
		for i := 0; i < 100; i++ {
			m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelUp})
		}
		if m.tabs[m.activeTab].top != 0 {
			t.Fatal("scrolled past the beginning")
		}
	}
}

func TestEditorScrollClampsAfterResizeAndTextDeletion(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	d := m.document()
	d.Edit(strings.Repeat("line\n", 49) + "last")
	d.SetPosition(49, 0)
	m.ensureEditor()
	if m.tabs[m.activeTab].top == 0 {
		t.Fatal("cursor did not scroll into view")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 80})
	if m.tabs[m.activeTab].top != 0 {
		t.Fatal("larger viewport kept unnecessary bottom space")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	d.SelectAll()
	d.Insert("short")
	m.ensureEditor()
	if m.tabs[m.activeTab].top != 0 {
		t.Fatal("deleting text left scroll beyond file")
	}
}

func TestSidebarWheelScrollsWithoutSelectionOrPreview(t *testing.T) {
	for _, section := range []section{explorer, changes, history} {
		m := tabWorkspace(t, 0)
		m.section = section
		for i := 0; i < 200; i++ {
			m.items = append(m.items, item{label: "file.go", key: fmt.Sprint(i)})
		}
		m.selected = 3
		for i := 0; i < 100; i++ {
			_, cmd := m.Update(tea.MouseWheelMsg{X: 5, Y: 10, Button: tea.MouseWheelDown})
			if cmd != nil {
				t.Fatal("scroll scheduled a preview or Git request")
			}
		}
		want := len(m.items) - (m.bodyHeight() - headerHeight)
		if m.sideTop != want || m.selected != 3 || m.diffSeq != 0 {
			t.Fatalf("section %d changed selection/diff or exceeded bottom: top=%d", section, m.sideTop)
		}
		for i := 0; i < 100; i++ {
			m.Update(tea.MouseWheelMsg{X: 5, Y: 10, Button: tea.MouseWheelUp})
		}
		if m.sideTop != 0 {
			t.Fatal("sidebar scrolled past beginning")
		}
	}
}

func TestExplorerRefreshPreservesScrolledViewport(t *testing.T) {
	m := tabWorkspace(t, 0)
	for i := 0; i < 200; i++ {
		path := fmt.Sprintf("file%03d.go", i)
		m.treeEntries = append(m.treeEntries, files.Entry{Path: path, Label: path})
	}
	m.rebuildItems()
	m.selected = 0
	m.sideTop = 70
	m.rebuildItems()
	if m.sideTop != 70 || m.selected != 0 {
		t.Fatal("background rebuild snapped scrolled viewport back to selection")
	}
}

func TestEditorRowCacheIsBoundedAndInvalidatesOnTextAndLayoutChanges(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	d := m.document()
	d.Edit(strings.Repeat("line\n", 1000))
	m.ensureEditor()
	for i := 0; i < 200; i++ {
		m.tabs[0].top = i
		m.editorLines()
		if len(m.tabs[0].renderedRows) > m.contentHeight() {
			t.Fatal("render cache grows with scroll history")
		}
	}
	d.SelectAll()
	d.Insert("changed")
	m.ensureEditor()
	if !strings.Contains(plain(m.editorLines()[0]), "changed") {
		t.Fatal("text edit reused stale rows")
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	for _, line := range m.editorLines() {
		if ansi.StringWidth(line) != m.mainWidth() {
			t.Fatal("resize reused rows with stale widths")
		}
	}
}

func TestEditorRowCacheMatchesFreshRenderAfterCursorAndStickyChanges(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	d := m.document()
	d.Edit("package sample\nfunc outer() {\n\tif ready {\n" +
		strings.Repeat("\t\twork()\n", 10) + "\t}\n}\n")
	drain(t, m, m.scheduleHighlight())
	tab := &m.tabs[m.activeTab]
	m.editorLines()
	for _, step := range []struct{ line, top int }{{4, 3}, {5, 4}, {12, 9}, {14, 10}, {0, 0}} {
		d.SetPosition(step.line, 0)
		tab.top = step.top
		rows := m.editorLines()
		tab.renderedRows = nil
		if want := m.editorLines(); !slices.Equal(rows, want) {
			t.Fatalf("cached rows differ from a fresh render at line %d, top %d", step.line, step.top)
		}
	}
}
