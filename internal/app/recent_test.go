package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRecentFoldersPickerSearchAndOpen(t *testing.T) {
	m := explorerWorkspace(t)
	target := explorerWorkspace(t)
	old := m.repo
	press(m, "ctrl+r")
	if m.modal != "recent" || !m.isPicker() {
		t.Fatal("Ctrl+R did not open recent folder picker")
	}
	m.Update(recentFoldersMsg{seq: m.recentSeq, folders: []string{m.repo.Root, target.repo.Root}})
	m.input.SetValue(target.repo.Root)
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != target.repo.Root {
		t.Fatalf("folder search: %+v", rows)
	}
	drain(t, m, press(m, "enter"))
	if m.NextRepository() == nil || m.NextRepository().Root != target.repo.Root {
		t.Fatal("selected repository was not handed to runner")
	}
	if m.repo != old {
		t.Fatal("old model was mutated instead of replaced")
	}
}

func TestRecentFolderResultsRespectDismissalAndNewRequests(t *testing.T) {
	m := explorerWorkspace(t)
	m.openRecentFolders()
	stale := recentFoldersMsg{seq: m.recentSeq, folders: []string{m.repo.Root}}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(stale)
	if m.modal != "" || len(m.palette) != 0 {
		t.Fatal("dismissed picker was reopened")
	}
	m.openRecentFolders()
	m.Update(stale)
	if len(m.palette) != 0 {
		t.Fatal("stale request populated new picker")
	}
	m.Update(recentFoldersMsg{seq: m.recentSeq, err: errors.New("unreadable history")})
	if m.modal != "" {
		t.Fatal("history error left picker loading")
	}
}

func TestRepositorySwitchProtectsDirtyFilesAndBusyOperations(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	doc := m.document()
	target := explorerWorkspace(t)
	doc.Insert("edited")
	if cmd := m.openRepository(target.repo.Root); cmd != nil {
		t.Fatal("dirty editor allowed switching")
	}
	doc.Undo()
	m.busy = true
	if cmd := m.openRepository(target.repo.Root); cmd != nil {
		t.Fatal("busy operation allowed switching")
	}
	m.busy = false
	cmd := m.openRepository(target.repo.Root)
	if cmd == nil {
		t.Fatal("clean editor did not start repository discovery")
	}
	doc.Insert("edited during discovery")
	drain(t, m, cmd)
	if m.NextRepository() != nil || m.document() != doc || !doc.Dirty() {
		t.Fatal("asynchronous discovery lost unsaved changes")
	}
}

func TestFailedAndCurrentFolderKeepRepository(t *testing.T) {
	m := explorerWorkspace(t)
	old := m.repo
	drain(t, m, m.openRepository(filepath.Join(t.TempDir(), "missing")))
	if m.repo != old || m.NextRepository() != nil || m.openingRepository {
		t.Fatal("failed switch changed repository or stayed pending")
	}
	drain(t, m, m.openRepository(old.Root))
	if m.NextRepository() != nil || !strings.Contains(m.notice, "已在此仓库") {
		t.Fatal("current repository restarted")
	}
}
