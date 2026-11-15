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

func childWorkspace(t *testing.T) (*Model, *gitservice.Repository) {
	t.Helper()
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	source := explorerWorkspace(t)
	parent := explorerWorkspace(t)
	t.Cleanup(source.Close)
	t.Cleanup(parent.Close)
	for _, m := range []*Model{source, parent} {
		for _, args := range [][]string{{"config", "user.name", "Test"}, {"config", "user.email", "test@example.test"}, {"config", "commit.gpgsign", "false"}, {"add", "."}, {"commit", "-m", "initial"}} {
			if _, err := m.repo.Run(m.ctx, args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := parent.repo.Run(parent.ctx, "submodule", "add", source.repo.Root, "child space"); err != nil {
		t.Fatal(err)
	}
	if _, err := parent.repo.Run(parent.ctx, "commit", "-am", "add child"); err != nil {
		t.Fatal(err)
	}
	child, err := parent.repo.RepositoryAt(parent.ctx, "child space")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config", "user.name", "Test"}, {"config", "user.email", "test@example.test"}, {"config", "commit.gpgsign", "false"}} {
		if _, err := child.Run(parent.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, parent, parent.refresh())
	return parent, child
}

func selectChanged(t *testing.T, m *Model, path string, staged bool) {
	t.Helper()
	m.section = changes
	m.sidebar = true
	m.rebuildItems()
	for i, e := range m.items {
		if e.change != nil && e.change.Path == path && e.staged == staged {
			m.selected = i
			return
		}
	}
	t.Fatalf("missing %s staged=%v: %+v", path, staged, m.items)
}

func selectRepository(t *testing.T, m *Model, path string) {
	t.Helper()
	m.section = changes
	m.sidebar = true
	m.rebuildItems()
	for i, e := range m.items {
		if e.repoHeader && e.repository == path {
			m.selected = i
			return
		}
	}
	t.Fatalf("repository header missing: %s", path)
}

func TestChangesGroupRepositoriesWithLocalPathsAndParentSummaries(t *testing.T) {
	m, child := childWorkspace(t)
	m.section = changes
	m.rebuildItems()
	headers := 0
	for _, e := range m.items {
		if e.repoHeader {
			headers++
		}
		if e.header && !e.repoHeader {
			t.Fatal("clean repository contains empty change groups")
		}
		if e.repoHeader && m.commitDisabled(e.repository) == "" {
			t.Fatal("clean commit enabled")
		}
	}
	if headers != 2 {
		t.Fatal("clean child must remain visible")
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, "README.md"), []byte("parent edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("child edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	var root, childFiles, pointers int
	for _, e := range m.items {
		if e.change == nil {
			continue
		}
		if e.change.Gitlink {
			pointers++
			if e.repository != "" || e.change.Path != "child space" {
				t.Fatal("parent summary moved into child group")
			}
			text, _ := m.explorerRow(e, 50, theme.Panel)
			if !strings.HasSuffix(plain(text), "S") {
				t.Fatal("submodule summary lacks S marker")
			}
		} else if e.repository == "" {
			root++
		} else {
			childFiles++
			if e.repository != "child space" || e.label != "README.md" || e.depth != 1 {
				t.Fatalf("child path is not repository-relative: %+v", e)
			}
		}
	}
	if root != 1 || childFiles != 1 || pointers != 1 {
		t.Fatalf("groups: root=%d child=%d pointers=%d", root, childFiles, pointers)
	}
	for _, e := range m.items {
		if e.repoHeader && !strings.Contains(e.branch, "main*") {
			t.Fatalf("branch state missing: %q", e.label)
		}
	}
}

func TestRepositoryStageAndCommitDoNotCrossIndexes(t *testing.T) {
	m, child := childWorkspace(t)
	for _, root := range []string{m.repo.Root, child.Root} {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	if _, err := m.repo.Run(m.ctx, "config", "submodule.recurse", "true"); err != nil {
		t.Fatal(err)
	}
	selectRepository(t, m, "")
	drain(t, m, m.stageAll())
	for _, c := range m.snapshot.Changes {
		if c.Repository == "child space" && c.Staged() {
			t.Fatal("parent stage all staged child index")
		}
	}
	selectChanged(t, m, "README.md", true)
	drain(t, m, m.stageSelected(true))
	selectRepository(t, m, "child space")
	drain(t, m, m.stageAll())
	for _, c := range m.snapshot.Changes {
		if c.Repository == "child space" && !c.Staged() {
			t.Fatal("child file not staged")
		}
		if c.Repository == "" && c.Staged() {
			t.Fatal("child stage all staged parent")
		}
	}
	rootHead, err := m.repo.Run(m.ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	selectRepository(t, m, "")
	if cmd := m.commit(); cmd != nil || m.modal != "" {
		t.Fatal("parent commit used child staged files")
	}
	selectRepository(t, m, "child space")
	drain(t, m, m.commit())
	if m.modal != "commit" || !strings.Contains(m.modalTitle, "child space [main]") {
		t.Fatal("child commit target not visible")
	}
	m.input.SetValue("child commit")
	drain(t, m, press(m, "enter"))
	if got, err := m.repo.Run(m.ctx, "rev-parse", "HEAD"); err != nil || got != rootHead {
		t.Fatal("child commit changed parent HEAD")
	}
	if got, err := child.Run(m.ctx, "log", "-1", "--format=%s"); err != nil || strings.TrimSpace(got) != "child commit" {
		t.Fatal("child commit failed")
	}
	for _, c := range m.snapshot.Changes {
		if c.Repository == "child space" {
			t.Fatal("child commit left tracked changes")
		}
		if c.Path == "child space" && (c.GitlinkDirty || c.Staged()) {
			t.Fatal("child commit automatically staged parent pointer")
		}
	}
}

func TestRepositoryCommitDraftsAreIndependentAndFailuresKeepDraft(t *testing.T) {
	m, child := childWorkspace(t)
	for _, root := range []string{m.repo.Root, child.Root} {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	for _, target := range []string{"", "child space"} {
		selectRepository(t, m, target)
		drain(t, m, m.stageAll())
		drain(t, m, m.commit())
		m.input.SetValue("draft:" + target)
		drain(t, m, press(m, "esc"))
	}
	selectRepository(t, m, "child space")
	drain(t, m, m.commit())
	if m.input.Value() != "draft:child space" {
		t.Fatal("child draft lost")
	}
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Run(m.ctx, "config", "core.hooksPath", hooks); err != nil {
		t.Fatal(err)
	}
	drain(t, m, press(m, "enter"))
	if m.commitDrafts["child space"] != "draft:child space" {
		t.Fatal("failed commit cleared child draft")
	}
	drain(t, m, m.commit())
	if m.input.Value() != "draft:child space" {
		t.Fatal("retry lost draft")
	}
	if err := os.Remove(filepath.Join(hooks, "pre-commit")); err != nil {
		t.Fatal(err)
	}
	drain(t, m, press(m, "enter"))
	if m.commitDrafts["child space"] != "" || m.commitDrafts[""] != "draft:" {
		t.Fatal("successful child commit cleared wrong draft")
	}
	selectRepository(t, m, "")
	drain(t, m, m.commit())
	if m.input.Value() != "draft:" {
		t.Fatal("parent draft changed")
	}
}

func TestRepositoryFoldNavigationAndScopeStayIndependent(t *testing.T) {
	m, child := childWorkspace(t)
	for _, root := range []string{m.repo.Root, child.Root} {
		if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("changed\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	selectChanged(t, m, "child space/cmd/main.go", false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.current().folder != "child space/cmd" || len(m.scopedChanges(*m.current())) != 1 {
		t.Fatal("child folder scope crossed repository boundary")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.current().header || m.current().repoHeader || m.current().repository != "child space" {
		t.Fatal("left selected another repository's group")
	}
	scope := m.scopedChanges(*m.current())
	if len(scope) != 1 || scope[0].Repository != "child space" {
		t.Fatal("child group includes parent files")
	}
	m.changeCollapsed[m.current().key] = true
	m.rebuildItems()
	if rows, count := m.repositoryChangesTree("", false); len(rows) == 0 || count != 2 {
		t.Fatal("child fold affected parent group")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.current().repoHeader || m.current().repository != "child space" {
		t.Fatal("left did not select repository header")
	}
	drain(t, m, m.openSelected())
	if !m.changeCollapsed[repositoryKey("child space")] {
		t.Fatal("repository did not collapse")
	}
	seenParent := false
	for _, e := range m.items {
		if e.change != nil && e.change.Repository == "child space" {
			t.Fatal("collapsed child files remain visible")
		}
		if e.change != nil && e.change.Repository == "" {
			seenParent = true
		}
	}
	if !seenParent {
		t.Fatal("child collapse hid parent files")
	}
}

func TestRepositoryHeaderMouseCommitAndFrameWidths(t *testing.T) {
	m, _ := childWorkspace(t)
	childPath := filepath.Join(m.repo.Root, "child space", "README.md")
	if err := os.WriteFile(childPath, []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	selectRepository(t, m, "child space")
	drain(t, m, m.stageSelected(false))
	for _, width := range []int{30, 48, 120} {
		for _, height := range []int{8, 24} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: height})
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != width {
					t.Fatalf("frame overflow at %dx%d", width, height)
				}
			}
		}
	}
	selectRepository(t, m, "child space")
	m.ensureSidebar()
	var commit rowAction
	for _, a := range m.changeRowActions(*m.current(), m.sideWidth()) {
		if a.id == "commit" {
			commit = a
		}
	}
	if commit.disabled != "" {
		t.Fatal(commit.disabled)
	}
	// Repository header buttons are visible, so a prior hover is unnecessary.
	m.hoverX, m.hoverY = -1, -1
	drain(t, m, m.clickSidebar(tea.MouseClickMsg{X: m.sideX() + commit.x + 1, Y: sidebarHeaderRows + m.selected - m.sideTop, Button: tea.MouseLeft}))
	if m.modal != "commit" || m.commitRepository != "child space" {
		t.Fatal("header commit targeted wrong repository")
	}
	press(m, "esc")
}

func TestChildIndexDoesNotMarkParentStaged(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/newsnext"}, false)
	t.Cleanup(m.Close)
	m.snapshot = gitservice.Snapshot{Branch: "dev", Changes: []gitservice.Change{
		{Path: "app-config.dev", Index: ' ', Worktree: 'M', Gitlink: true, GitlinkDirty: true},
		{Path: "app-config.dev/startup.json", Repository: "app-config.dev", Index: 'A', Worktree: ' '},
	}}
	for _, segment := range m.statusLayout() {
		if segment.id == "branch" && strings.Contains(segment.text, "+") {
			t.Fatal("child index leaked into parent branch status")
		}
	}
}

func TestDirtyEditorInParentDoesNotBlockChildStaging(t *testing.T) {
	m, child := childWorkspace(t)
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("child change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	m.openFile("README.md")
	m.document().Insert("unsaved parent edit")
	selectRepository(t, m, "child space")
	drain(t, m, m.stageAll())
	for _, c := range m.snapshot.Changes {
		if c.Repository == "child space" && !c.Staged() {
			t.Fatal("unrelated dirty parent editor blocked child staging")
		}
	}
	selectRepository(t, m, "")
	if cmd := m.stageAll(); cmd != nil {
		t.Fatal("parent staging ignored its own dirty editor")
	}
}
