package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	searchservice "github.com/foursmith/vl/internal/search"
)

func TestWorkspaceSearchSecondTabAndNavigation(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	original := m.document()
	nav := m.navigation()
	if len(nav) != 5 || nav[1].section != search || nav[2].section != changes || nav[3].section != history || nav[4].section != pullRequests {
		t.Fatal("unexpected sidebar order", nav)
	}
	press(m, "ctrl+2")
	if m.section != search || !m.sidebar || m.document() != original {
		t.Fatal("search must preserve current file")
	}
	// Search text is input, including Vim commands and spaces.
	m.Update(tea.PasteMsg{Content: "package main"})
	drain(t, m, m.runSearch())
	if m.search.result.Count != 1 || len(m.searchRows()) != 2 {
		t.Fatalf("unexpected result: %+v", m.search.result)
	}
	press(m, "ctrl+j")
	if m.search.selected != 1 || !m.search.query.Focused() || m.search.focus != 0 || m.search.query.Value() != "package main" {
		t.Fatal("Ctrl+J did not select the next result while keeping query focus")
	}
	press(m, "ctrl+k")
	if m.search.selected != 0 || !m.search.query.Focused() {
		t.Fatal("Ctrl+K did not select the previous result while keeping query focus")
	}
	press(m, "esc")
	press(m, "j")
	press(m, "enter")
	if m.sidebar || m.document() != original || m.document().SelectedText() != "package main" || !m.tabs[m.activeTab].preview && len(m.tabs) != 1 {
		t.Fatal("result did not open in the existing editor")
	}
	if m.tabs[m.activeTab].vim.InsertMode || original.Dirty() {
		t.Fatal("search must not edit the file")
	}
	press(m, "ctrl+3")
	if m.section != changes {
		t.Fatal("Ctrl+3 must open Changes")
	}
	press(m, "ctrl+4")
	if m.section != history {
		t.Fatal("Ctrl+4 must open History")
	}
	press(m, "ctrl+2")
	if m.search.query.Value() != "package main" || m.search.result.Count != 1 {
		t.Fatal("search state lost on tab switch")
	}
}

func TestWorkspaceSearchCancelsStaleRepliesAndIncludesBuffers(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.document().Insert("unsavedNeedle ")
	m.openSearch()
	m.search.query.SetValue("unsavedNeedle")
	old := m.runSearch()
	m.search.query.SetValue("package")
	fresh := m.runSearch()
	m.Update(old())
	if !m.search.loading || m.search.result.Count != 0 {
		t.Fatal("stale result applied")
	}
	drain(t, m, fresh)
	if m.search.result.Count != 1 {
		t.Fatal("latest search missing")
	}
	m.search.query.SetValue("unsavedNeedle")
	drain(t, m, m.runSearch())
	if m.search.result.Count != 1 {
		t.Fatal("unsaved buffer was not searched")
	}
	m.search.options.Regex = true
	m.search.query.SetValue("[")
	m.runSearch()
	if m.search.err == "" || m.search.loading {
		t.Fatal("invalid regex was not reported")
	}
	m.search.query.SetValue("")
	m.runSearch()
	if m.search.err != "" || m.search.result.Count != 0 {
		t.Fatal("clearing query did not reset search")
	}
}

func TestWorkspaceSearchLayoutScrollAndMouse(t *testing.T) {
	m := tabWorkspace(t, 0)
	path := filepath.Join(m.repo.Root, "示例.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("\t前缀 needle é🙂\n", 60)), 0644); err != nil {
		t.Fatal(err)
	}
	m.openSearch()
	m.search.query.SetValue("needle")
	drain(t, m, m.runSearch())
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(36, right)
		for _, width := range []int{30, 69, 70, 120} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
			m.sidebar = true
			m.search.focus = 0
			m.search.query.Focus()
			w := m.sidebarContentWidth()
			view := m.View()
			if len(strings.Split(view.Content, "\n")) != m.height {
				t.Fatal("wrong frame height")
			}
			if !m.search.query.Focused() {
				t.Fatal("query lost input focus")
			}
			lines := m.sidebarLinesWidth(w)
			for _, line := range lines {
				if ansi.StringWidth(line) != w {
					t.Fatalf("width %d: %q", w, plain(line))
				}
			}
			if !strings.Contains(plain(lines[m.searchResultY()]), "示例.go") || !strings.Contains(plain(lines[m.searchResultY()+1]), "1") || !strings.Contains(plain(lines[m.searchResultY()+1]), "needle") {
				t.Fatal("missing file/line/snippet", plain(lines[m.searchResultY()+1]))
			}
			m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: m.searchResultY() + 2, Button: tea.MouseWheelDown})
			if m.search.top != 3 {
				t.Fatal("results did not scroll")
			}
			m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: 2, Button: tea.MouseWheelDown})
			if m.search.top != 3 {
				t.Fatal("scrolling query scrolled results")
			}
			m.search.top = 0
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m.searchControlAction("case") // Match case
	if !m.search.options.Case {
		t.Fatal("option click did not toggle")
	}
	drain(t, m, m.runSearch())
	m.clickSearch(5, m.searchResultY()+1)
	if !m.sidebar || m.document() == nil || m.document().SelectedText() != "needle" {
		t.Fatal("match click did not open file")
	}
}

func TestWorkspaceSearchResultHoverWithNoTreeItems(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openSearch()
	m.search.query.SetValue("package")
	drain(t, m, m.runSearch())
	if len(m.items) != 0 || len(m.searchRows()) == 0 {
		t.Fatal("expected search results without tree items")
	}

	y := m.searchResultY()
	m.Update(tea.MouseMotionMsg{X: m.sideX() + 2, Y: y})
	assertFreshHoverFrame(t, m)
	if hint := m.hoverHint(); hint != "" {
		t.Fatalf("unexpected result hover hint: %q", hint)
	}

	m.Update(tea.MouseMotionMsg{X: m.sideX() + m.sidebarContentWidth() - 1, Y: y})
	assertFreshHoverFrame(t, m)
	if hint := m.hoverHint(); hint != searchText("Remove from Search Results (Delete)") {
		t.Fatalf("missing remove result hover hint: %q", hint)
	}
}

func TestWorkspaceSearchFoldsAndStaleFile(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openSearch()
	m.search.query.SetValue("package")
	drain(t, m, m.runSearch())
	m.search.focus = 2
	press(m, "h")
	if len(m.searchRows()) != 2 {
		t.Fatal("parent navigation folded file")
	}
	m.clickSearch(1, m.searchResultY())
	if len(m.searchRows()) != 1 {
		t.Fatal("file not folded")
	}
	press(m, "l")
	if len(m.searchRows()) != 2 {
		t.Fatal("file not expanded")
	}
	path := m.search.result.Files[0].Path
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	m.search.selected = 1
	cmd := m.openSelected()
	if cmd == nil || !m.search.loading || !strings.Contains(m.notice, "changed") {
		t.Fatal("stale file not refreshed")
	}
	drain(t, m, cmd)
	if m.search.result.Count != 0 {
		t.Fatal("stale result remains")
	}
	// Old replies cannot repopulate a cleared query.
	m.search.query.SetValue("")
	m.runSearch()
	m.Update(searchMsg{seq: m.search.seq - 1, result: searchservice.Result{Count: 42}})
	if m.search.result.Count != 0 {
		t.Fatal("stale results restored after clear")
	}
}

func TestSearchResultCentersMatchAndClampsAtFileEdges(t *testing.T) {
	for _, target := range []int{0, 60, 119} {
		m := tabWorkspace(t, 1)
		t.Cleanup(m.Close)
		lines := make([]string, 120)
		for i := range lines {
			lines[i] = "source"
		}
		lines[target] = "needle"
		if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
		m.openSearch()
		m.search.query.SetValue("needle")
		drain(t, m, m.runSearch())
		m.clickSearch(5, m.searchResultY()+1)
		tab := &m.tabs[m.activeTab]
		want := max(0, target-m.contentHeight()/2)
		if tab.top != want || !m.sidebar || m.document().SelectedText() != "needle" {
			t.Fatalf("target=%d top=%d want=%d", target, tab.top, want)
		}
		row, _ := m.document().Position()
		if row != target {
			t.Fatal("cursor left matched line")
		}
		// Reopening an already visible result still centers it.
		tab.top = min(119, max(0, want+3))
		m.sidebar = true
		m.clickSearch(5, m.searchResultY()+1)
		if tab.top != want {
			t.Fatal("existing tab was not recentered")
		}
	}
}
