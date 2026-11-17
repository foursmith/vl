package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/theme"
)

func selectHistoryCommit(t *testing.T, m *Model, repo, id string) item {
	t.Helper()
	for i, e := range m.items {
		if e.commit != nil && e.repository == repo && e.commit.ID == id {
			m.selected = i
			m.sidebar = true
			return e
		}
	}
	t.Fatalf("missing commit %s in %q", id, repo)
	return item{}
}

func TestHistoryGroupsSharedCommitsByRepository(t *testing.T) {
	// Identical initial commits deliberately share an object ID across repositories.
	t.Setenv("GIT_AUTHOR_DATE", "2026-11-15T12:00:00+08:00")
	t.Setenv("GIT_COMMITTER_DATE", "2026-11-15T12:00:00+08:00")
	m, child := childWorkspace(t)
	shared := strings.TrimSpace(m.snapshot.RepositoryStates["child space"].Commits[0].ID)
	found := false
	for _, c := range m.snapshot.Commits {
		found = found || c.ID == shared
	}
	if !found {
		t.Fatal("fixture did not create a shared commit ID")
	}
	for _, target := range []struct {
		repo *gitservice.Repository
		tag  string
	}{{m.repo, "parent-only"}, {child, "child-only"}} {
		if _, err := target.repo.Run(m.ctx, "tag", "-a", target.tag, "-m", "release", shared); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	drain(t, m, m.selectSection(history))
	root := selectHistoryCommit(t, m, "", shared)
	drain(t, m, m.previewHistory(root))
	rootText, _ := m.historyRow(root, 120, theme.Dark().Panel)
	childEntry := selectHistoryCommit(t, m, "child space", shared)
	drain(t, m, m.previewHistory(childEntry))
	childText, _ := m.historyRow(childEntry, 120, theme.Dark().Panel)
	if !strings.Contains(plain(rootText), "parent-only") || strings.Contains(plain(rootText), "child-only") {
		t.Fatal("parent history uses child refs")
	}
	if !strings.Contains(plain(childText), "child-only") || strings.Contains(plain(childText), "parent-only") {
		t.Fatal("child history uses parent refs")
	}
	rootKey, childKey := historyKey("", shared), historyKey("child space", shared)
	if rootKey == childKey || !m.historyExpanded(rootKey) || !m.historyExpanded(childKey) {
		t.Fatal("shared commit identities collided")
	}
	for _, e := range m.historyTree(childKey) {
		if e.historyChange != nil && (!strings.HasPrefix(e.historyChange.Path, "child space/") || e.historyChange.Repository != "child space") {
			t.Fatal("child history file ownership lost")
		}
		if e.folder == "child space" {
			t.Fatal("child tree repeats its repository prefix")
		}
	}
	selectHistoryCommit(t, m, "", shared)
	drain(t, m, m.openSelected())
	if m.historyExpanded(rootKey) || !m.historyExpanded(childKey) {
		t.Fatal("parent fold collapsed child commit")
	}
	selectHistoryCommit(t, m, "child space", shared)
	drain(t, m, m.openSelected())
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.current().repoHeader || m.current().repository != "child space" {
		t.Fatal("collapsed commit did not navigate to its own repository")
	}
}

func TestHistoryChildOnlyCommitDiffAndRefresh(t *testing.T) {
	m, child := childWorkspace(t)
	rootHead := m.snapshot.Head
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("# child history only\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Run(m.ctx, "commit", "-am", "child history change"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	data := m.snapshot.RepositoryStates["child space"]
	if len(data.Commits) == 0 || data.Commits[0].Subject != "child history change" || m.snapshot.Head != rootHead {
		t.Fatal("child log refresh used parent history")
	}
	id := data.Commits[0].ID
	if _, err := m.repo.Run(m.ctx, "cat-file", "-e", id); err == nil {
		t.Fatal("child-only commit unexpectedly exists in parent object database")
	}
	drain(t, m, m.selectSection(history))
	entry := selectHistoryCommit(t, m, "child space", id)
	drain(t, m, m.openHistory(entry))
	if m.diffCommit != id || m.diffPath != "child space/README.md" || !strings.Contains(m.patch.Raw, "+# child history only") {
		t.Fatalf("child history diff: %s %s\n%s", m.diffCommit, m.diffPath, m.patch.Raw)
	}
	if cmd := m.stageHunk(false); cmd != nil {
		t.Fatal("history diff allowed staging")
	}
	if cmd := m.editDiff(); cmd != nil {
		t.Fatal("history diff allowed editing")
	}
	m.changeCollapsed[repositoryKey("child space")] = true
	for i, e := range m.items {
		if e.repoHeader && e.repository == "child space" {
			m.selected = i
			break
		}
	}
	drain(t, m, m.openSelected())
	if !m.changeCollapsed[historyRepositoryKey("child space")] {
		t.Fatal("history repository did not collapse")
	}
	for _, e := range m.items {
		if e.commit != nil && e.repository == "child space" {
			t.Fatal("collapsed child history remains visible")
		}
	}
	m.changeCollapsed[historyRepositoryKey("child space")] = false
	m.rebuildItems()
	// Commit expansion survives repository folding, independently of Changes.
	if !m.historyExpanded(historyKey("child space", id)) || !m.changeCollapsed[repositoryKey("child space")] {
		t.Fatal("repository folds leaked across tabs")
	}
	for _, width := range []int{30, 48, 80, 120} {
		for _, nerd := range []bool{false, true} {
			m.nerd = nerd
			m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != width {
					t.Fatalf("history layout overflow at %d", width)
				}
			}
		}
	}
}

func TestHistoryRepliesWithSameIDStayInTheirRepository(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: t.TempDir()}, false)
	t.Cleanup(m.Close)
	id := strings.Repeat("a", 40)
	commits := []gitservice.Commit{{ID: id, Subject: "shared"}}
	m.snapshot = gitservice.Snapshot{Commits: commits, Repositories: []string{"child"}, RepositoryStates: map[string]gitservice.Snapshot{"child": {Commits: commits}}}
	m.section = history
	m.rebuildItems()
	root := selectHistoryCommit(t, m, "", id)
	m.previewHistory(root)
	rootKey := historyKey("", id)
	rootSeq := m.historyStates[rootKey].seq
	child := selectHistoryCommit(t, m, "child", id)
	m.previewHistory(child)
	key := historyKey("child", id)
	childSeq := m.historyStates[key].seq
	m.applyHistoryFiles(historyFilesMsg{repository: "child", seq: childSeq, id: id, files: []gitservice.Change{{Repository: "child", Path: "child/src/child.go", Index: 'M', Worktree: ' '}}})
	activeSeq := m.diffSeq
	if cmd := m.applyHistoryFiles(historyFilesMsg{seq: rootSeq, id: id, files: []gitservice.Change{{Path: "src/root.go", Index: 'M', Worktree: ' '}}}); cmd != nil || m.historyCommit != key || m.diffSeq != activeSeq {
		t.Fatal("late parent reply replaced child preview")
	}
	seenRoot, seenChild := false, false
	for _, e := range m.items {
		if e.historyChange == nil {
			continue
		}
		seenRoot = seenRoot || e.historyID == rootKey && e.historyChange.Path == "src/root.go"
		seenChild = seenChild || e.historyID == key && e.historyChange.Path == "child/src/child.go"
	}
	if !seenRoot || !seenChild {
		t.Fatal("shared-ID trees overwrote each other")
	}
	m.snapshot.Repositories = nil
	m.reconcileHistoryRepositories()
	if m.historyStates[key] != nil || m.historyStates[rootKey] == nil || m.diffOpen {
		t.Fatal("removed child history was not released independently")
	}
	if cmd := m.applyHistoryFiles(historyFilesMsg{repository: "child", seq: childSeq, id: id}); cmd != nil {
		t.Fatal("removed repository reply reopened history")
	}
}

func TestHistoryEmptyAndNestedRepositorySections(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/root"}, false)
	t.Cleanup(m.Close)
	id := strings.Repeat("b", 40)
	m.snapshot = gitservice.Snapshot{
		Branch: "main", Repositories: []string{"app", "app/nested"},
		RepositoryStates: map[string]gitservice.Snapshot{"app": {Branch: "dev"}, "app/nested": {Branch: "release", Commits: []gitservice.Commit{{ID: id, Subject: "nested commit"}}}},
	}
	m.section = history
	m.rebuildItems()
	headers, empty := 0, 0
	for _, e := range m.items {
		if e.repoHeader {
			headers++
		}
		if e.label == "No commits yet" {
			empty++
		}
	}
	if headers != 3 || empty != 2 {
		t.Fatal("empty or nested repository sections missing")
	}
	for _, entry := range m.items {
		if entry.label == "No commits yet" {
			m.sidebar = true
			if cmd := m.openHistory(entry); cmd != nil || !m.sidebar {
				t.Fatal("empty history note stole focus")
			}
		}
	}
	nested := selectHistoryCommit(t, m, "app/nested", id)
	m.selectHistoryParent(nested)
	if !m.current().repoHeader || m.current().repository != "app/nested" {
		t.Fatal("nested commit selected wrong parent repository")
	}
}

func TestHistoryCollapsedRepositoryDoesNotOpenLatePreview(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: t.TempDir()}, false)
	t.Cleanup(m.Close)
	id := strings.Repeat("c", 40)
	m.snapshot = gitservice.Snapshot{
		Repositories:     []string{"child"},
		RepositoryStates: map[string]gitservice.Snapshot{"child": {Commits: []gitservice.Commit{{ID: id, Subject: "child commit"}}}},
	}
	m.section = history
	m.rebuildItems()
	m.previewHistory(selectHistoryCommit(t, m, "child", id))
	key := historyKey("child", id)
	seq := m.historyStates[key].seq
	for i, e := range m.items {
		if e.repoHeader && e.repository == "child" {
			m.selected = i
			break
		}
	}
	m.openHistory(*m.current())
	cmd := m.applyHistoryFiles(historyFilesMsg{repository: "child", id: id, seq: seq, files: []gitservice.Change{{Repository: "child", Path: "child/file.go", Index: 'M', Worktree: ' '}}})
	if cmd != nil || m.diffOpen || !m.current().repoHeader || m.current().repository != "child" {
		t.Fatal("late result opened preview inside collapsed repository")
	}
	if !m.historyStates[key].loaded {
		t.Fatal("collapsed repository lost its completed file cache")
	}
}
