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
	editorActions := m.editorActions()
	if m.editorOnly {
		return editorActions
	}
	target := m.selectedRepository()
	suffix := " · " + m.repositoryName(target)
	commitLabel, stageAllLabel := m.tr("Git: Commit Staged")+suffix, m.tr("Git: Stage All Changes")+suffix
	fileReason := ""
	if m.current() == nil || m.current().change == nil {
		fileReason = m.tr("Select a file in Changes")
	}
	tabReason := ""
	if m.activeTab < 0 {
		tabReason = m.tr("No active tab")
	}
	openReason := ""
	if m.current() == nil || m.current().change == nil && m.current().file == "" {
		openReason = m.tr("Select a file in Explorer or Changes")
	}
	githubReason := ""
	if m.repositoryState(m.statusRepository()).GitHubURL == "" {
		githubReason = m.tr("No GitHub remote found")
	}
	actions := []action{
		{"workspace.openRecent", m.tr("File: Open Recent Workspace"), workbenchKeys.Recent.Help().Key, "", m.openRecentWorkspaces},

		{"view.explorer", m.tr("View: Show Explorer"), workbenchKeys.Explorer.Help().Key, "", func() tea.Cmd { return m.selectSection(explorer) }},
		{"explorer.newFile", m.tr("Explorer: New File"), "", "", func() tea.Cmd { return m.newEntry(false) }},
		{"explorer.newFolder", m.tr("Explorer: New Folder"), "", "", func() tea.Cmd { return m.newEntry(true) }},
		{"explorer.collapseAll", m.tr("Explorer: Collapse Folders in Explorer"), "", "", m.collapseFolders},
		{"search.options", m.tr("Search: Options"), "Ctrl+Shift+J", "", m.openSearchOptions},
		{"search.replace", m.tr("Search: Replace in Files"), workbenchKeys.Replace.Help().Key, "", m.openReplace},
		{"view.search", m.tr("Search: Find in Files"), workbenchKeys.Search.Help().Key, "", m.openSearch},
		{"view.changes", m.tr("View: Changes"), workbenchKeys.Changes.Help().Key, "", func() tea.Cmd { return m.selectSection(changes) }},
		{"view.history", m.tr("View: Commit History"), workbenchKeys.History.Help().Key, "", func() tea.Cmd { return m.selectSection(history) }},
		{"view.diff", m.tr("Diff Editor: Toggle Inline View"), workbenchKeys.DiffLayout.Help().Key, "", func() tea.Cmd { m.split = !m.split; m.renderDiff(); return nil }},
		{"view.sidebar", m.tr("View: Toggle Sidebar"), workbenchKeys.Sidebar.Help().Key, "", m.toggleSidebar},
		{"view.sidebarLeft", m.tr("View: Move Sidebar Left"), "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, false); return nil }},
		{"view.sidebarRight", m.tr("View: Move Sidebar Right"), "", "", func() tea.Cmd { m.SetSidebarLayout(m.sidebarWidth, true); return nil }},
		{"view.sidebarResetWidth", m.tr("View: Reset Sidebar Width"), "", "", func() tea.Cmd { m.SetSidebarLayout(0, m.sidebarRight); return nil }},
		{"view.output", m.tr("View: Toggle Output"), workbenchKeys.Output.Help().Key, "", func() tea.Cmd { m.outputVisible = !m.outputVisible; m.resize(); return nil }},
		{"git.refresh", m.tr("Git: Refresh"), workbenchKeys.Refresh.Help().Key, "", m.refresh},
		{"git.stage", m.tr("Git: Stage Changes"), "s", fileReason, func() tea.Cmd { return m.stageSelected(false) }},
		{"git.unstage", m.tr("Git: Unstage Changes"), "u", fileReason, func() tea.Cmd { return m.stageSelected(true) }},
		{"git.stageAll", stageAllLabel, "a", "", func() tea.Cmd { return m.stageAllInRepository(target) }},
		{"git.commit", commitLabel, "c", m.commitDisabled(target), func() tea.Cmd { return m.commitInRepository(target) }},
		{"git.fetch", m.tr("Git: Fetch"), "", "", func() tea.Cmd { return m.gitOperation("Fetch", "fetch", "--all", "--prune") }},
		{"git.pull", m.tr("Git: Pull (Fast Forward Only)"), "", "", func() tea.Cmd { return m.gitOperation("Pull", "pull", "--ff-only") }},
		{"git.push", m.tr("Git: Push"), "", "", func() tea.Cmd { return m.gitOperation("Push", "push") }},
		{"git.branch", m.tr("Git: Create Branch…"), "", "", func() tea.Cmd {
			m.branchStart = ""
			m.branchRepository = ""
			return m.prompt("branch", m.tr("Create and switch to branch"), "")
		}},
		{"git.checkout", m.tr("Git: Checkout to…"), "", "", func() tea.Cmd { return m.openBranchPicker("", "") }},
		{"git.sync", m.tr("Git: Sync"), "", "", m.syncRepository},
		{"git.publish", m.tr("Git: Publish Branch"), "", "", m.publishBranch},
		{"github.open", m.tr("GitHub: Open Repository"), "", githubReason, m.openGitHub},
		{"git.stash", m.tr("Git: Stash (Include Untracked)"), "", "", func() tea.Cmd { return m.prompt("stash", m.tr("Stash message"), "") }},
		{"git.stashPop", m.tr("Git: Pop Latest Stash"), "", "", func() tea.Cmd {
			return m.ask(m.tr("Apply and remove the latest stash?"), func() tea.Cmd { return m.gitOperation("Stash pop", "stash", "pop") })
		}},
		{"git.discard", m.tr("Git: Discard Unstaged Changes"), "", fileReason, m.discard},
		{"file.open", m.tr("File: Open Selected File"), "e", openReason, m.editSelected},
		{"file.keepOpen", m.tr("View: Keep Editor"), "", tabReason, m.keepTab},
		{"file.closeOthers", m.tr("View: Close Other Editors"), "", "", m.closeOtherTabs},
		{"file.closeAll", m.tr("View: Close All Editors"), "", "", m.closeAllTabs},
	}
	if m.snapshot.NoRepository {
		for i := range actions {
			if strings.HasPrefix(actions[i].id, "git.") || actions[i].id == "github.open" {
				actions[i].disabled = m.tr("This folder is not a Git repository")
			}
		}
		actions = append(actions, action{id: "git.init", label: m.tr("Git: Initialize Repository"), run: m.initializeRepository})
	}
	if m.diffOpen && !m.editing() && m.images == nil {
		actions = append(actions, m.reviewActions()...)
	}
	return append(editorActions, actions...)
}

func (m *Model) initializeRepository() tea.Cmd {
	if m.editorOnly || !m.snapshot.NoRepository {
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

// Cache filtering across passive mouse motion and repeated reads in one event.
// Other events clear the cache before they can change action labels or targets.
type pickerFilterCache struct {
	valid        bool
	modal, query string
	palette      *action
	count        int
	rows         []action
}

func (m *Model) filteredActions() []action {
	var first *action
	if len(m.palette) > 0 {
		first = &m.palette[0]
	}
	cache := &m.pickerFilter
	if cache.valid && cache.modal == m.modal && cache.query == m.input.Value() && cache.palette == first && cache.count == len(m.palette) {
		return cache.rows
	}
	rows := m.filterActions()
	*cache = pickerFilterCache{valid: true, modal: m.modal, query: m.input.Value(), palette: first, count: len(m.palette), rows: rows}
	return rows
}

func (m *Model) filterActions() []action {
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
	var heading action
	pendingHeading := false
	for _, a := range m.palette {
		if m.pickerHeading(a) {
			heading, pendingHeading = a, true
			continue
		}
		if strings.Contains(strings.ToLower(a.label), query) || a.id != a.label && strings.Contains(strings.ToLower(a.id), query) ||
			strings.Contains(strings.ToLower(heading.label), query) {
			if pendingHeading {
				result = append(result, heading)
				pendingHeading = false
			}
			result = append(result, a)
		}
	}
	return result
}

func (m *Model) prompt(kind, title, value string) tea.Cmd {
	m.leaderTarget = ""
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
	m.confirmationLabel = m.tr("Confirm")
	m.confirmationButton = 0
	return m.prompt("confirm", title, "")
}

func (m *Model) updateModal(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if m.modal == "searchOptions" {
		if (k == "space" || k == " ") && m.input.Value() == "" {
			return m.selectPickerChoice()
		}
	}

	if m.confirmationDialog() {
		count := len(m.confirmationActions())
		switch k {
		case "right", "l", "tab":
			m.confirmationButton = (m.confirmationButton + 1) % count
			return nil
		case "left", "h", "shift+tab":
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
		case "up", "k":
			top--
		case "down", "j":
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
		case "newFile":
			return m.createEntry(value, false)
		case "pasteFile":
			return m.pasteEntry(value)
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
		m.notice = m.tr("This folder is not a Git repository")
		return nil
	}
	return m.startOperation(title, run)
}

func (m *Model) startOperation(title string, run func() (string, error)) tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation")
		return nil
	}
	m.busy = true
	m.notice = m.tr(title) + "…"
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
		m.notice = m.tr("Save or close edited files before this operation")
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
		m.notice = m.tr("Select a changed file first")
		return nil
	}
	c := *entry.change
	if !unstage && c.GitlinkDirty {
		m.notice = m.tr("Submodule has file changes; stage and commit them in its repository")
		return nil
	}
	if !unstage && m.fileDirty(c.Path) {
		m.notice = m.tr("Save this file before staging")
		return nil
	}
	if unstage {
		if !c.Staged() {
			m.notice = m.tr("This file has no staged changes")
			return nil
		}
		return m.operation(m.tr("Unstage %s", c.Path), func() (string, error) { return m.repo.Unstage(m.ctx, c) })
	}
	if !c.Unstaged() {
		m.notice = m.tr("This file has no unstaged changes")
		return nil
	}
	return m.operation(m.tr("Stage %s", c.Path), func() (string, error) { return m.repo.Stage(m.ctx, c) })
}

func (m *Model) stageAll() tea.Cmd {
	return m.stageAllInRepository(m.selectedRepository())
}

func (m *Model) stageAllInRepository(target string) tea.Cmd {
	if m.hasDirtyTabsInRepository(target) {
		m.notice = m.tr("Save edited files before staging all changes")
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
		m.notice = m.tr("Stage or unstage the whole image file from Changes")
		return nil
	}
	if m.diffLoading || m.diffChange == nil || m.editing() {
		m.notice = m.tr("Open a working-tree or index diff first")
		return nil
	}
	if m.diffStaged != reverse {
		m.notice = m.tr("Use Stage Hunk in a working-tree diff or Unstage Hunk in an index diff")
		return nil
	}
	patch, err := m.patch.HunkPatch(m.diffHunk)
	if err != nil {
		m.notice = m.tr(err.Error())
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
	if m.section == search {
		return m.openSidebarFile(false)
	}
	if m.snapshot.NoRepository && (m.section == changes || m.section == history) {
		return m.initializeRepository()
	}
	entry := m.current()
	if entry == nil {
		return nil
	}
	if m.section == history {
		return m.openHistoryRow(*entry, true)
	}
	if m.section == changes && (entry.header || entry.folder != "") {
		m.changeCollapsed[entry.key] = !m.changeCollapsed[entry.key]
		m.rebuildItems()
		return nil
	}
	if entry.folder != "" {
		m.expanded[entry.folder] = !m.expanded[entry.folder]
		return m.refreshTree()
	}
	return m.openSidebarFile(false)
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
			m.notice = m.tr("Submodule commit pointer; select files in its repository")
			return nil
		}
		return m.openFile(entry.change.Path)
	}
	return nil
}
func (m *Model) editDiff() tea.Cmd {
	if m.diffCommit != "" {
		m.notice = m.tr("Commit diffs are read-only")
		return nil
	}
	if m.diffChange != nil && m.diffChange.Gitlink {
		m.notice = m.tr("Submodule commit pointer; select files in its repository")
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
		m.notice = m.tr("Cannot discard a repository directory as a file")
		return nil
	}
	if c.Conflicted() || entry.staged {
		m.notice = m.tr("Select a non-conflicted working-tree file to discard")
		return nil
	}
	if m.hasDirtyTabsInRepository(c.Repository) {
		m.notice = m.tr("Save or close edited files before discarding")
		return nil
	}
	title := m.tr("Permanently discard unstaged edits to %s?", c.Path)
	if c.Untracked() {
		title = m.tr("Delete untracked file %s?", c.Path)
	}
	cmd := m.ask(title, func() tea.Cmd {
		if m.hasDirtyTabsInRepository(c.Repository) {
			m.notice = m.tr("Save or close edited files before discarding")
			return nil
		}
		return m.operation("Discard", func() (string, error) { return m.repo.Discard(m.ctx, c) })
	})
	if c.Untracked() {
		m.confirmationDetail = m.tr("This file cannot be recovered by Git.")
	}
	return cmd
}

func (m *Model) quit() tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation before quitting")
		return nil
	}
	if m.hasDirtyTabs() {
		return m.ask(m.tr("Discard unsaved editor changes and quit?"), func() tea.Cmd { return tea.Quit })
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
	m.findHidden = false
	if m.findOptions.Query != "" {
		if !m.findWidget.workspaceMatch {
			m.presentFileFind()
		}
		m.navigateFileFind(1)
	}
	return nil
}
