package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

// Scope membership comes from the full snapshot, including collapsed descendants.
func (m *Model) scopedChanges(entry item) []gitservice.Change {
	if entry.change != nil {
		return []gitservice.Change{*entry.change}
	}
	var result []gitservice.Change
	for _, c := range m.snapshot.Changes {
		if entry.staged && !c.Staged() || !entry.staged && !c.Unstaged() {
			continue
		}
		if entry.folder != "" && !strings.HasPrefix(c.Path, entry.folder+"/") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (m *Model) changeActionDisabled(entry item, id string, scope []gitservice.Change) string {
	if m.busy {
		return "等待当前 Git 操作完成"
	}
	if len(scope) == 0 {
		return "此范围没有更改"
	}
	switch id {
	case "open":
		if c := entry.change; c != nil && (c.Worktree == 'D' || c.Index == 'D' && c.Worktree == ' ') {
			return "文件已删除"
		}
	case "discard":
		for _, c := range scope {
			if c.Conflicted() {
				return "先解决文件冲突"
			}
		}
		if m.hasDirtyTabs() {
			return "先保存或关闭未保存的文件"
		}
	case "stage":
		for _, c := range scope {
			if m.fileDirty(c.Path) {
				return "先保存此范围的文件"
			}
		}
	}
	return ""
}

func (m *Model) runScopeAction(entry item, id string) tea.Cmd {
	scope := m.scopedChanges(entry)
	if len(scope) == 0 {
		return nil
	}
	if id == "open" {
		for _, c := range scope {
			if c.Worktree != 'D' && !(c.Index == 'D' && c.Worktree == ' ') {
				m.openFile(c.Path)
			}
		}
		return nil
	}
	run := func() tea.Cmd {
		// Recheck editor state when a confirmation is accepted.
		if reason := m.changeActionDisabled(entry, id, scope); reason != "" {
			m.notice = reason
			return nil
		}
		title := map[string]string{"stage": "Stage", "unstage": "Unstage", "discard": "Discard"}[id]
		return m.operation(title, func() (string, error) {
			for i, c := range scope {
				var err error
				switch id {
				case "stage":
					_, err = m.repo.Stage(m.ctx, c)
				case "unstage":
					_, err = m.repo.Unstage(m.ctx, c)
				case "discard":
					_, err = m.repo.Discard(m.ctx, c)
				}
				if err != nil {
					return "", fmt.Errorf("已完成 %d/%d，%s: %w", i, len(scope), c.Path, err)
				}
			}
			return fmt.Sprintf("已处理 %d 个文件", len(scope)), nil
		})
	}
	if id != "discard" {
		return run()
	}
	name := entry.folder
	if name == "" {
		name = "更改分组"
	}
	cmd := m.ask(fmt.Sprintf("丢弃 %s 中的 %d 个文件更改？", name, len(scope)), run)
	m.confirmationDetail = "仅处理当前范围；未跟踪文件将被删除，Git 无法恢复。"
	return cmd
}
