package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSidebarWorkspaceSwitch(t *testing.T) {
	for _, width := range []int{30, 60, 120} {
		for _, right := range []bool{false, true} {
			for _, section := range []section{explorer, changes, history, search} {
				t.Run(fmt.Sprintf("width%d/right%v/section%d", width, right, section), func(t *testing.T) {
					m := newHoverRenderModel(t)
					m.section, m.sidebar = section, true
					m.SetSidebarLayout(minSidebarWidth, right)
					m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
					w := m.sideWidth()
					if !m.sideShown() {
						w = m.width
					}
					target := explorerWorkspace(t)
					m.Update(workspacesMsg{folders: []string{m.repo.Root, target.repo.Root}})
					lines := m.sidebarLinesWidth(w)
					if !strings.Contains(ansi.Strip(lines[0]), "WORKSPACES") || !strings.Contains(ansi.Strip(lines[2]), filepath.Base(target.repo.Root)) {
						t.Fatal("recent workspaces are not listed above the section")
					}
					for _, line := range lines {
						if ansi.StringWidth(line) != w {
							t.Fatal("workspace sidebar overflow")
						}
					}
					x := m.sideX() + 3
					m.View()
					m.Update(tea.MouseMotionMsg{X: x, Y: 2})
					assertFreshHoverFrame(t, m)
					if !strings.Contains(m.hoverHint(), target.repo.Root) {
						t.Fatal("workspace path hint missing")
					}
					drain(t, m, m.click(tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseLeft}))
					if m.modal != "" || m.NextRepository() == nil || m.NextRepository().Root != target.repo.Root {
						t.Fatal("click did not switch directly to the workspace")
					}
				})
			}
		}
	}
}

func TestWorkspaceListScrollAndSectionCoordinates(t *testing.T) {
	m := newHoverRenderModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 15})
	var folders []string
	for i := 0; i < 20; i++ {
		folders = append(folders, filepath.Join(t.TempDir(), fmt.Sprintf("workspace%d", i)))
	}
	m.Update(workspacesMsg{folders: folders})
	for range 30 {
		m.Update(tea.MouseWheelMsg{X: m.sideX() + 3, Y: 2, Button: tea.MouseWheelDown})
	}
	if m.workspaceTop+m.workspaceRows()-1 != len(folders) {
		t.Fatal("last recent workspace cannot be reached")
	}
	if lines := m.sidebarLines(); !strings.Contains(ansi.Strip(lines[m.workspaceRows()-1]), "workspace19") {
		t.Fatal("last workspace not rendered after scrolling")
	}
	rootY := sidebarRootRow + m.workspaceRows()
	m.click(tea.MouseClickMsg{X: m.sideX() + 3, Y: rootY, Button: tea.MouseLeft})
	if !m.rootCollapsed {
		t.Fatal("workspace list displaced the root click target")
	}
	m.click(tea.MouseClickMsg{X: m.sideX() + 14, Y: m.workspaceRows(), Button: tea.MouseLeft})
	if m.section != search {
		t.Fatal("navigation tabs are not clickable below workspaces")
	}
	m.click(tea.MouseClickMsg{X: m.sideX() + 3, Y: 1 + m.workspaceRows(), Button: tea.MouseLeft})
	if !m.search.replace {
		t.Fatal("workspace list displaced search controls")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	if m.workspaceTop != 0 {
		t.Fatal("resizing did not restore the complete workspace list")
	}
}

func TestWorkspaceLabelsOmitPathsUnlessNamesCollide(t *testing.T) {
	m := newHoverRenderModel(t)
	m.workspaces = []string{"/projects/newsnext", "/work/elegit", "/personal/elegit"}
	for folder, want := range map[string]string{
		"/projects/newsnext": "newsnext",
		"/work/elegit":       "elegit · work",
		"/personal/elegit":   "elegit · personal",
	} {
		if got := m.workspaceLabel(folder); got != want {
			t.Fatalf("workspace label: got %q, want %q", got, want)
		}
	}
}

func TestRecentFoldersPickerSearchAndOpen(t *testing.T) {
	m := explorerWorkspace(t)
	target := explorerWorkspace(t)
	old := m.repo
	press(m, "ctrl+r")
	if m.modal != "recent" || !m.isPicker() {
		t.Fatal("Ctrl+R did not open recent folder picker")
	}
	m.Update(workspacesMsg{seq: m.workspaceSeq, folders: []string{m.repo.Root, target.repo.Root}})
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
	stale := workspacesMsg{seq: m.workspaceSeq, folders: []string{m.repo.Root}}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(stale)
	if m.modal != "" || len(m.palette) != 0 {
		t.Fatal("dismissed picker was reopened")
	}
	m.openRecentFolders()
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
