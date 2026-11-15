package app

import (
	"os"
	"path/filepath"
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
	if stagedFile == nil || workingFile == nil || stagedFile.key == workingFile.key || stagedFile.depth != 1 || stagedFile.label != "main.go" || stagedFile.change != workingFile.change {
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
	folderKey := changeKey("", false, "directory", "src/app")
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

func TestCompactChangeTreeNavigationAndFolding(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = changes
	m.snapshot.Changes = []gitservice.Change{
		{Path: "apps/web/src/hooks/use.go", Index: 'M', Worktree: 'M'},
		{Path: "apps/web/src/components/view.go", Index: ' ', Worktree: 'M'},
	}
	m.rebuildItems()
	selectFolder := func(path string) {
		t.Helper()
		for i, entry := range m.items {
			if !entry.staged && entry.folder == path {
				m.selected = i
				return
			}
		}
		t.Fatalf("missing folder %s", path)
	}
	selectFolder("apps/web/src/hooks")
	if m.current().label != "hooks" {
		t.Fatal("branch label lost")
	}
	m.selectParentFolder()
	if m.current().folder != "apps/web/src" || m.current().label != "apps/web/src" {
		t.Fatalf("parent: %+v", m.current())
	}
	m.selectParentFolder()
	if !m.current().header {
		t.Fatal("compact root did not select group")
	}
	selectFolder("apps/web/src")
	press(m, "left")
	if !m.changeCollapsed[changeKey("", false, "directory", "apps/web/src")] {
		t.Fatal("compact folder did not collapse")
	}
	press(m, "right")
	selectFolder("apps/web/src/hooks")
}

func TestCompactExplorerNavigation(t *testing.T) {
	m := explorerWorkspace(t)
	if err := os.MkdirAll(filepath.Join(m.repo.Root, "apps/web/src/hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, "apps/web/src/hooks/use.go"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refreshTree())
	for i, entry := range m.items {
		if entry.folder == "apps/web/src/hooks" {
			m.selected = i
			break
		}
	}
	if m.current() == nil || m.current().label != "apps/web/src/hooks" {
		t.Fatal("compact row missing")
	}
	drain(t, m, press(m, "right"))
	press(m, "down")
	if m.current().file != "apps/web/src/hooks/use.go" {
		t.Fatal("expanded file missing")
	}
	press(m, "left")
	if m.current().folder != "apps/web/src/hooks" {
		t.Fatal("parent navigation failed")
	}
	drain(t, m, press(m, "left"))
	if m.expanded["apps/web/src/hooks"] {
		t.Fatal("collapse failed")
	}
}

func TestCompactHistoryParentNavigation(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = history
	commit := gitservice.Commit{ID: "compact-history"}
	id := historyKey("", commit.ID)
	m.historyStates[id] = &historyState{expanded: true, loaded: true, collapsed: map[string]bool{}, files: []gitservice.Change{{Path: "apps/web/src/hooks/use.go", Index: 'M', Worktree: ' '}}}
	m.items = append([]item{{commit: &commit}}, m.historyTree(id)...)
	if len(m.items) != 3 || m.items[1].label != "apps/web/src/hooks" {
		t.Fatalf("history tree: %+v", m.items)
	}
	m.selected = 2
	m.selectHistoryParent(*m.current())
	if m.selected != 1 {
		t.Fatal("file did not select compact folder")
	}
	m.selectHistoryParent(*m.current())
	if m.selected != 0 {
		t.Fatal("compact folder did not select commit")
	}
}
