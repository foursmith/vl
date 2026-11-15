package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func repositoryBranchWorkspace(t *testing.T) (*Model, *gitservice.Repository) {
	t.Helper()
	m, child := childWorkspace(t)
	if _, err := child.Run(m.ctx, "switch", "-c", "child-topic"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("# Child topic\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"commit", "-am", "child topic"}, {"tag", "child-tag"}, {"switch", "main"}} {
		if _, err := child.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.repo.Run(m.ctx, "branch", "parent-only"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	return m, child
}

func clickRepositoryBranch(t *testing.T, m *Model, part section, path string) {
	t.Helper()
	m.section = part
	m.sidebar = true
	m.rebuildItems()
	for i, entry := range m.items {
		if !entry.repoHeader || entry.repository != path {
			continue
		}
		m.selected = i
		m.ensureSidebar()
		folded := m.changeCollapsed[entry.key]
		w := m.sideWidth()
		if !m.sideShown() {
			w = m.width
		}
		x := w - 2
		if !m.repositoryBranchAt(entry, x, w) {
			t.Fatal("visible branch label has no click target")
		}
		drain(t, m, m.click(tea.MouseClickMsg{X: m.sideX() + x, Y: sidebarHeaderRows + i - m.sideTop, Button: tea.MouseLeft}))
		if m.modal != "refs" || m.branchRepository != path || m.changeCollapsed[entry.key] != folded {
			t.Fatal("branch click folded the repository or opened the wrong picker")
		}
		return
	}
	t.Fatalf("missing repository %q", path)
}

func pickBranchAction(t *testing.T, m *Model, id string) {
	t.Helper()
	m.input.SetValue(id)
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != id {
		t.Fatalf("picker target %q: %+v", id, rows)
	}
	drain(t, m, press(m, "enter"))
}

func TestRepositoryBranchLabelsReusePickerAndSwitchOnlyTheirRepository(t *testing.T) {
	m, child := repositoryBranchWorkspace(t)
	rootHead := m.snapshot.Head
	index, err := m.repo.Run(m.ctx, "ls-files", "--stage", "--", "child space")
	if err != nil {
		t.Fatal(err)
	}
	m.openFile("child space/README.md")
	clickRepositoryBranch(t, m, changes, "child space")
	for _, a := range m.palette {
		if a.id == "refs/heads/parent-only" {
			t.Fatal("child picker contains parent branches")
		}
	}
	pickBranchAction(t, m, "refs/heads/child-topic")
	if m.snapshot.Branch != "main" || m.snapshot.Head != rootHead || m.snapshot.RepositoryHistory["child space"].Branch != "child-topic" {
		t.Fatal("child branch switch modified parent")
	}
	if m.document().Text != "# Child topic\n" {
		t.Fatal("child checkout did not reload clean editor")
	}
	if got, err := m.repo.Run(m.ctx, "ls-files", "--stage", "--", "child space"); err != nil || got != index {
		t.Fatal("branch switch staged parent pointer")
	}
	clickRepositoryBranch(t, m, history, "child space")
	pickBranchAction(t, m, "refs/tags/child-tag")
	if !strings.Contains(m.snapshot.RepositoryHistory["child space"].Branch, "(分离)") || m.snapshot.Branch != "main" {
		t.Fatal("history branch picker detached the wrong repository")
	}
	clickRepositoryBranch(t, m, changes, "child space")
	pickBranchAction(t, m, "branch.createFrom")
	pickBranchAction(t, m, "refs/heads/main")
	if m.modal != "branch" || m.branchRepository != "child space" || m.branchStart == "" {
		t.Fatal("child create-from lost its repository")
	}
	m.input.SetValue("child-from-main")
	drain(t, m, press(m, "enter"))
	if m.snapshot.RepositoryHistory["child space"].Branch != "child-from-main" || m.snapshot.Branch != "main" {
		t.Fatal("create-from changed parent branch")
	}
	if _, err := child.Run(m.ctx, "show-ref", "--verify", "refs/heads/child-from-main"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.repo.Run(m.ctx, "show-ref", "--verify", "refs/heads/child-from-main"); err == nil {
		t.Fatal("child branch created in parent")
	}
	for _, s := range m.statusLayout() {
		if s.id == "branch" {
			drain(t, m, m.clickStatus(s.x+1))
			break
		}
	}
	if m.branchRepository != "" {
		t.Fatal("status bar reused last child target")
	}
	pickBranchAction(t, m, "refs/heads/parent-only")
	if m.snapshot.Branch != "parent-only" || m.snapshot.RepositoryHistory["child space"].Branch != "child-from-main" {
		t.Fatal("status bar checkout changed child branch")
	}
}

func TestRepositoryBranchPickerProtectsTargetBuffersAndStaleRefs(t *testing.T) {
	m, child := repositoryBranchWorkspace(t)
	m.openFile("child space/README.md")
	m.document().Insert("unsaved child")
	clickRepositoryBranch(t, m, changes, "child space")
	m.input.SetValue("refs/heads/child-topic")
	drain(t, m, press(m, "enter"))
	if m.modal != "refs" || m.busy || m.snapshot.RepositoryHistory["child space"].Branch != "main" {
		t.Fatal("branch picker ignored dirty child editor")
	}
	press(m, "esc")
	m.document().Undo()
	m.openFile("README.md")
	m.document().Insert("unsaved parent")
	clickRepositoryBranch(t, m, changes, "child space")
	if _, err := child.Run(m.ctx, "branch", "-f", "child-topic", "main"); err != nil {
		t.Fatal(err)
	}
	pickBranchAction(t, m, "refs/heads/child-topic")
	if m.snapshot.RepositoryHistory["child space"].Branch != "main" || !strings.Contains(m.notice, "reference changed") {
		t.Fatal("child picker ignored stale ref")
	}
	clickRepositoryBranch(t, m, history, "child space")
	pickBranchAction(t, m, "refs/heads/child-topic")
	if m.snapshot.RepositoryHistory["child space"].Branch != "child-topic" || !m.document().Dirty() || m.snapshot.Branch != "main" {
		t.Fatal("unrelated parent edits blocked or were lost during child checkout")
	}
}

func TestRepositoryBranchClickTargetsFollowSidebarLayout(t *testing.T) {
	m, _ := repositoryBranchWorkspace(t)
	data := m.snapshot.RepositoryHistory["child space"]
	data.Branch = "a-very-long-branch-name-that-is-truncated"
	m.snapshot.RepositoryHistory["child space"] = data
	for _, part := range []section{changes, history} {
		for _, width := range []int{30, 48, 120} {
			for _, right := range []bool{false, true} {
				m.SetSidebarLayout(32, right)
				m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
				clickRepositoryBranch(t, m, part, "child space")
				press(m, "esc")
			}
		}
	}
	// A canceled child picker cannot redirect the existing global create command.
	clickRepositoryBranch(t, m, changes, "child space")
	press(m, "esc")
	for _, a := range m.actions() {
		if a.id == "git.branch" {
			drain(t, m, a.run())
			break
		}
	}
	m.input.SetValue("created-in-parent")
	drain(t, m, press(m, "enter"))
	if m.snapshot.Branch != "created-in-parent" || m.snapshot.RepositoryHistory["child space"].Branch != "main" {
		t.Fatal("global create reused a child repository")
	}
}
