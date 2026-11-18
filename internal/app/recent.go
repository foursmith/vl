package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/config"
	gitservice "github.com/ourongxing/vl/internal/git"
)

type repositoryOpenedMsg struct {
	repo *gitservice.Repository
	err  error
}

func (m *Model) openRecentWorkspaces() tea.Cmd {
	prompt := m.prompt("recent", m.tr("Open Recent Workspace"), "")
	m.input.Placeholder = m.tr("Search recent workspaces")
	m.populateRecentWorkspaces()
	return tea.Batch(prompt, m.loadWorkspaces())
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
	if m.busy || m.openingRepository {
		m.notice = m.tr("Wait for the current operation to finish")
		return true
	}
	if m.hasDirtyTabs() {
		m.notice = m.tr("Save or close unsaved files first")
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
	// Editing or a Git operation may have started while discovery was running.
	if m.repositorySwitchBlocked() {
		return nil
	}
	if msg.repo.Root == m.repo.Root {
		m.notice = m.tr("Already in this repository")
		return nil
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
