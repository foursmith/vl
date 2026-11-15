package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

type browserMsg struct {
	target string
	err    error
}

func (m *Model) isPicker() bool { return m.modal == "palette" || m.modal == "refs" }

func (m *Model) openBranchPicker(mode string) tea.Cmd {
	m.palette = nil
	if mode == "" {
		m.palette = append(m.palette,
			action{id: "branch.create", label: m.symbol(symbolAdd) + " 创建新分支…", run: func() tea.Cmd { m.branchStart = ""; return m.prompt("branch", "新分支名称", "") }},
			action{id: "branch.createFrom", label: m.symbol(symbolAdd) + " 创建新分支依据…", run: func() tea.Cmd { return m.openBranchPicker("base") }},
			action{id: "branch.detach", label: "签出已分离…", run: func() tea.Cmd { return m.openBranchPicker("detach") }})
	}
	for _, ref := range m.snapshot.Refs {
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
		if m.hasDirtyTabs() && mode != "base" && (!ref.Current || mode == "detach") {
			disabled = "先保存或关闭未保存的文件"
		}
		m.palette = append(m.palette, action{id: ref.FullName, label: label, key: kind, disabled: disabled, run: func() tea.Cmd {
			if mode == "base" {
				m.branchStart = ref.OID
				return m.prompt("branch", "新分支名称 · 基于 "+ref.Name, "")
			}
			return m.checkout(ref, mode == "detach")
		}})
	}
	title := "选择要签出的分支或标记"
	if mode == "base" {
		title = "选择新分支的起点"
	}
	if mode == "detach" {
		title = "选择以分离 HEAD 签出的引用"
	}
	cmd := m.prompt("refs", title, "")
	m.input.Placeholder = "输入分支、标签名称或提交号"
	return cmd
}

func shortOID(oid string) string { return oid[:min(7, len(oid))] }

func (m *Model) checkout(ref gitservice.Ref, detach bool) tea.Cmd {
	if ref.Current && !detach {
		m.notice = "已在分支 " + ref.Name
		return nil
	}
	if m.hasDirtyTabs() {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	if detach {
		ref.Kind = gitservice.Tag
	}
	return m.operation("Switch branch", func() (string, error) { return m.repo.Switch(m.ctx, ref) })
}

func (m *Model) syncRepository() tea.Cmd {
	if m.snapshot.Detached {
		m.notice = "分离 HEAD 状态下不能同步分支"
		return nil
	}
	if m.hasDirtyTabs() {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	if m.snapshot.Upstream == "" {
		return m.publishBranch()
	}
	return m.operation("Sync", func() (string, error) { return m.repo.Sync(m.ctx) })
}

func (m *Model) publishBranch() tea.Cmd {
	if m.snapshot.Detached || m.snapshot.Head == "" {
		m.notice = "先创建分支并完成首次提交"
		return nil
	}
	if len(m.snapshot.Remotes) == 0 {
		m.notice = "没有配置远程仓库"
		return nil
	}
	m.palette = nil
	branch := m.snapshot.Branch
	for _, remote := range m.snapshot.Remotes {
		m.palette = append(m.palette, action{id: remote.Name, label: remote.Name, key: "发布分支", run: func() tea.Cmd {
			if m.hasDirtyTabs() {
				m.notice = "先保存或关闭未保存的文件"
				return nil
			}
			return m.operation("Publish", func() (string, error) { return m.repo.Publish(m.ctx, remote.Name, branch) })
		}})
	}
	return m.prompt("palette", "选择发布分支的远程仓库", "")
}

func (m *Model) openGitHub() tea.Cmd {
	target := m.snapshot.GitHubURL
	if target == "" {
		m.notice = "没有找到 github.com 远程仓库"
		return nil
	}
	open := m.openURL
	return func() tea.Msg { return browserMsg{target: target, err: open(m.ctx, target)} }
}

func (m *Model) createBranch(name string) tea.Cmd {
	if m.hasDirtyTabs() {
		m.notice = "先保存或关闭未保存的文件"
		return nil
	}
	start := m.branchStart
	m.branchStart = ""
	return m.operation("Create branch", func() (string, error) { return m.repo.CreateBranch(m.ctx, name, start) })
}

func (m *Model) selectPickerChoice() tea.Cmd {
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[min(m.choice, len(rows)-1)]
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	m.modal = ""
	m.input.Blur()
	return a.run()
}

func (m *Model) goToPosition(value string) tea.Cmd {
	d := m.document()
	if !m.showEditor || d == nil {
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
