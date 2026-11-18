package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func TestChangesNavigationShowsFileCount(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	for _, tc := range []struct {
		count int
		want  string
	}{{0, "    "}, {1, "1   "}, {99, "99  "}, {100, "99+ "}} {
		m.snapshot.Changes = make([]gitservice.Change, tc.count)
		lines := m.sidebarLines()
		if got := plain(lines[1])[8:12]; got != tc.want {
			t.Errorf("%d changes rendered %q, want %q", tc.count, got, tc.want)
		}
		if got := plain(lines[0])[8:13]; got != "G   H" {
			t.Fatalf("changes count displaced the navigation icons: %q", got)
		}
		cells := uv.NewStyledString(lines[1]).Lines(ansi.GraphemeWidth)[0]
		for x, ch := range tc.want {
			background := m.colors.Panel
			if ch != ' ' {
				background = m.colors.SidebarSelection(true)
			}
			if !sameColor(cells[8+x].Style.Bg, lipgloss.Color(background)) {
				t.Errorf("%d changes: wrong background at badge column %d", tc.count, x)
			}
		}
	}
}

func TestNavigationAndTabsShareCompactHeaderWithoutTitleBar(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	drain(t, m, m.refreshTree())
	lines := strings.Split(m.View().Content, "\n")
	first := ansi.Strip(lines[tabRow])
	if !strings.Contains(first, "E") || !strings.Contains(first, "file0.go") || strings.Contains(first, "Commands") {
		t.Fatalf("navigation and tabs are not aligned: %q", first)
	}
	sidebar := m.sidebarLines()
	for i, item := range m.navigation() {
		x := 4 * i
		if ansi.Strip(ansi.Cut(sidebar[0], x, x+4)) != item.glyph+"   " {
			t.Fatalf("navigation icon %q did not start at column %d", item.glyph, x)
		}
	}
	if strings.TrimSpace(plain(sidebar[1])) != "" {
		t.Fatal("the second navigation row should be empty")
	}
	if len(lines) != m.height || m.bodyHeight() != m.height-1 {
		t.Fatal("title bar space was not reclaimed")
	}
	second := ansi.Strip(lines[tabRow+1])
	if !strings.Contains(second, "file0.go") || !strings.Contains(plain(m.sidebarLines()[headerHeight]), m.items[0].label) {
		t.Fatalf("file tree and breadcrumbs start at the wrong row: %q", second)
	}
	if !strings.Contains(ansi.Strip(lines[editorContentRow]), "package main") {
		t.Fatal("editor content starts at the wrong row")
	}
	m.click(tea.MouseClickMsg{X: m.mainX() + 7, Y: editorContentRow, Button: tea.MouseLeft})
	line, col := m.document().Position()
	if line != 0 || col != 0 {
		t.Fatalf("editor click offset: %d:%d", line, col)
	}
	for _, slot := range m.tabLayout() {
		if slot.control == 0 && slot.index == m.activeTab {
			m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + slot.width - 3, Y: tabRow, Button: tea.MouseLeft})
			break
		}
	}
	if len(m.tabs) != 0 {
		t.Fatal("centered close button did not close tab")
	}
}

func TestPickerOutsideClickDismissesWithoutClickThrough(t *testing.T) {
	for _, point := range []struct{ x, y int }{{0, 0}, {23, 0}, {23, 20}, {119, 35}} {
		m := tabWorkspace(t, 0)
		m.openPalette()
		before := m.section
		_, cmd := m.Update(tea.MouseClickMsg{X: point.x, Y: point.y, Button: tea.MouseLeft})
		if m.modal != "" || m.input.Focused() || m.section != before || cmd != nil {
			t.Fatalf("click at %d,%d did not just dismiss picker", point.x, point.y)
		}
	}
	m := tabWorkspace(t, 0)
	executed := false
	m.openPalette()
	m.palette = []action{{label: "Run", run: func() tea.Cmd { executed = true; return nil }}}
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
	if !executed || m.modal != "" {
		t.Fatal("command click did not execute and dismiss")
	}
}

func TestAllDialogsDismissOnClickAndPreserveUnsavedFiles(t *testing.T) {
	for _, kind := range []string{"newFile", "newFolder", "branch", "commit", "find", "position", "confirm"} {
		m := tabWorkspace(t, 0)
		confirmed := false
		m.prompt(kind, "Dialog", "")
		layout := m.dialogLayout()
		view := m.View()
		top := strings.Split(view.Content, "\n")[layout.y]
		if !strings.HasPrefix(plain(ansi.Cut(top, layout.x, layout.x+layout.width)), "╭") {
			t.Fatalf("%s dialog has no rounded frame", kind)
		}
		if kind != "confirm" {
			cells := uv.NewStyledString(layout.content[1]).Lines(ansi.GraphemeWidth)[0]
			for _, cell := range cells {
				if !sameColor(cell.Style.Bg, lipgloss.Color(m.colors.Popup)) {
					t.Fatalf("%s dialog input must share the frame background", kind)
				}
			}
		}
		m.confirm = func() tea.Cmd { confirmed = true; return nil }
		_, cmd := m.Update(tea.MouseClickMsg{X: 24, Y: 5, Button: tea.MouseLeft})
		if m.modal != "" || m.input.Focused() || confirmed || cmd != nil || m.confirm != nil {
			t.Fatalf("%s dialog was not canceled on click", kind)
		}
	}
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	press(m, "x")
	doc := m.document()
	m.closeTab()
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.modal != "" || len(m.closeQueue) != 0 || m.document() != doc || !doc.Dirty() {
		t.Fatal("canceling dirty close lost unsaved document")
	}
}
