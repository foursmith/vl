package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/config"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

type recentFoldersMsg struct {
	seq     int
	folders []string
	err     error
}

type repositoryOpenedMsg struct {
	repo *gitservice.Repository
	err  error
}

func (m *Model) openRecentFolders() tea.Cmd {
	m.palette = nil
	m.recentSeq++
	seq, ctx := m.recentSeq, m.ctx
	prompt := m.prompt("recent", "最近打开的文件夹", "")
	m.input.Placeholder = "搜索最近打开的文件夹"
	return tea.Batch(prompt, func() tea.Msg {
		folders, err := config.RecentFolders()
		if ctx.Err() != nil {
			return nil
		}
		return recentFoldersMsg{seq: seq, folders: folders, err: err}
	})
}

func (m *Model) applyRecentFolders(msg recentFoldersMsg) {
	if msg.seq != m.recentSeq || m.modal != "recent" {
		return
	}
	if msg.err != nil {
		m.modal = ""
		m.input.Blur()
		m.log("读取最近文件夹失败: " + msg.err.Error())
		return
	}
	m.palette = []action{}
	for _, folder := range msg.folders {
		m.palette = append(m.palette, action{
			id: folder, label: filepath.Base(folder) + "  " + folder,
			run: func() tea.Cmd { return m.openRepository(folder) },
		})
	}
}

func (m *Model) repositorySwitchBlocked() bool {
	if m.busy || m.openingRepository {
		m.notice = "请等待当前操作完成"
		return true
	}
	if m.hasDirtyTabs() {
		m.notice = "先保存或关闭未保存的文件"
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
		m.log("打开文件夹失败: " + msg.err.Error())
		return nil
	}
	// Editing or a Git operation may have started while discovery was running.
	if m.repositorySwitchBlocked() {
		return nil
	}
	if msg.repo.Root == m.repo.Root {
		m.notice = "已在此仓库"
		return nil
	}
	m.nextRepository = msg.repo
	return tea.Quit
}

// NextRepository is set only after successful discovery and unsaved-file checks.
// The runner closes this model's background work before opening a fresh model.
func (m *Model) NextRepository() *gitservice.Repository { return m.nextRepository }
