package app

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

func decorationTheme(t *testing.T) *theme.Theme {
	t.Helper()
	name := filepath.Join(t.TempDir(), "tree.json")
	raw := `{"colors":{"editor.background":"#000","sideBar.background":"#000","sideBar.foreground":"#bfbaaa","list.activeSelectionBackground":"#181818","list.activeSelectionForeground":"#dbd7ca","list.inactiveSelectionBackground":"#121212","list.inactiveSelectionForeground":"#bbb8aa","gitDecoration.addedResourceForeground":"#4d9375","gitDecoration.modifiedResourceForeground":"#6394bf","gitDecoration.deletedResourceForeground":"#cb7676","gitDecoration.untrackedResourceForeground":"#5eaab5","gitDecoration.renamedResourceForeground":"#73c991","gitDecoration.conflictingResourceForeground":"#d4976c","gitDecoration.stageModifiedResourceForeground":"#445566","gitDecoration.stageDeletedResourceForeground":"#778899","gitDecoration.submoduleResourceForeground":"#999aaa","gitDecoration.ignoredResourceForeground":"#dedcd550"}}`
	if err := os.WriteFile(name, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(name, "")
	if err != nil {
		t.Fatal(err)
	}
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
				if !sameColor(cell.Style.Fg, lipgloss.Color(wantMarker)) {
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
					if !sameColor(cell.Style.Fg, lipgloss.Color(want)) {
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
		if cell.Content == "z" && !sameColor(cell.Style.Fg, lipgloss.Color("#464543")) {
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
