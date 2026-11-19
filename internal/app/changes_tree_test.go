package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
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
	m.sidebarCollapsed[changeKey("", false, "directory", "src")] = true
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
	if cmd := press(m, " "); cmd != nil || m.busy {
		t.Fatal("Space triggered a Git operation")
	}
	drain(t, m, press(m, "s"))
	if m.current() == nil || m.current().change == nil || m.current().change.Path != "cmd/main.go" || !m.current().staged {
		t.Fatal("staging lost selected tree file")
	}
	drain(t, m, press(m, "s"))
	if m.current() == nil || m.current().change == nil || m.current().change.Path != "cmd/main.go" || m.current().staged {
		t.Fatal("unstaging lost selected tree file")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.current() == nil || m.current().folder != "cmd" {
		t.Fatal("left did not select parent directory")
	}
	parentKey := m.current().key
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !m.sidebarCollapsed[parentKey] || m.current() == nil || m.current().key != parentKey {
		t.Fatal("right did not collapse selected directory")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.sidebarCollapsed[parentKey] {
		t.Fatal("right did not expand selected directory")
	}
	index := m.selected
	m.click(tea.MouseClickMsg{X: 5, Y: headerHeight + index - m.sideTop, Button: tea.MouseLeft})
	if !m.sidebarCollapsed[parentKey] {
		t.Fatal("folder click did not collapse tree")
	}
}

func TestChangesStageAndUnstageAllShortcutsWorkWithUnbornIndex(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	drain(t, m, m.selectSection(changes))
	selectChanged(t, m, "cmd/main.go", false)
	drain(t, m, press(m, "s"))
	selectChanged(t, m, "README.md", false)

	for _, tc := range []struct {
		oldKey string
		staged bool
	}{{"a", true}, {"A", false}} {
		if cmd := press(m, tc.oldKey); cmd != nil || m.busy {
			t.Fatalf("removed shortcut %q started a Git operation", tc.oldKey)
		}
		drain(t, m, press(m, "S"))
		if len(m.snapshot.Changes) != 3 {
			t.Fatalf("expected three changes, got %+v", m.snapshot.Changes)
		}
		for _, change := range m.snapshot.Changes {
			if change.Staged() != tc.staged || change.Unstaged() == tc.staged {
				t.Fatalf("expected staged=%v, got %+v", tc.staged, change)
			}
		}
	}
}

func TestChangesDiscardAllShortcutConfirmsAndPreservesIndex(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	drain(t, m, m.selectSection(changes))
	selectChanged(t, m, "README.md", false)
	drain(t, m, press(m, "s"))
	if cmd := press(m, "D"); cmd != nil || m.modal != "" {
		t.Fatal("D opened discard from a staged row")
	}
	selectChanged(t, m, "cmd/main.go", false)
	if cmd := press(m, "D"); cmd != nil || m.modal != "confirm" || m.busy {
		t.Fatal("repository discard did not wait for confirmation")
	}
	drain(t, m, press(m, "enter"))
	if m.modal != "" || len(m.snapshot.Changes) != 3 {
		t.Fatal("the default choice did not cancel repository discard")
	}
	press(m, "D")
	drain(t, m, press(m, "x"))
	if len(m.snapshot.Changes) != 1 || m.snapshot.Changes[0].Path != "README.md" || !m.snapshot.Changes[0].Staged() {
		t.Fatalf("discard affected the index or left worktree changes: %+v", m.snapshot.Changes)
	}
}

func TestChangesDiscardAndStashShortcuts(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	drain(t, m, m.selectSection(changes))
	selectChanged(t, m, "cmd/main.go", false)
	path := filepath.Join(m.repo.Root, "cmd", "main.go")

	press(m, "t")
	if m.modal != "stash" {
		t.Fatalf("expected stash prompt, got %q", m.modal)
	}
	press(m, "esc")
	press(m, "d")
	if m.modal != "confirm" {
		t.Fatalf("expected discard confirmation, got %q", m.modal)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	drain(t, m, press(m, "x"))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected untracked file to be discarded, got %v", err)
	}
	for _, identity := range [][2]string{{"user.name", "Test"}, {"user.email", "test@example.test"}} {
		if _, err := m.repo.Run(m.ctx, "config", identity[0], identity[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.repo.Run(m.ctx, "commit", "--no-gpg-sign", "--allow-empty", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	press(m, "t")
	drain(t, m, press(m, "enter"))
	if len(m.snapshot.Changes) != 0 {
		t.Fatalf("stash left changes in the working tree: %+v", m.snapshot.Changes)
	}
	saved, err := m.repo.Run(m.ctx, "stash", "show", "--include-untracked", "--name-only")
	if err != nil || !strings.Contains(saved, "README.md") {
		t.Fatalf("stash did not save the untracked file: %q, %v", saved, err)
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
	m.sidebarCollapsed[folderKey] = true
	m.rebuildItems()
	clickGroup := func(staged bool) {
		t.Helper()
		for i, entry := range m.items {
			if entry.header && !entry.repoHeader && entry.staged == staged {
				m.clickSidebar(tea.MouseClickMsg{X: 2, Y: headerHeight + i - m.sideTop, Button: tea.MouseLeft})
				if m.current() == nil || !m.current().header || m.current().staged != staged {
					t.Fatal("group focus was lost")
				}
				return
			}
		}
		t.Fatal("group header missing")
	}
	clickGroup(true)
	if !m.sidebarCollapsed[changeKey("", true, "group", "")] {
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
	if !m.sidebarCollapsed[folderKey] {
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
	if m.selected != 0 || m.sidebarCollapsed[repositoryKey("")] {
		t.Fatal("left must leave the root in place without folding")
	}
	press(m, "right")
	if !m.sidebarCollapsed[repositoryKey("")] || len(m.items) != 1 {
		t.Fatal("right did not collapse repository")
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
	if !m.sidebarCollapsed[changeKey("", false, "group", "")] {
		t.Fatal("right did not collapse an expanded group")
	}
	press(m, "left")
	if m.current() == nil || !m.current().repoHeader {
		t.Fatal("left did not select the group parent")
	}
	drain(t, m, clickScopeAction(t, m, "", true, false, "stage"))
	for _, c := range m.snapshot.Changes {
		if !c.Staged() {
			t.Fatalf("collapsed batch missed %s", c.Path)
		}
	}
	if !m.sidebarCollapsed[changeKey("", false, "group", "")] {
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
	press(m, "right")
	if !m.sidebarCollapsed[changeKey("", false, "directory", "apps/web/src")] {
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
	if !m.current().header || !m.expanded["apps/web/src/hooks"] {
		t.Fatal("left did not select the workspace root while keeping the folder expanded")
	}
	press(m, "down")
	drain(t, m, press(m, "right"))
	if m.expanded["apps/web/src/hooks"] {
		t.Fatal("right did not collapse the folder")
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
	m.selectParentFolder()
	if m.selected != 1 {
		t.Fatal("file did not select compact folder")
	}
	m.selectParentFolder()
	if m.selected != 0 {
		t.Fatal("compact folder did not select commit")
	}
}
