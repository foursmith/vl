package app

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type action struct {
	id, label, key string
	disabled       string
	run            func() tea.Cmd
}

func (m *Model) actions() []action {
	target := m.selectedRepository()
	suffix := " · " + m.repositoryName(target)
	commitLabel, stageAllLabel := "Git: Commit Staged Changes"+suffix, "Git: Stage All Changes"+suffix
	fileReason := ""
	if m.current() == nil || m.current().change == nil {
		fileReason = "Select a file in Changes"
	}
	docReason := ""
	tabReason := ""
	if m.activeTab < 0 {
		tabReason = "No active tab"
	}
	openReason := ""
	if m.current() == nil || m.current().change == nil && m.current().file == "" {
		openReason = "Select a file in Explorer or Changes"
	}
	if !m.editing() {
		docReason = "Open a text file first"
	}
	githubReason := ""
	if m.repositoryState(m.statusRepository()).GitHubURL == "" {
		githubReason = "没有找到 github.com 远程仓库"
	}
	actions := []action{
		{"folder.openRecent", "File: Open Recent Folder", workbenchKeys.Recent.Help().Key, "", m.openRecentFolders},
		{"theme.select", "Preferences: Color Theme", "", "", m.openThemePicker},

		{"view.explorer", "View: Explorer", workbenchKeys.Explorer.Help().Key, "", func() tea.Cmd { return m.selectSection(explorer) }},
		{"explorer.newFile", "Explorer: New File", "", "", func() tea.Cmd { return m.newEntry(false) }},
		{"explorer.newFolder", "Explorer: New Folder", "", "", func() tea.Cmd { return m.newEntry(true) }},
		{"explorer.collapseAll", "Explorer: Collapse Folders", "", "", m.collapseFolders},
		{"view.changes", "View: Changes", workbenchKeys.Changes.Help().Key, "", func() tea.Cmd { return m.selectSection(changes) }},
		{"view.history", "View: Commit History", workbenchKeys.History.Help().Key, "", func() tea.Cmd { return m.selectSection(history) }},
		{"view.diff", "Diff: Toggle Side by Side", workbenchKeys.DiffLayout.Help().Key, "", func() tea.Cmd { m.split = !m.split; m.renderDiff(); return nil }},
		{"view.sidebar", "View: Toggle Sidebar", workbenchKeys.Sidebar.Help().Key, "", m.toggleSidebar},
		{"view.sidebarLeft", "View: Move Sidebar Left", "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, false); return nil }},
		{"view.sidebarRight", "View: Move Sidebar Right", "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, true); return nil }},
		{"view.sidebarResetWidth", "View: Reset Sidebar Width", "", "", func() tea.Cmd { m.SetSidebarLayout(0, m.sidebarRight); return nil }},
		{"view.output", "View: Toggle Output", workbenchKeys.Output.Help().Key, "", func() tea.Cmd { m.outputVisible = !m.outputVisible; m.resize(); return nil }},
		{"git.refresh", "Git: Refresh", workbenchKeys.Refresh.Help().Key, "", m.refresh},
		{"git.stage", "Git: Stage Selected File", "S", fileReason, func() tea.Cmd { return m.stageSelected(false) }},
		{"git.unstage", "Git: Unstage Selected File", "U", fileReason, func() tea.Cmd { return m.stageSelected(true) }},
		{"git.stageAll", stageAllLabel, "A", "", func() tea.Cmd { return m.stageAllInRepository(target) }},
		{"git.commit", commitLabel, "C", m.commitDisabled(target), func() tea.Cmd { return m.commitInRepository(target) }},
		{"git.fetch", "Git: Fetch", "", "", func() tea.Cmd { return m.gitOperation("Fetch", "fetch", "--all", "--prune") }},
		{"git.pull", "Git: Pull (Fast Forward Only)", "", "", func() tea.Cmd { return m.gitOperation("Pull", "pull", "--ff-only") }},
		{"git.push", "Git: Push", "", "", func() tea.Cmd { return m.gitOperation("Push", "push") }},
		{"git.branch", "Git: Create Branch", "", "", func() tea.Cmd {
			m.branchStart = ""
			m.branchRepository = ""
			return m.prompt("branch", "Create and switch to branch", "")
		}},
		{"git.checkout", "Git: Checkout Branch or Tag", "", "", func() tea.Cmd { return m.openBranchPicker("", "") }},
		{"git.sync", "Git: Sync with Upstream", "", "", m.syncRepository},
		{"git.publish", "Git: Publish Branch", "", "", m.publishBranch},
		{"github.open", "GitHub: Open Repository", "", githubReason, m.openGitHub},
		{"git.stash", "Git: Stash Changes (Including Untracked)", "", "", func() tea.Cmd { return m.prompt("stash", "Stash message", "") }},
		{"git.stashPop", "Git: Pop Latest Stash", "", "", func() tea.Cmd {
			return m.ask("Apply and remove the latest stash?", func() tea.Cmd { return m.gitOperation("Stash pop", "stash", "pop") })
		}},
		{"git.discard", "Git: Discard Unstaged Changes", "", fileReason, m.discard},
		{"file.open", "File: Open Selected File", "E", openReason, m.editSelected},
		{"file.save", "File: Save", workbenchKeys.Save.Help().Key, docReason, m.save},
		{"file.close", "File: Close Active Tab", workbenchKeys.Close.Help().Key, tabReason, m.closeTab},
		{"file.keepOpen", "File: Keep Preview Open", "", tabReason, m.keepTab},
		{"file.closeOthers", "File: Close Other Tabs", "", "", m.closeOtherTabs},
		{"file.closeAll", "File: Close All Tabs", "", "", m.closeAllTabs},
		{"edit.find", "Find: Search Current View", workbenchKeys.Find.Help().Key, "", func() tea.Cmd { return m.prompt("find", "Find in current view", m.find) }},
		{"app.quit", "Quit", workbenchKeys.Quit.Help().Key, "", m.quit},
	}
	if m.snapshot.NoRepository {
		for i := range actions {
			if strings.HasPrefix(actions[i].id, "git.") || actions[i].id == "github.open" {
				actions[i].disabled = "此文件夹尚未初始化 Git 仓库"
			}
		}
		actions = append(actions, action{id: "git.init", label: "Git: Initialize Repository", run: m.initializeRepository})
	}
	return actions
}

func (m *Model) initializeRepository() tea.Cmd {
	if !m.snapshot.NoRepository {
		return nil
	}
	if m.repo.HasRepository() {
		return m.refresh()
	}
	return m.startOperation("Initialize repository", func() (string, error) { return m.repo.Run(m.ctx, "init") })
}

func (m *Model) commandActions() []action {
	actions := m.actions()
	slices.SortFunc(actions, func(a, b action) int {
		if (a.id == m.lastCommand) != (b.id == m.lastCommand) {
			if a.id == m.lastCommand {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.label), strings.ToLower(b.label))
	})
	return actions
}

func (m *Model) openPalette() tea.Cmd { return m.openQuickPicker(">") }

func (m *Model) filteredActions() []action {
	query := m.input.Value()
	if m.commandPicker() {
		query = strings.TrimSpace(strings.TrimPrefix(query, ">"))
	} else if m.modal == "quick" {
		if rows, ok := m.quickPositionActions(query); ok {
			return rows
		}
	}
	query = strings.ToLower(query)
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
	m.dialogTop = 0
	m.input.SetValue(value)
	m.input.Placeholder = ""
	m.input.Prompt = "> "
	m.resizeInput()
	if m.isPicker() {
		m.input.Prompt = ""
		m.input.Placeholder = title
	}
	m.input.CursorEnd()
	m.pickerIndex = 0
	m.syncPicker()
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
	m.confirmationLabel = "确认"
	m.confirmationButton = 0
	return m.prompt("confirm", title, "")
}

func (m *Model) updateModal(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if m.confirmationDialog() {
		count := len(m.confirmationActions())
		switch k {
		case "right", "tab":
			m.confirmationButton = (m.confirmationButton + 1) % count
			return nil
		case "left", "shift+tab":
			m.confirmationButton = (m.confirmationButton + count - 1) % count
			return nil
		case "enter", "space":
			return m.chooseConfirmation()
		}
	}
	if m.confirmationDialog() {
		l := m.dialogLayout()
		top := l.top
		switch k {
		case "up":
			top--
		case "down":
			top++
		case "pgup":
			top -= l.page
		case "pgdown":
			top += l.page
		case "home":
			top = 0
		case "end":
			top = l.maxTop
		}
		if top != l.top {
			m.dialogTop = min(max(0, top), l.maxTop)
			return nil
		}
	}
	if m.isPicker() {
		switch {
		case key.Matches(msg, workbenchKeys.Files):
			return m.openQuickFiles()
		case key.Matches(msg, workbenchKeys.Commands):
			return m.openPalette()
		}
	}
	if m.modal == "close" {
		switch k {
		case "s":
			return m.chooseClose("save")
		case "d":
			return m.chooseClose("discard")
		case "esc":
			return m.chooseClose("cancel")
		}
		return nil
	}
	if key.Matches(msg, workbenchKeys.Escape) {
		restore := m.cancelThemePreview()
		m.rememberCommitDraft()
		m.modal = ""
		m.confirm = nil
		m.input.Blur()
		return restore
	}
	if m.isPicker() {
		if key.Matches(msg, pickerKeys.Accept) {
			return m.selectPickerChoice()
		}
		if m.pickerKey(msg) {
			return m.previewSelectedTheme()
		}
	}

	if k == "enter" && !m.isPicker() {
		m.rememberCommitDraft()
		kind, value := m.modal, strings.TrimSpace(m.input.Value())
		m.modal = ""
		m.input.Blur()
		switch kind {
		case "commit":
			if value != "" {
				target := m.commitRepository
				if reason := m.commitDisabled(target); reason != "" {
					m.notice = reason
					return nil
				}
				return m.operation("Commit", func() (string, error) {
					repo, err := m.repo.RepositoryAt(m.ctx, target)
					if err != nil {
						return "", err
					}
					return repo.Run(m.ctx, "commit", "-m", value)
				})
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
	m.syncQuickPicker()
	m.selectPickerIndex(0)
	return cmd
}

func (m *Model) operation(title string, run func() (string, error)) tea.Cmd {
	if m.snapshot.NoRepository {
		m.notice = "此文件夹尚未初始化 Git 仓库"
		return nil
	}
	return m.startOperation(title, run)
}

func (m *Model) startOperation(title string, run func() (string, error)) tea.Cmd {
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
	for _, t := range m.tabs {
		if d := t.diff; d != nil {
			d.diffSeq++
			if d.diffCancel != nil {
				d.diffCancel()
				d.diffCancel = nil
			}
			d.diffLoading = false
		}
	}
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
	if m.section == changes && entry != nil && entry.repoHeader {
		scope := *entry
		scope.staged = unstage
		id := "stage"
		if unstage {
			id = "unstage"
		}
		return m.runScopeAction(scope, id)
	}
	if entry == nil || entry.change == nil {
		m.notice = "Select a changed file first"
		return nil
	}
	c := *entry.change
	if !unstage && c.GitlinkDirty {
		m.notice = "子模块内部文件有更改，请在其仓库中暂存和提交"
		return nil
	}
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
	return m.stageAllInRepository(m.selectedRepository())
}

func (m *Model) stageAllInRepository(target string) tea.Cmd {
	if m.hasDirtyTabsInRepository(target) {
		m.notice = "Save edited files before staging all changes"
		return nil
	}
	return m.operation("Stage all", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.Run(m.ctx, "add", "--all", "--", ".")
	})
}

func (m *Model) stageHunk(reverse bool) tea.Cmd {
	if m.images != nil {
		m.notice = "Stage or unstage the whole image file from Changes"
		return nil
	}
	if m.diffLoading || m.diffChange == nil || m.editing() {
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
		repo, _, err := m.repo.ForChange(m.ctx, c)
		if err != nil {
			return "", err
		}
		return repo.ApplyHunk(m.ctx, patch, reverse)
	})
}

func (m *Model) commit() tea.Cmd { return m.commitInRepository(m.selectedRepository()) }

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
	cmd := m.preview()
	m.keepTab()
	return cmd
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
		if entry.change.Gitlink {
			m.notice = "子模块提交指针；请在对应仓库中选择文件"
			return nil
		}
		return m.openFile(entry.change.Path)
	}
	return nil
}
func (m *Model) editDiff() tea.Cmd {
	if m.diffCommit != "" {
		m.notice = "Commit diffs are read-only"
		return nil
	}
	if m.diffChange != nil && m.diffChange.Gitlink {
		m.notice = "子模块提交指针；请在对应仓库中选择文件"
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
	if c.Gitlink {
		m.notice = "仓库目录不能作为文件丢弃"
		return nil
	}
	if c.Conflicted() || entry.staged {
		m.notice = "Select a non-conflicted working-tree file to discard"
		return nil
	}
	if m.hasDirtyTabsInRepository(c.Repository) {
		m.notice = "Save or close edited files before discarding"
		return nil
	}
	title := "Permanently discard unstaged edits to " + c.Path + "?"
	if c.Untracked() {
		title = "删除未跟踪文件 " + c.Path + "？"
	}
	cmd := m.ask(title, func() tea.Cmd {
		if m.hasDirtyTabsInRepository(c.Repository) {
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
	index := ((m.activeTab+delta)%len(m.tabs) + len(m.tabs)) % len(m.tabs)
	m.activateTab(index)
	return nil
}

func (m *Model) findNext() tea.Cmd {
	if m.find == "" {
		return nil
	}
	if m.editing() {
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
