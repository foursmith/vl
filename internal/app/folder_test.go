package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
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
	m := folderWorkspace(t)
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
	if cmd := m.gitOperation("Fetch", "fetch"); cmd != nil {
		t.Fatal("Git operation allowed before initialization")
	}
	drain(t, m, press(m, "enter"))
	if m.snapshot.NoRepository || len(m.snapshot.Changes) != 1 || len(m.snapshot.GitDirectories) == 0 {
		t.Fatalf("initialization failed: %+v; %s", m.snapshot, m.notice)
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
	m := folderWorkspace(t)
	drain(t, m, m.selectSection(history))
	drain(t, m, m.clickSidebar(tea.MouseClickMsg{X: m.sideX() + 3, Y: m.sidebarHeaderRows() + 3, Button: tea.MouseLeft}))
	if m.snapshot.NoRepository {
		t.Fatal("mouse initialization failed")
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
