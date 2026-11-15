package app

import (
	"fmt"
	"testing"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
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
				m.hoverY = sidebarHeaderRows + i - m.sideTop
				for _, line := range m.sidebarLinesWidth(w) {
					check(line, w)
				}
			}
			// Render the picker too: branch/cloud/tag/current markers share the mode.
			m.openBranchPicker("")
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
