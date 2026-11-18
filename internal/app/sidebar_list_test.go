package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func sidebarListWorkspace(t *testing.T, section section) *Model {
	t.Helper()
	if section == search {
		return sidebarSearchWorkspace(t, 20)
	}
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.section, m.sidebar = section, true
	for i := 0; i < 40; i++ {
		m.items = append(m.items, item{key: fmt.Sprint(i), label: "row"})
	}
	m.selected = 0
	return m
}

func TestSidebarListsShareNavigationAndScroll(t *testing.T) {
	for _, section := range []section{explorer, changes, history, search} {
		t.Run(fmt.Sprint(section), func(t *testing.T) {
			m := sidebarListWorkspace(t, section)
			m.height = 12
			l := m.sidebarList()
			*l.selected = 3
			press(m, "2")
			press(m, "j")
			if *l.selected != 5 || !m.sidebar {
				t.Fatal("counted navigation changed selection or focus incorrectly")
			}
			press(m, "g")
			press(m, "g")
			if *l.selected != 0 || *l.top != 0 {
				t.Fatal("gg did not reveal first row")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			if *l.selected != l.height {
				t.Fatal("PageDown did not use the list's viewport height")
			}
			press(m, "G")
			if *l.selected != l.count-1 || m.sidebarRowAt(m.sidebarSelectionY()) != *l.selected {
				t.Fatal("last selection was not visible or hit testing disagreed")
			}
			press(m, "2")
			press(m, "G")
			if *l.selected != 1 {
				t.Fatal("2G did not select the second row")
			}
			selected, doc := *l.selected, m.document()
			for i := 0; i < 50; i++ {
				_, cmd := m.Update(tea.MouseWheelMsg{X: m.sideX() + 5, Y: l.start + 1, Button: tea.MouseWheelDown})
				if cmd != nil {
					t.Fatal("wheel scheduled a preview")
				}
			}
			if *l.top != l.count-l.height || *l.selected != selected || m.document() != doc {
				t.Fatal("wheel exceeded bounds or changed selection/preview")
			}
			for i := 0; i < 50; i++ {
				m.Update(tea.MouseWheelMsg{X: m.sideX() + 5, Y: l.start + 1, Button: tea.MouseWheelUp})
			}
			if *l.top != 0 {
				t.Fatal("wheel scrolled past first row")
			}
		})
	}
}

func TestSidebarNavigationPrefixesBelongToSelectedTarget(t *testing.T) {
	for _, section := range []section{explorer, changes, history, search} {
		m := sidebarListWorkspace(t, section)
		l := m.sidebarList()
		press(m, "9")
		*l.selected = 2
		press(m, "j")
		if *l.selected != 3 {
			t.Fatalf("section %d retained count across target change", section)
		}
	}
	m := sidebarSearchWorkspace(t, 20)
	press(m, "9")
	m.search.seq++
	press(m, "j")
	if m.search.selected != 1 {
		t.Fatal("search count crossed a result generation")
	}
}

func TestEmptySidebarListsDoNotInventTargets(t *testing.T) {
	for _, section := range []section{explorer, changes, history, search} {
		m := tabWorkspace(t, 0)
		t.Cleanup(m.Close)
		m.section, m.sidebar = section, true
		m.items = nil
		for _, key := range []string{"j", "k", "h", "l", "g", "g", "G", "enter"} {
			drain(t, m, press(m, key))
		}
		if m.sidebarEntry() != nil || m.sidebarRowAt(m.sidebarList().start) != -1 {
			t.Fatalf("section %d invented an empty-list target", section)
		}
	}
}

func TestSidebarClickIdentityIncludesSource(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.items = []item{{key: "same", file: "file0.go"}}
	m.selected = 0
	m.clickSidebarEntry()
	if len(m.tabs) != 1 || !m.tabs[0].preview {
		t.Fatal("file click did not preview")
	}
	m.section, m.sidebar = changes, true
	m.items = []item{{key: "same", change: &gitservice.Change{Path: "file0.go"}}}
	m.clickSidebarEntry()
	if len(m.tabs) != 1 || !m.tabs[0].preview || !m.sidebar {
		t.Fatal("click in a different source was treated as a double click")
	}
}

func TestFailedSidebarOpenDoesNotKeepPreviousPreview(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.openPreviewFile("file0.go")
	doc := m.document()
	m.section, m.sidebar, m.selected = explorer, true, 0
	m.items = []item{{key: "missing", file: "missing.go"}}
	press(m, "enter")
	if m.document() != doc || !m.tabs[0].preview || !m.sidebar {
		t.Fatal("failed open promoted or focused an unrelated preview")
	}
}
