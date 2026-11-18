package app

import (
	"github.com/foursmith/vl/internal/watcher"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalReloadPreservesScrollAndOwnSaveHistory(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	original := m.document()
	path := original.Path
	original.Insert("// edit\n")
	if err := original.Save(); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{path}))
	if m.document() != original {
		t.Fatal("own save replaced document/history")
	}
	original.Undo()
	if !original.Dirty() {
		t.Fatal("own save discarded undo history")
	}
	original.Redo()
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 100)), 0644); err != nil {
		t.Fatal(err)
	}
	m.tabs[0].top = 5
	drain(t, m, m.reloadWatchedFiles([]string{path}))
	if m.document() == original || m.document().Dirty() || m.tabs[0].top != 5 {
		t.Fatal("clean external reload lost scroll or dirty state")
	}
}
func TestExternalReloadKeepsEditsMadeWhileReadWasPending(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	original := m.document()
	path := original.Path
	cmd := m.reloadWatchedFiles([]string{path})
	original.Insert("// unsaved\n")
	if err := os.WriteFile(path, []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, cmd)
	if m.document() != original || !strings.Contains(original.Text, "unsaved") || !strings.Contains(m.notice, "unsaved") {
		t.Fatal("external reload overwrote pending local edits")
	}
	if err := original.Save(); err == nil {
		t.Fatal("watcher updated dirty buffer's disk baseline")
	}
}
func TestReloadRejectsStaleReadsAndKeepsDeletedBuffer(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	path := m.document().Path
	// Materialize the older result before the second external write.
	old := m.reloadWatchedFiles([]string{path})()
	if err := os.WriteFile(path, []byte("newest\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{path}))
	m.Update(old)
	if m.document().Text != "newest\n" {
		t.Fatal("stale reload won")
	}
	current := m.document()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{path}))
	if m.document() != current || current.Text != "newest\n" {
		t.Fatal("external deletion closed/lost buffer")
	}
	if err := os.WriteFile(filepath.Clean(path), []byte("recreated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{path}))
	if m.document().Text != "recreated\n" {
		t.Fatal("recreated file was not reloaded")
	}
}

func TestWatchRefreshCoalescesWhileSnapshotIsPending(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	m.loading = true
	m.handleWatch(watcher.Batch{Paths: []string{filepath.Join(m.repo.Root, "new.txt")}})
	m.Update(nil)
	if m.refreshSeq != 0 || !m.watchDirty {
		t.Fatal("event cancelled/restarted a pending snapshot")
	}
	m.Update(snapshotMsg{seq: 0})
	if m.refreshSeq != 1 || m.watchDirty || !m.loading {
		t.Fatal("snapshot completion did not schedule one follow-up refresh")
	}
	m.handleWatch(watcher.Batch{Paths: []string{filepath.Join(m.repo.Root, "another.txt")}})
	m.Update(nil)
	if m.refreshSeq != 1 || !m.watchDirty {
		t.Fatal("burst restarted the follow-up snapshot")
	}
}

func TestUndoToCleanAppliesDeferredExternalChange(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Insert("local")
	if err := os.WriteFile(d.Path, []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{d.Path}))
	if m.document() != d {
		t.Fatal("dirty content replaced")
	}
	d.Undo()
	_, cmd := m.Update(nil)
	drain(t, m, cmd)
	if m.document().Text != "external\n" || m.tabs[0].reloadWhenClean {
		t.Fatal("returning to clean state left stale external content")
	}
}

func TestMetadataLockDoesNotRefreshWorkspace(t *testing.T) {
	m := tabWorkspace(t, 0)
	metadata := filepath.Join(m.repo.Root, ".git")
	m.snapshot.GitDirectories = []string{metadata}
	m.handleWatch(watcher.Batch{Paths: []string{filepath.Join(metadata, "index.lock")}})
	if m.watchDirty {
		t.Fatal("Git lock file scheduled a refresh")
	}
	m.handleWatch(watcher.Batch{Paths: []string{filepath.Join(metadata, "index")}})
	if !m.watchDirty {
		t.Fatal("Git index update did not schedule a refresh")
	}
}
