package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	gitservice "github.com/foursmith/vl/internal/git"
)

type repositoryOpenedMsg struct {
	repo *gitservice.Repository
	err  error
}

func (m *Model) openRecentWorkspaces() tea.Cmd {
	m.prompt("recent", m.tr("Open Recent Workspace"), "")
	m.input.Placeholder = m.tr("Search recent workspaces")
	m.populateRecentWorkspaces()
	return m.loadWorkspaces()
}

func (m *Model) populateRecentWorkspaces() {
	m.palette = []action{}
	for _, folder := range m.workspaces {
		m.palette = append(m.palette, action{
			id: folder, label: filepath.Base(folder),
			run: func() tea.Cmd { return m.openRepository(folder) },
		})
	}
	m.syncPicker()
}

func (m *Model) repositorySwitchBlocked() bool {
	if m.busy || m.openingRepository || m.modal != "" || m.flash != nil {
		m.notice = m.tr("Wait for the current operation to finish")
		return true
	}
	return false
}

func (m *Model) openRepository(path string) tea.Cmd {
	if m.repositorySwitchBlocked() {
		return nil
	}
	m.openingRepository = true
	ctx := m.ctx
	return func() tea.Msg {
		repo, err := gitservice.Open(ctx, path)
		return repositoryOpenedMsg{repo: repo, err: err}
	}
}

func (m *Model) finishOpenRepository(msg repositoryOpenedMsg) tea.Cmd {
	m.openingRepository = false
	if msg.err != nil {
		m.notice = m.tr("Failed to open folder: %s", m.tr(msg.err.Error()))
		return nil
	}
	// Discovery can finish after the user starts editing or opens another dialog.
	if m.repositorySwitchBlocked() {
		return nil
	}
	if msg.repo.Root == m.repo.Root {
		m.showStatusFeedback(m.tr("Already in this repository"))
		return nil
	}
	if m.hasDirtyTabs() {
		return m.confirmLeavingWorkspace(m.tr("Save changes before switching workspaces?"), func() tea.Cmd {
			m.nextRepository = msg.repo
			return nil
		})
	}
	m.nextRepository = msg.repo
	return nil
}

type workspacesMsg struct {
	seq     int
	folders []string
	err     error
}

func (m *Model) loadWorkspaces() tea.Cmd {
	if m.removingWorkspace {
		return nil
	}
	m.workspaceSeq++
	seq, ctx := m.workspaceSeq, m.ctx
	return func() tea.Msg {
		folders, err := config.RecentFolders()
		if ctx.Err() != nil {
			return nil
		}
		return workspacesMsg{seq: seq, folders: folders, err: err}
	}
}

func (m *Model) removeWorkspace(folder string) tea.Cmd {
	if m.removingWorkspace {
		return nil
	}
	m.removingWorkspace = true
	m.workspaceSeq++
	seq, ctx := m.workspaceSeq, m.ctx
	return func() tea.Msg {
		folders, err := config.ForgetFolder(folder)
		if ctx.Err() != nil {
			return nil
		}
		return workspacesMsg{seq: seq, folders: folders, err: err}
	}
}
