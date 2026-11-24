package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFileTimelineStaysCollapsedWhenOutlineIsUnavailable(t *testing.T) {
	m := explorerWorkspace(t)
	t.Cleanup(m.Close)

	open := func(path string) {
		t.Helper()
		drain(t, m, m.openFileInExplorer(path))
		drain(t, m, m.syncFileHistory())
	}
	open("README.md")
	if _, layout := m.outlineLayout(); layout.height == 0 {
		t.Fatal("Outline did not expand for Markdown")
	}
	open("cmd/main.go")
	if header, layout := m.fileHistoryLayout(); header >= m.bodyHeight() || layout.height != 0 {
		t.Fatal("Timeline should show only its header for a non-Markdown file")
	}

	open("README.md")
	if _, layout := m.outlineLayout(); layout.height == 0 {
		t.Fatal("Outline did not expand after returning to Markdown")
	}
	drain(t, m, press(m, "ctrl+1"))
	drain(t, m, press(m, "o"))
	if !m.outlineHasFocus() {
		t.Fatal("Outline did not receive focus")
	}
	open("cmd/main.go")
	if header, layout := m.fileHistoryLayout(); header >= m.bodyHeight() || layout.height != 0 {
		t.Fatal("Timeline expanded after switching away from a focused Outline")
	}
	drain(t, m, press(m, "ctrl+1"))
	drain(t, m, press(m, "t"))
	if _, layout := m.fileHistoryLayout(); layout.height == 0 {
		t.Fatal("Timeline did not expand after pressing t")
	}
}

func TestFileTimelineFollowsNestedFileInExplorer(t *testing.T) {
	m, child := childWorkspace(t)
	path := "child space/README.md"
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("# Nested history example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Run(m.ctx, "commit", "-am", "Edit the nested file"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	drain(t, m, m.openFileInExplorer(path))
	drain(t, m, m.syncFileHistory())
	timeline, collapsed := m.fileHistoryLayout()
	_, outline := m.outlineLayout()
	if outline.height == 0 || collapsed.height != 0 {
		t.Fatal("Outline should be the only initially open panel for Markdown")
	}
	panelHeight, treeHeight := outline.height, m.treeList().height
	_, expand := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.sideX() + 2, Y: timeline})
	drain(t, m, expand)
	_, outline = m.outlineLayout()
	if outline.height != 0 {
		t.Fatal("opening Timeline did not collapse Outline")
	}
	_, opened := m.fileHistoryLayout()
	if opened.height != panelHeight || m.treeList().height != treeHeight {
		t.Fatal("switching panels changed the Explorer viewport")
	}
	drain(t, m, press(m, "o"))
	_, outline = m.outlineLayout()
	_, collapsed = m.fileHistoryLayout()
	if outline.height == 0 || collapsed.height != 0 {
		t.Fatal("focusing Outline did not collapse Timeline")
	}
	drain(t, m, press(m, "t"))
	_, outline = m.outlineLayout()
	if outline.height != 0 {
		t.Fatal("focusing Timeline did not collapse Outline")
	}
	state := m.fileHistory
	if m.section != explorer || state == nil || len(state.revisions) < 2 || state.revisions[0].Subject != "Edit the nested file" {
		t.Fatalf("missing nested file timeline: %+v", state)
	}
	sidebar := plain(strings.Join(m.sidebarLinesWidth(37), "\n"))
	for _, label := range []string{"Outline", "Timeline", "Edit the nested file"} {
		if !strings.Contains(sidebar, label) {
			t.Fatalf("missing %q in sidebar:\n%s", label, sidebar)
		}
	}

	_, layout := m.fileHistoryLayout()
	m.View()
	editor := m.document()
	content := m.renderFrame.content
	_, readDiff := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.sideX() + 2, Y: layout.start})
	if m.document() != editor || m.diffOpen || state.pending == nil {
		t.Fatal("a timeline click switched away before the diff was available")
	}
	if len(m.renderFrame.content) == 0 || &m.renderFrame.content[0] != &content[0] {
		t.Fatal("a timeline click invalidated unchanged editor rows")
	}
	assertFreshHoverFrame(t, m)
	drain(t, m, readDiff)
	if m.diffRepository != "child space" || m.diffPath != path || m.diffCommit == "" || !strings.Contains(m.patch.Raw, "+# Nested history example") {
		t.Fatalf("unexpected timeline diff: %s %s %s", m.diffRepository, m.diffPath, m.patch.Raw)
	}
	if m.fileHistory != state || !m.fileHistoryHasFocus() || !m.tabs[m.activeTab].preview {
		t.Fatal("preview did not preserve timeline and sidebar focus")
	}
	if m.treeList().height != treeHeight || len(m.sidebarLinesWidth(37)) != m.bodyHeight() {
		t.Fatal("hiding Outline for a diff changed the Explorer viewport")
	}
	_, timelineOnly := m.fileHistoryLayout()
	if timelineOnly.height != panelHeight+1 || timelineOnly.start+timelineOnly.height != m.bodyHeight() {
		t.Fatal("Timeline did not use the row freed by the Outline header")
	}
	readOlder := press(m, "j")
	oldDiff := state.pending
	if oldDiff == nil || m.diffCommit != state.revisions[0].ID {
		t.Fatal("pending diff replaced the visible preview")
	}
	drain(t, m, press(m, "k"))
	drain(t, m, readOlder)
	if state.pending != nil || oldDiff.diffOpen || m.diffCommit != state.revisions[0].ID {
		t.Fatal("an older request replaced the latest selection")
	}
	drain(t, m, press(m, "j"))
	if m.diffCommit != state.revisions[1].ID {
		t.Fatal("moving to an older commit did not preview it")
	}
	drain(t, m, press(m, "enter"))
	if m.sidebar || m.tabs[m.activeTab].preview {
		t.Fatal("Enter did not keep the preview and focus the editor")
	}

	m.openFileHistory("untracked.txt")
	pending := m.fileHistory
	drain(t, m, m.openFileInExplorer("README.md"))
	drain(t, m, m.syncFileHistory())
	m.Update(fileHistoryMsg{target: pending})
	if m.fileHistory == pending || m.fileHistory.path != "README.md" {
		t.Fatal("timeline did not follow the new active file")
	}

	loadEmpty := m.openFileHistory("untracked.txt")
	if m.treeList().height != treeHeight {
		t.Fatal("loading history changed the Explorer viewport")
	}
	drain(t, m, loadEmpty)
	if m.treeList().height != treeHeight {
		t.Fatal("an empty Timeline changed the Explorer viewport")
	}
	state = m.fileHistory
	if len(state.revisions) != 0 || m.fileHistoryMessage(state) != "No commit history for this file" {
		t.Fatalf("expected empty timeline: %+v", state)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if _, layout := m.fileHistoryLayout(); layout.height != 0 {
		t.Fatal("Left did not collapse the timeline")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if _, layout := m.fileHistoryLayout(); layout.height == 0 {
		t.Fatal("Right did not expand the timeline")
	}
	drain(t, m, press(m, "ctrl+4"))
	if m.section != history || m.fileHistory != nil || !m.items[0].repoHeader {
		t.Fatal("repository history was not reachable independently")
	}
}
