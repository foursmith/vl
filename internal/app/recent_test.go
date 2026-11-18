package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/config"
)

func TestRecentWorkspacesDoNotDisplaceSidebarNavigation(t *testing.T) {
	for _, right := range []bool{false, true} {
		m := newHoverRenderModel(t)
		m.SetSidebarLayout(38, right)
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
		before := m.sidebarLines()[0]
		m.Update(workspacesMsg{seq: m.workspaceSeq, folders: []string{m.repo.Root, "/projects/other"}})
		if m.sidebarLines()[0] != before {
			t.Fatal("recent workspaces displaced the sidebar navigation")
		}
		m.click(tea.MouseClickMsg{X: m.sideX() + 3*sidebarNavigationWidth, Y: 0, Button: tea.MouseLeft})
		if m.section != history {
			t.Fatal("navigation is not clickable at the top of the sidebar")
		}
		m.Close()
	}
}

func TestRecentWorkspacesPickerSearchAndOpen(t *testing.T) {
	m := explorerWorkspace(t)
	target := explorerWorkspace(t)
	old := m.repo
	press(m, "ctrl+h")
	if m.modal != "recent" || !m.isPicker() {
		t.Fatal("Ctrl+H did not open recent workspace picker")
	}
	m.Update(workspacesMsg{seq: m.workspaceSeq, folders: []string{m.repo.Root, target.repo.Root}})
	m.input.SetValue(target.repo.Root)
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != target.repo.Root {
		t.Fatalf("folder search: %+v", rows)
	}
	drain(t, m, press(m, "enter"))
	if m.nextRepository == nil || m.nextRepository.Root != target.repo.Root {
		t.Fatal("selected repository was not handed to runner")
	}
	if m.repo != old {
		t.Fatal("old model was mutated instead of replaced")
	}
}

func TestRecentWorkspacesSearchNameAndPath(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.workspaces = []string{"/Projects/Client/alpha", "/Projects/Personal/beta"}
	press(m, "ctrl+h")
	for query, want := range map[string]string{
		"alpha":                   "/Projects/Client/alpha",
		"client":                  "/Projects/Client/alpha",
		"/projects/personal/":     "/Projects/Personal/beta",
		"/Projects/Personal/beta": "/Projects/Personal/beta",
	} {
		m.input.SetValue(query)
		rows := m.filteredActions()
		if len(rows) != 1 || rows[0].id != want {
			t.Fatalf("search %q: got %+v, want %q", query, rows, want)
		}
	}
}

func TestRecentWorkspaceResultsRespectDismissalAndNewRequests(t *testing.T) {
	m := explorerWorkspace(t)
	m.openRecentWorkspaces()
	stale := workspacesMsg{seq: m.workspaceSeq, folders: []string{m.repo.Root}}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(stale)
	if m.modal != "" || len(m.palette) != 0 {
		t.Fatal("dismissed picker was reopened")
	}
	m.openRecentWorkspaces()
	if len(m.palette) != 1 || m.palette[0].id != m.repo.Root {
		t.Fatal("new picker did not reuse the workspace list")
	}
	stale.folders = []string{"stale-folder"}
	m.Update(stale)
	if len(m.palette) != 1 || m.palette[0].id != m.repo.Root || len(m.workspaces) != 1 || m.workspaces[0] != m.repo.Root {
		t.Fatal("stale request replaced the workspace list or picker")
	}
	m.Update(workspacesMsg{seq: m.workspaceSeq, err: errors.New("unreadable history")})
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
	if m.nextRepository != nil || m.document() != doc || !doc.Dirty() {
		t.Fatal("asynchronous discovery lost unsaved changes")
	}
}

func TestFailedAndCurrentFolderKeepRepository(t *testing.T) {
	m := explorerWorkspace(t)
	old := m.repo
	drain(t, m, m.openRepository(filepath.Join(t.TempDir(), "missing")))
	if m.repo != old || m.nextRepository != nil || m.openingRepository {
		t.Fatal("failed switch changed repository or stayed pending")
	}
	drain(t, m, m.openRepository(old.Root))
	if m.nextRepository != nil || !strings.Contains(m.notice, "Already in this repository") {
		t.Fatal("current repository restarted")
	}
}

func TestRecentPickerContextMenuRemovesFilteredWorkspace(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := explorerWorkspace(t)
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	other := t.TempDir()
	folder := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{folder, other} {
		if err := config.RememberFolder(path); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.openRecentWorkspaces())
	m.input.SetValue("target")
	m.syncPicker()
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 3, Y: l.y + 3, Button: tea.MouseRight})
	if m.contextMenu == nil || m.modal != "recent" {
		t.Fatal("right click did not open a menu above the recent picker")
	}
	m.View()
	drain(t, m, chooseContextAction(t, m, "Remove from workspace list"))
	if m.contextMenu != nil || m.modal != "recent" || m.input.Value() != "target" || len(m.filteredActions()) != 0 {
		t.Fatal("removal did not refresh the picker while preserving its search")
	}
	folders, err := config.RecentFolders()
	if err != nil || len(folders) != 1 || folders[0] != other {
		t.Fatalf("persisted workspaces: %v, %v", folders, err)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Fatalf("workspace directory changed: %v", err)
	}
	if m.nextRepository != nil {
		t.Fatal("right click switched workspace")
	}
}
