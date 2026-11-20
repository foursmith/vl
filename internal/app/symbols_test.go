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
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/theme"
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
				m.hoverY = headerHeight + i - m.sideTop
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
	imported, _ := fixtureTheme(t, "hc-black", `{"colors":{"sideBar.background":"#121212","activityBar.foreground":"#EAEAEA","activityBar.inactiveForeground":"#8A8A8D"}}`)
	if colors := imported.Palette(); colors.NavigationForeground != "#EAEAEA" || colors.NavigationInactiveForeground != "#8A8A8D" {
		t.Fatal("imported activity bar icon colors were ignored")
	}
	for _, selected := range []*theme.Theme{
		theme.Builtin("dark_modern"), theme.Builtin("light_modern"), theme.Builtin("light_plus"),
		theme.Builtin("hc_black"), theme.Builtin("hc_light"), imported,
	} {
		colors := selected.Palette()
		m.colors = colors
		activeColor := theme.Readable(colors.NavigationForeground, colors.Panel)
		inactiveColor := theme.Readable(colors.NavigationInactiveForeground, colors.Panel)
		if activeColor == inactiveColor {
			t.Fatalf("theme=%s: active and inactive icons have the same foreground", selected.ID)
		}
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
						want := inactiveColor
						if item.section == active {
							want = activeColor
						}
						cells := uv.NewStyledString(ansi.Cut(nav, item.x, item.x+item.width)).Lines(ansi.GraphemeWidth)[0]
						for _, cell := range cells {
							if !sameColor(cell.Style.Fg, lipgloss.Color(want)) ||
								!sameColor(cell.Style.Bg, lipgloss.Color(colors.Panel)) || cell.Style.Underline != uv.UnderlineNone {
								t.Fatalf("theme=%s icons=%v dirty=%v active=%d tab=%d: wrong navigation style", selected.ID, nerd, dirty, active, item.section)
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
