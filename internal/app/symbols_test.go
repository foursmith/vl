package app

import (
	"fmt"
	"testing"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/theme"
)

func TestControlSymbolsHaveNarrowGlyphsAndASCIIFallbacks(t *testing.T) {
	for i, s := range uiSymbols {
		if utf8.RuneCountInString(s.nerd) != 1 || ansi.StringWidth(s.nerd) != 1 {
			t.Fatalf("control %d must occupy one Nerd Font cell", i)
		}
		if s.plain == "" {
			t.Fatalf("control %d has no fallback", i)
		}
		for _, r := range s.plain {
			if r < 32 || r > 126 {
				t.Fatalf("control %d fallback is not printable ASCII", i)
			}
		}
	}
}

func TestSymbolModesPreserveLayoutAcrossWidthsAndHoverTargets(t *testing.T) {
	m := tabWorkspace(t, 3)
	for i := 0; i < 3; i++ {
		m.openFile(fmt.Sprintf("file%d.go", i))
	}
	m.document().Insert("// dirty\n")
	m.section = changes
	m.snapshot.Changes = []gitservice.Change{{Path: "src/app/main.go", Index: 'M', Worktree: 'M'}}
	m.snapshot.Refs = []gitservice.Ref{{Name: "main", FullName: "refs/heads/main", Kind: gitservice.LocalBranch, Current: true}, {Name: "origin/main", FullName: "refs/remotes/origin/main", Kind: gitservice.RemoteBranch}, {Name: "v1", FullName: "refs/tags/v1", Kind: gitservice.Tag}}
	m.rebuildItems()
	for _, nerd := range []bool{false, true} {
		m.nerd = nerd
		for _, width := range []int{30, 48, 70, 80, 120, 200} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			m.sidebar = true
			check := func(text string, w int) {
				t.Helper()
				if ansi.StringWidth(text) != w {
					t.Fatalf("icons=%v width=%d: rendered %d cells, want %d", nerd, width, ansi.StringWidth(text), w)
				}
				if !nerd {
					for _, r := range plain(text) {
						if unicode.Is(unicode.Co, r) {
							t.Fatalf("plain mode leaked private font glyph %U", r)
						}
					}
				}
			}
			check(m.tabBar(), m.mainWidth())
			check(m.breadcrumb("src/app/main.go", m.mainWidth()), m.mainWidth())
			check(m.statusLine(), width)
			w := m.sideWidth()
			if !m.sideShown() {
				w = width
			}
			for i := range m.items {
				m.hoverX = 7
				m.hoverY = m.sidebarHeaderRows() + i - m.sideTop
				for _, line := range m.sidebarLinesWidth(w) {
					check(line, w)
				}
			}
			// Render the picker too: branch/cloud/tag/current markers share the mode.
			m.openBranchPicker("", "")
			for _, a := range m.palette {
				if !nerd {
					for _, r := range a.label {
						if unicode.Is(unicode.Co, r) {
							t.Fatal("picker leaked private glyph")
						}
					}
				}
			}
			m.modal = ""
			m.input.Blur()
		}
	}
}

func TestSidebarNavigationUsesActiveColorWithoutChangeDecoration(t *testing.T) {
	m := tabWorkspace(t, 0)
	for _, colors := range []theme.Palette{theme.Dark(), theme.Light()} {
		m.colors = colors
		for _, nerd := range []bool{false, true} {
			m.nerd = nerd
			for _, dirty := range []bool{false, true} {
				m.snapshot.Changes = nil
				if dirty {
					m.snapshot.Changes = []gitservice.Change{{Path: "main.go", Worktree: 'M'}}
				}
				for _, active := range []section{explorer, changes, history} {
					m.section = active
					nav := m.sidebarLinesWidth(40)[0]
					for _, item := range m.navigation() {
						want := colors.NavigationInactiveForeground
						if item.section == active {
							want = colors.NavigationForeground
						}
						cells := uv.NewStyledString(ansi.Cut(nav, item.x, item.x+item.width)).Lines(ansi.GraphemeWidth)[0]
						for _, cell := range cells {
							if !sameColor(cell.Style.Fg, lipgloss.Color(theme.Composite(want, colors.Panel))) || cell.Style.Underline != 0 {
								t.Fatalf("theme=%s icons=%v dirty=%v active=%d tab=%d: wrong navigation style", colors.Panel, nerd, dirty, active, item.section)
							}
						}
					}
				}
			}
		}
	}
	m.section = changes
	for _, a := range m.actions() {
		if a.id == "view.changes" && a.label != "View: Changes" {
			t.Fatal("Changes command not renamed")
		}
	}
}
