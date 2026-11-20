package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/fileicon"
	"github.com/foursmith/vl/internal/theme"
)

func TestSidebarDividerMatchesTabBarAndEditorBackgrounds(t *testing.T) {
	m := newHoverRenderModel(t)
	const y = 4
	assertCell := func(glyph, foreground string) {
		t.Helper()
		screen := uv.NewScreenBuffer(m.width, m.height)
		uv.NewStyledString(m.View().Content).Draw(screen, uv.Rect(0, 0, m.width, m.height))
		for _, expected := range []struct {
			y          int
			background string
		}{{tabRow, m.colors.Panel}, {y, m.colors.Background}} {
			cell := screen.CellAt(m.dividerX(), expected.y)
			if cell.Content != glyph ||
				(foreground != "" && !sameColor(cell.Style.Fg, lipgloss.Color(foreground))) ||
				!sameColor(cell.Style.Bg, lipgloss.Color(expected.background)) {
				t.Fatalf("unexpected divider cell on row %d: %#v", expected.y, cell)
			}
		}
	}

	for _, right := range []bool{false, true} {
		m.SetColorTheme(theme.Builtin("dark_modern"))
		m.SetSidebarLayout(38, right)
		assertCell(" ", "")
		m.Update(tea.MouseMotionMsg{X: m.dividerX(), Y: y})
		assertCell("│", m.colors.Accent)
		m.Update(tea.MouseClickMsg{X: m.dividerX(), Y: y, Button: tea.MouseLeft})
		m.Update(tea.MouseMotionMsg{X: m.dividerX(), Y: -1, Button: tea.MouseLeft})
		assertCell("│", m.colors.Accent)
		m.Update(tea.MouseReleaseMsg{X: m.dividerX(), Y: -1, Button: tea.MouseLeft})
		m.Update(tea.MouseMotionMsg{X: m.mainX() + 8, Y: y})
		assertCell(" ", "")

		m.SetColorTheme(decorationTheme(t))
		assertCell(" ", "")
	}
}

func TestTabBarAndInactiveTabsUseSidebarBackground(t *testing.T) {
	m := tabWorkspace(t, 2)
	t.Cleanup(m.Close)
	th, _ := fixtureTheme(t, "vs-dark", `{"colors":{"sideBar.background":"#332611","editorGroupHeader.tabsBackground":"#141611","tab.inactiveBackground":"#202511","tab.activeBackground":"#231b10"}}`)
	m.SetColorTheme(th)
	m.SetSidebarLayout(38, true)
	m.openFile("file0.go")
	m.openFile("file1.go")

	screen := uv.NewScreenBuffer(m.width, m.height)
	uv.NewStyledString(m.View().Content).Draw(screen, uv.Rect(0, 0, m.width, m.height))
	var activeStart, activeEnd int
	for _, slot := range m.tabLayout() {
		if slot.control == 0 && slot.index == m.activeTab {
			activeStart, activeEnd = slot.x, slot.x+slot.width
		}
	}
	if activeEnd == 0 {
		t.Fatal("active tab is missing")
	}
	for x := 0; x < m.width; x++ {
		want := m.colors.Panel
		if x >= activeStart && x < activeEnd {
			want = m.colors.TabActiveBackground
		}
		if bg := screen.CellAt(x, tabRow).Style.Bg; !sameColor(bg, lipgloss.Color(want)) {
			t.Fatalf("header column %d has background %v, want %s", x, bg, want)
		}
	}
}

func TestSelectedFolderKeepsChevronWithoutAnExtraArrow(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.sidebar = true
	m.items = []item{{folder: "src", label: "src"}}
	m.expanded["src"] = true
	m.selected = 0

	m.SetColorTheme(theme.Builtin("dark_modern"))
	selected := func() string { return plain(m.sidebarLinesWidth(34)[headerHeight]) }
	if got := selected(); !strings.HasPrefix(got, m.treeChevron(true)) {
		t.Fatalf("ordinary selection should use the existing highlight: %q", got)
	}

	m.SetColorTheme(theme.Builtin("hc_black"))
	if got := selected(); !strings.HasPrefix(got, m.treeChevron(true)) {
		t.Fatalf("high-contrast selection should also keep just the chevron: %q", got)
	}
	cells := uv.NewStyledString(m.sidebarLinesWidth(34)[headerHeight]).Lines(ansi.GraphemeWidth)[0]
	if sameColor(cells[0].Style.Bg, lipgloss.Color(m.colors.Panel)) {
		t.Fatal("high-contrast selection is indistinguishable from the sidebar")
	}
}

func decorationTheme(t *testing.T) *theme.Theme {
	t.Helper()
	raw := `{"colors":{"editor.background":"#000","sideBar.background":"#000","sideBar.foreground":"#bfbaaa","list.activeSelectionBackground":"#181818","list.activeSelectionForeground":"#dbd7ca","list.inactiveSelectionBackground":"#121212","list.inactiveSelectionForeground":"#bbb8aa","gitDecoration.addedResourceForeground":"#4d9375","gitDecoration.modifiedResourceForeground":"#6394bf","gitDecoration.deletedResourceForeground":"#cb7676","gitDecoration.untrackedResourceForeground":"#5eaab5","gitDecoration.renamedResourceForeground":"#73c991","gitDecoration.conflictingResourceForeground":"#d4976c","gitDecoration.stageModifiedResourceForeground":"#445566","gitDecoration.stageDeletedResourceForeground":"#778899","gitDecoration.submoduleResourceForeground":"#999aaa","gitDecoration.ignoredResourceForeground":"#dedcd550"}}`
	th, _ := fixtureTheme(t, "vs-dark", raw)
	return th
}
func treeCells(m *Model, e item, state treeRowState) []uv.Cell {
	bg := m.colors.Panel
	if state.selected {
		bg = m.colors.Selection
		if !m.sidebar {
			bg = m.colors.InactiveSelection
		}
	}
	text, fg := m.treeRow(e, 60, bg, state)
	return uv.NewStyledString(row(text, 60, fg, bg)).Lines(ansi.GraphemeWidth)[0]
}

func TestExplorerAndChangesUseDifferentDecorationRoles(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.SetColorTheme(decorationTheme(t))
	e := item{label: "zfile.go", file: "zfile.go", status: 'M'}
	for _, section := range []section{explorer, changes} {
		m.section = section
		cells := treeCells(m, e, treeRowState{})
		want := m.colors.GitModified
		if section == changes {
			want = m.colors.SideForeground
		}
		labelFound, markerFound := false, false
		for _, cell := range cells {
			if cell.Content == "z" {
				labelFound = true
				if !sameColor(cell.Style.Fg, lipgloss.Color(want)) {
					t.Fatalf("section %v: filename has wrong decoration", section)
				}
			}
			if cell.Content == "M" {
				markerFound = true
				wantMarker := "#6394BF"
				if section == explorer {
					wantMarker = "#4A6F8F"
				} // iconlabel.css opacity: .75
				if !sameColor(cell.Style.Fg, lipgloss.Color(theme.Readable(wantMarker, m.colors.Panel))) {
					t.Fatal("status marker lost Git decoration")
				}
			}
		}
		if !labelFound || !markerFound {
			t.Fatal("missing filename or marker")
		}
	}
}

func TestTreeSelectionAndIgnoredIcons(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.SetColorTheme(decorationTheme(t))
	e := item{label: "zfile.go", file: "zfile.go", status: 'M'}
	for _, section := range []section{explorer, changes} {
		for _, focus := range []bool{true, false} {
			m.section, m.sidebar = section, focus
			cells := treeCells(m, e, treeRowState{selected: true})
			fg, bg := m.colors.SelectionForeground, m.colors.Selection
			if !focus {
				fg, bg = m.colors.InactiveSelectionForeground, m.colors.InactiveSelection
				if section == explorer {
					fg = m.colors.GitModified
				}
			}
			for _, cell := range cells {
				if cell.Content == "z" && (!sameColor(cell.Style.Fg, lipgloss.Color(fg)) || !sameColor(cell.Style.Bg, lipgloss.Color(bg))) {
					t.Fatal("selected filename used a Git or border color instead of list selection")
				}
				if cell.Content == "M" {
					want := "#6394BF"
					if section == explorer {
						want = "#4F7394"
						if focus {
							want = "#AAA79D"
						}
					}
					if !sameColor(cell.Style.Fg, lipgloss.Color(theme.Readable(want, bg))) {
						t.Fatalf("selection section=%v focus=%v got=%v want=%s", section, focus, cell.Style.Fg, want)
					}
				}
			}
		}
	}
	m.section = explorer
	e.ignored = true
	icon := fileicon.For(e.file, m.nerd)
	for _, cell := range treeCells(m, e, treeRowState{}) {
		if cell.Content == "z" && !sameColor(cell.Style.Fg, lipgloss.Color(theme.Readable("#464543", m.colors.Panel))) {
			t.Fatal("ignored filename is not muted")
		}
		if cell.Content == icon.Glyph && !sameColor(cell.Style.Fg, lipgloss.Color(icon.Color)) {
			t.Fatal("Git decorations recolored the file icon")
		}
	}
}

func TestTreeStatusColorsAreIndependent(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.SetColorTheme(decorationTheme(t))
	for _, tc := range []struct {
		status byte
		staged bool
		want   string
	}{
		{'U', false, m.colors.GitUntracked}, {'A', false, m.colors.GitAdded}, {'D', false, m.colors.GitDeleted}, {'!', false, m.colors.GitConflicting}, {'R', false, m.colors.GitRenamed}, {'M', true, m.colors.GitStageModified}, {'D', true, m.colors.GitStageDeleted},
	} {
		if got := m.gitDecorationColor(item{status: tc.status, staged: tc.staged}); got != tc.want {
			t.Errorf("status %c staged %v: %s want %s", tc.status, tc.staged, got, tc.want)
		}
	}
}
