package app

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

type browserMsg struct {
	target string
	err    error
}

func (m *Model) openBranchPicker(repoPath, mode string) tea.Cmd {
	m.branchRepository = repoPath
	suffix := ""
	if repoPath != "" {
		suffix = " · " + m.repositoryName(repoPath)
	}
	m.palette = nil
	if mode == "" {
		m.palette = append(m.palette,
			action{id: "branch.create", label: m.symbol(symbolAdd) + " 创建新分支…", run: func() tea.Cmd {
				m.branchStart = ""
				m.branchRepository = repoPath
				return m.prompt("branch", "新分支名称"+suffix, "")
			}},
			action{id: "branch.createFrom", label: m.symbol(symbolAdd) + " 创建新分支依据…", run: func() tea.Cmd { return m.openBranchPicker(repoPath, "base") }},
			action{id: "branch.detach", label: "签出已分离…", run: func() tea.Cmd { return m.openBranchPicker(repoPath, "detach") }})
	}
	for _, ref := range m.historyData(repoPath).Refs {
		kind, glyph := "分支", m.symbol(symbolBranch)
		if ref.Kind == gitservice.RemoteBranch {
			kind, glyph = "远程分支", m.symbol(symbolCloud)
		}
		if ref.Kind == gitservice.Tag {
			kind, glyph = "标签", m.symbol(symbolTag)
		}
		mark := ""
		if ref.Current {
			mark = " " + m.symbol(symbolCheck)
		}
		label := glyph + " " + ref.Name + "  " + shortOID(ref.OID) + mark
		disabled := ""
		if m.hasDirtyTabsForCheckout(repoPath) && mode != "base" && (!ref.Current || mode == "detach") {
			disabled = "先保存或关闭未保存的文件"
		}
		m.palette = append(m.palette, action{id: ref.FullName, label: label, key: kind, disabled: disabled, run: func() tea.Cmd {
			if mode == "base" {
				m.branchStart = ref.OID
				m.branchRepository = repoPath
				return m.prompt("branch", "新分支名称 · 基于 "+ref.Name+suffix, "")
			}
			return m.checkout(repoPath, ref, mode == "detach")
		}})
	}
	title := "选择要签出的分支或标记"
	if mode == "base" {
		title = "选择新分支的起点"
	}
	if mode == "detach" {
		title = "选择以分离 HEAD 签出的引用"
	}
	title += suffix
	cmd := m.prompt("refs", title, "")
	m.input.Placeholder = title + " · 输入名称或提交号"
	return cmd
}

func shortOID(oid string) string { return oid[:min(7, len(oid))] }

func (m *Model) checkout(repoPath string, ref gitservice.Ref, detach bool) tea.Cmd {
	if ref.Current && !detach {
		m.notice = "已在分支 " + ref.Name
		return nil
	}
	if m.hasDirtyTabsForCheckout(repoPath) {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	if detach {
		ref.Kind = gitservice.Tag
	}
	return m.operation("Switch branch", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, repoPath)
		if err != nil {
			return "", err
		}
		return repo.Switch(m.ctx, ref)
	})
}

func (m *Model) syncRepository() tea.Cmd {
	target := m.statusRepository()
	state := m.repositoryState(target)
	if state.Detached {
		m.notice = "分离 HEAD 状态下不能同步分支"
		return nil
	}
	if m.hasDirtyTabs() {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	if state.Upstream == "" {
		return m.publishBranchInRepository(target)
	}
	return m.operation("Sync", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.Sync(m.ctx)
	})
}

func (m *Model) publishBranch() tea.Cmd {
	return m.publishBranchInRepository(m.statusRepository())
}

func (m *Model) publishBranchInRepository(target string) tea.Cmd {
	state := m.repositoryState(target)
	if state.Detached || state.Head == "" {
		m.notice = "先创建分支并完成首次提交"
		return nil
	}
	if len(state.Remotes) == 0 {
		m.notice = "没有配置远程仓库"
		return nil
	}
	m.palette = nil
	branch := state.Branch
	for _, remote := range state.Remotes {
		m.palette = append(m.palette, action{id: remote.Name, label: remote.Name, key: "发布分支", run: func() tea.Cmd {
			if m.hasDirtyTabs() {
				m.notice = "先保存或关闭未保存的文件"
				return nil
			}
			return m.operation("Publish", func() (string, error) {
				repo, err := m.repo.RepositoryAt(m.ctx, target)
				if err != nil {
					return "", err
				}
				return repo.Publish(m.ctx, remote.Name, branch)
			})
		}})
	}
	return m.prompt("palette", "选择发布分支的远程仓库", "")
}

func (m *Model) openGitHub() tea.Cmd {
	target := m.repositoryState(m.statusRepository()).GitHubURL
	if target == "" {
		m.notice = "没有找到 github.com 远程仓库"
		return nil
	}
	open := m.openURL
	return func() tea.Msg { return browserMsg{target: target, err: open(m.ctx, target)} }
}

func (m *Model) createBranch(name string) tea.Cmd {
	target := m.branchRepository
	if m.hasDirtyTabsForCheckout(target) {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	start := m.branchStart
	m.branchStart = ""
	return m.operation("Create branch", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.CreateBranch(m.ctx, name, start)
	})
}

func (m *Model) goToPosition(value string) tea.Cmd {
	d := m.document()
	if d == nil {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) > 2 {
		m.notice = "输入行号或 行:列"
		return nil
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		m.notice = "行号必须大于 0"
		return nil
	}
	column := 1
	if len(parts) == 2 {
		column, err = strconv.Atoi(parts[1])
		if err != nil || column < 1 {
			m.notice = "列号必须大于 0"
			return nil
		}
	}
	d.SetPosition(line-1, column-1)
	m.sidebar = false
	m.ensureEditor()
	return nil
}

// Switching a repository may also update nested worktrees through Git's recurse
// setting, so protect dirty descendants while allowing unrelated parent edits.
func (m *Model) hasDirtyTabsForCheckout(repoPath string) bool {
	if repoPath == "" {
		return m.hasDirtyTabs()
	}
	root := filepath.Join(m.repo.Root, repoPath)
	for _, t := range m.tabs {
		if t.doc == nil || !t.doc.Dirty() {
			continue
		}
		relative, err := filepath.Rel(root, t.doc.Path)
		if err == nil && filepath.IsLocal(relative) {
			return true
		}
	}
	return false
}
