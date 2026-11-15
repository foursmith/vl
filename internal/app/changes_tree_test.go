package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func TestChangeTreeSeparatesIndexAndWorktreeAndRetainsFileIdentity(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = changes
	m.snapshot.Changes = []gitservice.Change{
		{Path: "src/app/main.go", Index: 'M', Worktree: 'M'},
		{Path: "src/pkg/util.go", Index: ' ', Worktree: 'M'},
		{Path: "README.md", Index: '?', Worktree: '?'},
		{Path: "gone/file.go", Index: 'D', Worktree: ' '},
		{Path: "src", Index: ' ', Worktree: 'D'},
	}
	staged, stagedCount := m.repositoryChangesTree("", true)
	working, workingCount := m.repositoryChangesTree("", false)
	if stagedCount != 2 || workingCount != 4 {
		t.Fatalf("file counts: %d / %d", stagedCount, workingCount)
	}
	var stagedFile, workingFile *item
	for i := range staged {
		if staged[i].change != nil && staged[i].change.Path == "src/app/main.go" {
			stagedFile = &staged[i]
		}
	}
	for i := range working {
		if working[i].change != nil && working[i].change.Path == "src/app/main.go" {
			workingFile = &working[i]
		}
	}
	if stagedFile == nil || workingFile == nil || stagedFile.key == workingFile.key || stagedFile.depth != 2 || stagedFile.label != "main.go" || stagedFile.change != workingFile.change {
		t.Fatal("file identity or hierarchy lost")
	}
	m.changeCollapsed[changeKey("", false, "directory", "src")] = true
	working, count := m.repositoryChangesTree("", false)
	if count != 4 {
		t.Fatal("collapse changed group file count")
	}
	for _, row := range working {
		if row.change != nil && strings.HasPrefix(row.change.Path, "src/") {
			t.Fatal("collapsed descendants remain visible")
		}
	}
	staged, _ = m.repositoryChangesTree("", true)
	found := false
	for _, row := range staged {
		if row.change != nil && row.change.Path == "src/app/main.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("worktree collapse affected index tree")
	}
	fileAndDirectory := 0
	for _, row := range working {
		if row.folder == "src" || row.change != nil && row.change.Path == "src" {
			fileAndDirectory++
		}
	}
	if fileAndDirectory != 2 {
		t.Fatal("file-to-directory replacement lost one side")
	}
}

func TestTreeStageUnstageAndParentNavigation(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	for i, row := range m.items {
		if row.change != nil && row.change.Path == "cmd/main.go" {
			m.selected = i
			break
		}
	}
	drain(t, m, m.preview())
	if m.diffPath != "cmd/main.go" || m.patch.Raw == "" {
		t.Fatal("tree file did not open diff")
	}
	drain(t, m, m.stageSelected(false))
	if m.current() == nil || m.current().change == nil || m.current().change.Path != "cmd/main.go" || !m.current().staged {
		t.Fatal("staging lost selected tree file")
	}
	drain(t, m, m.stageSelected(true))
	if m.current() == nil || m.current().change == nil || m.current().change.Path != "cmd/main.go" || m.current().staged {
		t.Fatal("unstaging lost selected tree file")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.current() == nil || m.current().folder != "cmd" {
		t.Fatal("left did not select parent directory")
	}
	parentKey := m.current().key
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.changeCollapsed[parentKey] || m.current() == nil || m.current().key != parentKey {
		t.Fatal("left did not collapse selected directory")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.changeCollapsed[parentKey] {
		t.Fatal("right did not expand selected directory")
	}
	index := m.selected
	m.click(tea.MouseClickMsg{X: 5, Y: sidebarHeaderRows + index - m.sideTop, Button: tea.MouseLeft})
	if !m.changeCollapsed[parentKey] {
		t.Fatal("folder click did not collapse tree")
	}
}

func TestChangeGroupsCollapseIndependentlyAndKeepFolderState(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = changes
	m.snapshot.Changes = []gitservice.Change{
		{Path: "src/app/main.go", Index: 'M', Worktree: 'M'},
		{Path: "README.md", Index: ' ', Worktree: 'M'},
	}
	m.rebuildItems()
	folderKey := changeKey("", false, "directory", "src")
	m.changeCollapsed[folderKey] = true
	m.rebuildItems()
	clickGroup := func(staged bool) {
		t.Helper()
		for i, entry := range m.items {
			if entry.header && !entry.repoHeader && entry.staged == staged {
				m.clickSidebar(tea.MouseClickMsg{X: 2, Y: sidebarHeaderRows + i - m.sideTop, Button: tea.MouseLeft})
				if m.current() == nil || !m.current().header || m.current().staged != staged {
					t.Fatal("group focus was lost")
				}
				return
			}
		}
		t.Fatal("group header missing")
	}
	clickGroup(true)
	if !m.changeCollapsed[changeKey("", true, "group", "")] {
		t.Fatal("staged group did not collapse")
	}
	working := 0
	for _, entry := range m.items {
		if !entry.header && entry.staged {
			t.Fatal("collapsed staged rows remain visible")
		}
		if !entry.header && !entry.staged {
			working++
		}
	}
	if working != 2 {
		t.Fatalf("worktree rows changed: %d", working)
	}
	clickGroup(false)
	if len(m.items) != 3 {
		t.Fatal("collapsed groups must leave the repository header and two group headers")
	}
	m.rebuildItems()
	if len(m.items) != 3 || m.current() == nil || !m.current().header {
		t.Fatal("refresh lost collapse or focus")
	}
	clickGroup(false)
	if !m.changeCollapsed[folderKey] {
		t.Fatal("expanding group lost nested folder state")
	}
	for _, entry := range m.items {
		if entry.change != nil && entry.change.Path == "src/app/main.go" {
			t.Fatal("nested collapsed contents became visible")
		}
	}
	clickGroup(true)
	found := false
	for _, entry := range m.items {
		if entry.change != nil && entry.staged {
			found = true
		}
	}
	if !found {
		t.Fatal("staged group did not expand")
	}
}

func TestChangeGroupsKeyboardAndCollapsedBatchActions(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	// Ordinary repositories use the same header and omit empty change groups.
	m.selected = 0
	press(m, "left")
	if !m.changeCollapsed[repositoryKey("")] || len(m.items) != 1 {
		t.Fatal("left did not collapse repository")
	}
	press(m, "right")
	press(m, "down")
	if m.current() == nil || !m.current().header || m.current().repoHeader || m.current().staged {
		t.Fatal("keyboard skipped worktree group")
	}
	press(m, "enter")
	if len(m.items) != 2 {
		t.Fatal("enter did not collapse changes group")
	}
	press(m, "right")
	if len(m.items) <= 2 {
		t.Fatal("right did not expand changes group")
	}
	press(m, "right")
	if m.changeCollapsed[changeKey("", false, "group", "")] {
		t.Fatal("right collapsed an expanded group")
	}
	press(m, "left")
	drain(t, m, clickScopeAction(t, m, "", true, false, "stage"))
	for _, c := range m.snapshot.Changes {
		if !c.Staged() {
			t.Fatalf("collapsed batch missed %s", c.Path)
		}
	}
	if !m.changeCollapsed[changeKey("", false, "group", "")] {
		t.Fatal("batch action reset group state")
	}
}
