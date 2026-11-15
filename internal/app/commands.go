package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type action struct {
	id, label, key string
	disabled       string
	run            func() tea.Cmd
}

func (m *Model) actions() []action {
	fileReason := ""
	if m.current() == nil || m.current().change == nil {
		fileReason = "Select a file in Source Control"
	}
	docReason := ""
	tabReason := ""
	if !m.diffOpen && (!m.showEditor || m.document() == nil) {
		tabReason = "No active tab"
	}
	openReason := ""
	if m.current() == nil || m.current().change == nil && m.current().file == "" {
		openReason = "Select a file in Explorer or Source Control"
	}
	if !m.showEditor || m.document() == nil {
		docReason = "Open a text file first"
	}
	githubReason := ""
	if m.snapshot.GitHubURL == "" {
		githubReason = "没有找到 github.com 远程仓库"
	}
	return []action{
		{"folder.openRecent", "File: Open Recent Folder", "Ctrl+R", "", m.openRecentFolders},
		{"view.explorer", "View: Explorer", "Ctrl+1", "", func() tea.Cmd { return m.selectSection(explorer) }},
		{"explorer.newFile", "Explorer: New File", "", "", func() tea.Cmd { return m.newEntry(false) }},
		{"explorer.newFolder", "Explorer: New Folder", "", "", func() tea.Cmd { return m.newEntry(true) }},
		{"explorer.collapseAll", "Explorer: Collapse Folders", "", "", m.collapseFolders},
		{"view.changes", "View: Source Control", "Ctrl+2", "", func() tea.Cmd { return m.selectSection(changes) }},
		{"view.history", "View: Commit History", "Ctrl+3", "", func() tea.Cmd { return m.selectSection(history) }},
		{"view.diff", "Diff: Toggle Side by Side", "Ctrl+D", "", func() tea.Cmd { m.split = !m.split; m.renderDiff(); return nil }},
		{"view.sidebarLeft", "View: Move Side Bar Left", "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, false); return nil }},
		{"view.sidebarRight", "View: Move Side Bar Right", "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, true); return nil }},
		{"view.sidebarResetWidth", "View: Reset Side Bar Width", "", "", func() tea.Cmd { m.SetSidebarLayout(0, m.sidebarRight); return nil }},
		{"view.output", "View: Toggle Output", "Ctrl+J", "", func() tea.Cmd { m.outputVisible = !m.outputVisible; m.resize(); return nil }},
		{"git.refresh", "Git: Refresh", "F5", "", m.refresh},
		{"git.stage", "Git: Stage Selected File", "S", fileReason, func() tea.Cmd { return m.stageSelected(false) }},
		{"git.unstage", "Git: Unstage Selected File", "U", fileReason, func() tea.Cmd { return m.stageSelected(true) }},
		{"git.stageAll", "Git: Stage All Changes", "A", "", m.stageAll},
		{"git.commit", "Git: Commit Staged Changes", "C", "", m.commit},
		{"git.fetch", "Git: Fetch", "", "", func() tea.Cmd { return m.gitOperation("Fetch", "fetch", "--all", "--prune") }},
		{"git.pull", "Git: Pull (Fast Forward Only)", "", "", func() tea.Cmd { return m.gitOperation("Pull", "pull", "--ff-only") }},
		{"git.push", "Git: Push", "", "", func() tea.Cmd { return m.gitOperation("Push", "push") }},
		{"git.branch", "Git: Create Branch", "", "", func() tea.Cmd { m.branchStart = ""; return m.prompt("branch", "Create and switch to branch", "") }},
		{"git.checkout", "Git: Checkout Branch or Tag", "", "", func() tea.Cmd { return m.openBranchPicker("") }},
		{"git.sync", "Git: Sync with Upstream", "", "", m.syncRepository},
		{"git.publish", "Git: Publish Branch", "", "", m.publishBranch},
		{"github.open", "GitHub: Open Repository", "", githubReason, m.openGitHub},
		{"git.stash", "Git: Stash Changes (Including Untracked)", "", "", func() tea.Cmd { return m.prompt("stash", "Stash message", "") }},
		{"git.stashPop", "Git: Pop Latest Stash", "", "", func() tea.Cmd {
			return m.ask("Apply and remove the latest stash?", func() tea.Cmd { return m.gitOperation("Stash pop", "stash", "pop") })
		}},
		{"git.discard", "Git: Discard Unstaged Changes", "", fileReason, m.discard},
		{"file.open", "File: Open Selected File", "E", openReason, m.editSelected},
		{"file.save", "File: Save", "Ctrl+S", docReason, m.save},
		{"file.close", "File: Close Active Tab", "Ctrl+W", tabReason, m.closeTab},
		{"file.keepOpen", "File: Keep Preview Open", "", docReason, m.keepTab},
		{"file.closeOthers", "File: Close Other Tabs", "", "", m.closeOtherTabs},
		{"file.closeAll", "File: Close All Tabs", "", "", m.closeAllTabs},
		{"edit.find", "Find: Search Current View", "Ctrl+F", "", func() tea.Cmd { return m.prompt("find", "Find in current view", m.find) }},
		{"app.quit", "Quit", "Ctrl+Q", "", m.quit},
	}
}

func (m *Model) openPalette() tea.Cmd {
	m.palette = m.actions()
	return m.prompt("palette", "Commands", "")
}

func (m *Model) filteredActions() []action {
	query := strings.ToLower(m.input.Value())
	if query == "" {
		return m.palette
	}
	var result []action
	for _, a := range m.palette {
		if strings.Contains(strings.ToLower(a.label), query) || a.id != a.label && strings.Contains(strings.ToLower(a.id), query) {
			result = append(result, a)
		}
	}
	return result
}

func (m *Model) prompt(kind, title, value string) tea.Cmd {
	m.resizingSidebar = false
	m.modal, m.modalTitle = kind, title
	m.input.SetValue(value)
	m.input.Placeholder = ""
	m.input.Prompt = "> "
	m.resizeInput()
	if m.isPicker() && title != "Commands" {
		m.input.Placeholder = title
	}
	if kind == "files" {
		m.input.Prompt = ""
		m.input.Placeholder = "按名称搜索文件"
	} else if kind == "refs" {
		m.input.Prompt = ""
	}
	m.input.CursorEnd()
	m.choice = 0
	return m.input.Focus()
}

func (m *Model) resizeInput() {
	width := max(10, min(66, m.width-14))
	if m.isPicker() {
		width = max(1, min(96, m.width-4)-4)
	}
	m.input.SetWidth(width)
}

func (m *Model) ask(title string, run func() tea.Cmd) tea.Cmd {
	m.confirm = run
	m.confirmationDetail = ""
	return m.prompt("confirm", title, "")
}

func (m *Model) updateModal(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if m.isPicker() {
		switch k {
		case "ctrl+p":
			return m.openQuickFiles()
		case "ctrl+shift+p", "f1":
			return m.openPalette()
		}
	}
	if m.modal == "close" {
		switch k {
		case "s", "enter":
			return m.chooseClose("save")
		case "d":
			return m.chooseClose("discard")
		case "esc":
			return m.chooseClose("cancel")
		}
		return nil
	}
	if k == "esc" {
		m.modal = ""
		m.confirm = nil
		m.input.Blur()
		return nil
	}
	if m.isPicker() {
		rows := m.filteredActions()
		switch k {
		case "up":
			m.choice = max(0, m.choice-1)
			return nil
		case "down":
			m.choice = min(max(0, len(rows)-1), m.choice+1)
			return nil
		case "enter":
			return m.selectPickerChoice()
		}
	}
	if k == "enter" && !m.isPicker() {
		kind, value := m.modal, strings.TrimSpace(m.input.Value())
		m.modal = ""
		m.input.Blur()
		switch kind {
		case "confirm":
			run := m.confirm
			m.confirm = nil
			if run != nil {
				return run()
			}
		case "commit":
			if value != "" {
				return m.gitOperation("Commit", "commit", "-m", value)
			}
		case "branch":
			if value != "" {
				return m.createBranch(value)
			}
		case "stash":
			return m.gitOperation("Stash", "stash", "push", "--include-untracked", "-m", value)
		case "find":
			m.find = value
			return m.findNext()
		case "newFile":
			return m.createEntry(value, false)
		case "newFolder":
			return m.createEntry(value, true)
		case "position":
			return m.goToPosition(value)
		}
		return nil
	}
	if m.modal == "confirm" {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.choice = 0
	return cmd
}

func (m *Model) operation(title string, run func() (string, error)) tea.Cmd {
	if m.busy {
		m.notice = "Wait for the current Git operation"
		return nil
	}
	m.busy = true
	m.notice = title + "…"
	m.refreshSeq++
	if m.refreshCancel != nil {
		m.refreshCancel()
		m.refreshCancel = nil
	}
	m.loading = false
	m.diffSeq++
	if m.diffCancel != nil {
		m.diffCancel()
		m.diffCancel = nil
	}
	m.diffLoading = false
	return func() tea.Msg { output, err := run(); return operationMsg{title, output, err} }
}

func (m *Model) gitOperation(title string, args ...string) tea.Cmd {
	if (title == "Pull" || title == "Stash" || title == "Stash pop") && m.hasDirtyTabs() {
		m.notice = "Save or close edited files before this operation"
		return nil
	}
	return m.operation(title, func() (string, error) { return m.repo.Run(m.ctx, args...) })
}

func (m *Model) stageSelected(unstage bool) tea.Cmd {
	entry := m.current()
	if entry == nil || entry.change == nil {
		m.notice = "Select a changed file first"
		return nil
	}
	c := *entry.change
	if !unstage && m.fileDirty(c.Path) {
		m.notice = "Save this file before staging"
		return nil
	}
	if unstage {
		if !c.Staged() {
			m.notice = "This file has no staged changes"
			return nil
		}
		return m.operation("Unstage "+c.Path, func() (string, error) { return m.repo.Unstage(m.ctx, c) })
	}
	if !c.Unstaged() {
		m.notice = "This file has no unstaged changes"
		return nil
	}
	return m.operation("Stage "+c.Path, func() (string, error) { return m.repo.Stage(m.ctx, c) })
}

func (m *Model) stageAll() tea.Cmd {
	if m.hasDirtyTabs() {
		m.notice = "Save edited files before staging all changes"
		return nil
	}
	return m.gitOperation("Stage all", "add", "--all", "--", ".")
}

func (m *Model) stageHunk(reverse bool) tea.Cmd {
	if m.diffLoading || m.diffChange == nil || m.showEditor {
		m.notice = "Open a working-tree or index diff first"
		return nil
	}
	if m.diffStaged != reverse {
		m.notice = "Use S in a working-tree diff or U in an index diff"
		return nil
	}
	patch, err := m.patch.HunkPatch(m.diffHunk)
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	c, staged, raw := *m.diffChange, m.diffStaged, m.patch.Raw
	title := "Stage hunk"
	if reverse {
		title = "Unstage hunk"
	}
	return m.operation(title, func() (string, error) {
		current, err := m.repo.Diff(m.ctx, c, staged)
		if err != nil {
			return "", err
		}
		if current != raw {
			return "", fmt.Errorf("diff changed; refresh before applying a hunk")
		}
		return m.repo.ApplyHunk(m.ctx, patch, reverse)
	})
}

func (m *Model) commit() tea.Cmd {
	for _, c := range m.snapshot.Changes {
		if c.Conflicted() {
			m.notice = "Resolve and stage all conflicts before committing"
			return nil
		}
	}
	for _, c := range m.snapshot.Changes {
		if c.Staged() {
			return m.prompt("commit", "Commit staged changes", "")
		}
	}
	m.notice = "Stage changes before committing"
	return nil
}

func (m *Model) openSelected() tea.Cmd {
	entry := m.current()
	if entry == nil {
		return nil
	}
	if m.section == history {
		return m.openHistory(*entry)
	}
	if entry.header && m.section == changes {
		m.changeCollapsed[entry.key] = !m.changeCollapsed[entry.key]
		m.rebuildItems()
		return nil
	}
	if entry.folder != "" {
		if m.section == changes {
			m.changeCollapsed[entry.key] = !m.changeCollapsed[entry.key]
			m.rebuildItems()
			return nil
		}
		m.expanded[entry.folder] = !m.expanded[entry.folder]
		return m.refreshTree()
	}
	if entry.file != "" {
		return m.openFile(entry.file)
	}

	m.sidebar = false
	return m.preview()
}

func (m *Model) editSelected() tea.Cmd {
	entry := m.current()
	if entry == nil {
		return nil
	}
	if entry.file != "" {
		return m.openFile(entry.file)
	}
	if entry.change != nil {
		return m.openFile(entry.change.Path)
	}
	return nil
}
func (m *Model) editDiff() tea.Cmd {
	if m.diffCommit != "" {
		m.notice = "Commit diffs are read-only"
		return nil
	}
	if m.diffPath != "" {
		return m.openFile(m.diffPath)
	}
	return nil
}

func (m *Model) discard() tea.Cmd {
	entry := m.current()
	if entry == nil || entry.change == nil {
		return nil
	}
	c := *entry.change
	if c.Conflicted() || entry.staged {
		m.notice = "Select a non-conflicted working-tree file to discard"
		return nil
	}
	if m.hasDirtyTabs() {
		m.notice = "Save or close edited files before discarding"
		return nil
	}
	title := "Permanently discard unstaged edits to " + c.Path + "?"
	if c.Untracked() {
		title = "删除未跟踪文件 " + c.Path + "？"
	}
	cmd := m.ask(title, func() tea.Cmd {
		if m.hasDirtyTabs() {
			m.notice = "Save or close edited files before discarding"
			return nil
		}
		return m.operation("Discard", func() (string, error) { return m.repo.Discard(m.ctx, c) })
	})
	if c.Untracked() {
		m.confirmationDetail = "Git 无法恢复此文件。"
	}
	return cmd
}

func (m *Model) quit() tea.Cmd {
	if m.busy {
		m.notice = "Wait for the current Git operation before quitting"
		return nil
	}
	if m.hasDirtyTabs() {
		return m.ask("Discard unsaved editor changes and quit?", func() tea.Cmd { return tea.Quit })
	}
	return tea.Quit
}

func (m *Model) switchTab(delta int) tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	m.activeTab = (m.activeTab + delta + len(m.tabs)) % len(m.tabs)
	m.showEditor = true
	m.sidebar = false
	m.ensureTabVisible()
	return nil
}

func (m *Model) findNext() tea.Cmd {
	if m.find == "" {
		return nil
	}
	if m.showEditor && m.document() != nil {
		d := m.document()
		runes := []rune(d.Text)
		start := min(d.Cursor+1, len(runes))
		index := strings.Index(string(runes[start:]), m.find)
		if index < 0 {
			start = 0
			index = strings.Index(d.Text, m.find)
		}
		if index >= 0 {
			prefix := []rune(string(runes[start:])[:index])
			d.SelectRange(start+len(prefix), start+len(prefix)+len([]rune(m.find)))
			m.sidebar = false
			m.ensureEditor()
		} else {
			m.notice = "No match"
		}
	} else {
		lines := strings.Split(m.viewport.GetContent(), "\n")
		start := m.viewport.YOffset() + 1
		for i := 0; i < len(lines); i++ {
			n := (start + i) % len(lines)
			if strings.Contains(strings.ToLower(plain(lines[n])), strings.ToLower(m.find)) {
				m.viewport.SetYOffset(n)
				return nil
			}
		}
		m.notice = "No match"
	}
	return nil
}
