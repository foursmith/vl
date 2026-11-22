package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

func folderWorkspace(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello folder\n"), 0644); err != nil {
		t.Fatal(err)
	}
	repo, err := gitservice.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), repo, false)
	m.section = explorer
	t.Cleanup(m.Close)
	drain(t, m, m.refresh())
	return m
}

func TestFolderBrowseSearchEditAndInitialize(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(configDir, "gitconfig"))
	t.Setenv("GIT_CONFIG_COUNT", "0")
	branchConfig := filepath.Join(configDir, "branch.conf")
	if err := os.WriteFile(branchConfig, []byte("[init]\n\tdefaultBranch = other-default\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := folderWorkspace(t)
	key := "includeIf.gitdir:" + filepath.ToSlash(filepath.Join(m.repo.Root, ".git")) + ".path"
	if _, err := m.repo.Run(m.ctx, "config", "--global", key, branchConfig); err != nil {
		t.Fatal(err)
	}
	if !m.snapshot.NoRepository || len(m.treeEntries) != 1 || len(m.snapshot.Files) != 1 {
		t.Fatalf("folder did not load: %+v, %+v", m.snapshot, m.treeEntries)
	}
	drain(t, m, m.openQuickFiles())
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "hello.txt" {
		t.Fatalf("quick files: %+v", rows)
	}
	drain(t, m, press(m, "enter"))
	if m.document() == nil {
		t.Fatal("quick open failed")
	}
	m.document().Insert("edited ")
	doc := m.document()
	drain(t, m, m.selectSection(changes))
	if len(m.items) != 0 || !strings.Contains(strings.Join(m.sidebarLines(), "\n"), "Initialize Repository") {
		t.Fatal("missing folder empty state")
	}
	for _, a := range m.actions() {
		if strings.HasPrefix(a.id, "git.") && a.id != "git.init" && a.disabled == "" {
			t.Fatalf("Git action enabled: %s", a.id)
		}
	}
	if cmd := m.gitOperationInRepository("", "Fetch", "fetch"); cmd != nil {
		t.Fatal("Git operation allowed before initialization")
	}
	drain(t, m, press(m, "enter"))
	if m.snapshot.NoRepository || len(m.snapshot.Changes) != 1 || len(m.snapshot.GitDirectories) == 0 {
		t.Fatalf("initialization failed: %+v; %s", m.snapshot, m.notice)
	}
	if m.snapshot.Branch != "other-default" {
		t.Fatalf("initial branch = %q, want configured branch other-default", m.snapshot.Branch)
	}
	if m.document() != doc || !doc.Dirty() || !strings.Contains(doc.Text, "edited") {
		t.Fatal("initialization lost edits")
	}
	drain(t, m, m.save())
	data, err := os.ReadFile(doc.Path)
	if err != nil || string(data) != doc.Text {
		t.Fatal("save failed", err)
	}
}

func TestFolderWatcherAndRecentOpen(t *testing.T) {
	m := folderWorkspace(t)
	ready := m.startWatcher()().(watchReadyMsg)
	if ready.err != nil || ready.watcher == nil {
		t.Fatalf("folder watcher: %+v", ready)
	}
	ready.watcher.Close()
	target := folderWorkspace(t)
	drain(t, m, m.openRepository(target.repo.Root))
	if m.nextRepository == nil || m.nextRepository.Root != target.repo.Root {
		t.Fatal("recent folder did not open")
	}
}

func TestFolderExternalInitializationAndMouseEntry(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_COUNT", "0")
	m := folderWorkspace(t)
	drain(t, m, m.selectSection(history))
	drain(t, m, m.clickSidebar(tea.MouseClickMsg{X: m.sideX() + 3, Y: headerHeight + 3, Button: tea.MouseLeft}))
	if m.snapshot.NoRepository {
		t.Fatal("mouse initialization failed")
	}
	if m.snapshot.Branch != "main" {
		t.Fatalf("initial branch = %q, want main without init.defaultBranch", m.snapshot.Branch)
	}
	external := folderWorkspace(t)
	if _, err := external.repo.Run(external.ctx, "init"); err != nil {
		t.Fatal(err)
	}
	drain(t, external, external.refresh())
	if external.snapshot.NoRepository || len(external.snapshot.GitDirectories) == 0 {
		t.Fatal("external initialization not discovered")
	}
	for _, segment := range external.statusLayout() {
		if segment.id == "init" {
			t.Fatal("stale initialization status")
		}
	}
}
