package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func TestHistoryScrollLoadsOlderCommitsAndRefreshResetsPages(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	run := func(args ...string) string {
		t.Helper()
		out, err := m.repo.Run(m.ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.test")
	tree := run("write-tree")
	head, oldest := "", ""
	for i := 0; i < 105; i++ {
		args := []string{"commit-tree", tree, "-m", fmt.Sprintf("commit %03d", i)}
		if head != "" {
			args = append(args, "-p", head)
		}
		head = run(args...)
		if i == 0 {
			oldest = head
		}
	}
	run("update-ref", "HEAD", head)
	drain(t, m, m.refresh())
	drain(t, m, m.selectSection(history))
	page := m.historyPages[""]
	if page == nil || len(page.commits) != 100 || page.done {
		t.Fatal("initial page not bounded")
	}
	// A new commit between page requests must not shift the offset or duplicate rows.
	newHead := run("commit-tree", tree, "-m", "new head", "-p", head)
	run("update-ref", "HEAD", newHead)
	m.selected = len(m.items) - 1
	m.ensureSidebar()
	for range 40 {
		_, cmd := m.Update(tea.MouseWheelMsg{X: m.sideX() + 3, Y: m.sidebarHeaderRows() + 1, Button: tea.MouseWheelDown})
		drain(t, m, cmd)
	}
	if !page.done || len(page.commits) != 105 || page.commits[104].ID != oldest || page.commits[0].ID != head {
		t.Fatal("scroll did not reach the oldest commit on the original history")
	}
	seen := map[string]bool{}
	for _, c := range page.commits {
		if seen[c.ID] {
			t.Fatal("duplicate commit across pages")
		}
		seen[c.ID] = true
	}
	if entry := m.current(); entry == nil || entry.commit == nil || entry.commit.ID != page.commits[100].ID {
		t.Fatal("selection did not continue into the first loaded commit")
	}
	for _, entry := range m.items {
		if entry.historyPage != nil {
			t.Fatal("load row remains after history ends")
		}
	}
	// Unchanged snapshots preserve loaded history; a changed head starts a new cursor.
	run("update-ref", "HEAD", head)
	drain(t, m, m.refresh())
	if m.historyPages[""] != page {
		t.Fatal("periodic refresh discarded loaded history")
	}
	run("update-ref", "HEAD", newHead)
	m.sideTop, m.selected = 0, 0
	drain(t, m, m.refresh())
	current := m.historyPages[""]
	if current == page || len(current.commits) != 100 || current.commits[0].ID != newHead {
		t.Fatal("changed head did not reset pagination")
	}
	current.loading = true
	m.applyHistoryPage(historyPageMsg{target: page, commits: []gitservice.Commit{{ID: oldest}}})
	if len(current.commits) != 100 || !current.loading {
		t.Fatal("stale page changed the new history")
	}
	current.loading = false
}
