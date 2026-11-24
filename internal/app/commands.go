package app

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
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
	stageTarget := target
	if !m.sidebar && m.diffOpen && m.diffChange != nil {
		stageTarget = m.diffRepository
	}
	stageSuffix := " · " + m.repositoryName(stageTarget)
	var remoteTarget, stageAllKey, unstageAllKey, commitKey, stashKey, fetchKey, pullKey, pushKey string
	if m.section == changes {
		remoteTarget = target
		if m.sidebar {
			if m.stageScopeUnstages(m.current()) {
				unstageAllKey = gitKeys.StageScope.Help().Key
			} else {
				stageAllKey = gitKeys.StageScope.Help().Key
			}
			commitKey = gitKeys.Commit.Help().Key
			stashKey = gitKeys.Stash.Help().Key
			fetchKey, pullKey, pushKey = gitKeys.Fetch.Help().Key, gitKeys.Pull.Help().Key, gitKeys.Push.Help().Key
		}
	}
	tabReason := ""
	if m.activeTab < 0 {
		tabReason = m.tr("No active tab")
	}
	githubReason := ""
	if m.repositoryState(m.statusRepository()).GitHubURL == "" {
		githubReason = m.tr("No GitHub remote found")
	}
	historyPath := m.fileHistoryPath()
	fileHistory := m.fileHistoryAction(historyPath)
	if historyPath == "" {
		fileHistory.disabled = m.tr("Select a file in the workspace")
	} else if m.gitUnavailable {
		fileHistory.disabled = m.tr("Git is not available")
	}
	actions := []action{
		m.commandAction(commandRecent, "File: Open Recent Workspace", "", ""),

		m.commandAction(commandExplorer, "View: Show Explorer", "", ""),
		{"explorer.newFile", m.tr("Explorer: New File"), "", "", func() tea.Cmd { return m.newEntry(false) }},
		{"explorer.newFolder", m.tr("Explorer: New Folder"), "", "", func() tea.Cmd { return m.newEntry(true) }},
		{"search.replace", m.tr("Search: Replace in Files"), "", "", m.openReplace},
		m.commandAction(commandSearch, "Search: Find in Files", "", ""),
		m.commandAction(commandChanges, "View: Changes", "", ""),
		m.commandAction(commandHistory, "View: Commit History", "", ""),
		fileHistory,
		m.commandAction(commandPullRequests, "View: Pull Requests", "", ""),
		m.commandAction(commandDiffLayout, "Diff Editor: Toggle Inline View", "", ""),
		m.commandAction(commandSidebar, "Appearance: Toggle Sidebar Visibility", "", ""),
		{"view.sidebarPosition", m.tr("Appearance: Toggle Sidebar Position"), "", "", func() tea.Cmd {
			m.SetSidebarLayout(m.sidebarWidth, !m.sidebarRight)
			return m.savePreferences()
		}},
		{"view.sidebarResetWidth", m.tr("Appearance: Reset Sidebar Width"), "", "", func() tea.Cmd {
			m.SetSidebarLayout(0, m.sidebarRight)
			return m.savePreferences()
		}},
		{"git.stageAll", m.tr("Git: Stage All Changes") + stageSuffix, stageAllKey, m.repositoryStageReason(stageTarget, false), func() tea.Cmd { return m.stageRepository(stageTarget, false) }},
		{"git.unstageAll", m.tr("Git: Unstage All Changes") + stageSuffix, unstageAllKey, m.repositoryStageReason(stageTarget, true), func() tea.Cmd { return m.stageRepository(stageTarget, true) }},
		{"git.commit", m.tr("Git: Commit Staged") + suffix, commitKey, m.commitDisabled(target), func() tea.Cmd { return m.commitInRepository(target) }},
		{"git.fetch", m.tr("Git: Fetch"), fetchKey, "", func() tea.Cmd { return m.fetch(remoteTarget) }},
		{"git.pull", m.tr("Git: Pull (Fast Forward Only)"), pullKey, "", func() tea.Cmd { return m.pull(remoteTarget) }},
		{"git.push", m.tr("Git: Push"), pushKey, "", func() tea.Cmd { return m.push(remoteTarget) }},
		{"git.branch", m.tr("Git: Create Branch…"), "", "", func() tea.Cmd {
			m.branchStart = ""
			m.branchRepository = ""
			return m.prompt("branch", m.tr("Create and switch to branch"), "")
		}},
		{"git.checkout", m.tr("Git: Checkout to…"), "", "", func() tea.Cmd { return m.openBranchPicker("", "") }},
		{"git.sync", m.tr("Git: Sync"), "", "", m.syncRepository},
		{"git.publish", m.tr("Git: Publish Branch"), "", "", m.publishBranch},
		{"github.open", m.tr("GitHub: Open Repository"), "", githubReason, m.openGitHub},
		{"git.stash", m.tr("Git: Stash (Include Untracked)"), stashKey, "", func() tea.Cmd { return m.stash(target) }},
		{"git.stashList", m.tr("Git: Manage Stashes…") + suffix, "", "", func() tea.Cmd { return m.openStashes(target) }},
		{"git.stashModes", m.tr("Git: New Stash…") + suffix, "", "", func() tea.Cmd { return m.openStashModes(target) }},
		{"file.keepOpen", m.tr("View: Keep Editor"), "", tabReason, m.keepTab},
		m.commandAction(commandCloseOthers, "View: Close Other Editors", "", ""),
		m.commandAction(commandCloseAll, "View: Close All Editors", "", ""),
	}
	if m.snapshot.NoRepository {
		reason := m.tr("This folder is not a Git repository")
		if m.gitUnavailable {
			reason = m.tr("Git is not available")
		}
		for i := range actions {
			if strings.HasPrefix(actions[i].id, "git.") || actions[i].id == "github.open" {
				actions[i].disabled = reason
			}
		}
		if !m.gitUnavailable {
			actions = append(actions, action{id: "git.init", label: m.tr("Git: Initialize Repository"), run: m.initializeRepository})
		}
	}
	return append(editorActions, actions...)
}

func (m *Model) initializeRepository() tea.Cmd {
	if m.editorOnly || m.gitUnavailable || !m.snapshot.NoRepository {
		return nil
	}
	if m.repo.HasRepository() {
		return m.refresh()
	}
	return m.startOperation("Initialize repository", func() (string, error) {
		return m.repo.Initialize(m.ctx)
	})
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
	if m.modal == "githubPublish" {
		return m.githubPublishActions()
	}
	query := m.input.Value()
	if m.commandPicker() {
		query = strings.TrimSpace(strings.TrimPrefix(query, ">"))
	} else if m.modal == "quick" {
		if command := m.quickVimCommand(query); command.run != nil {
			return []action{command}
		}
		query = strings.TrimPrefix(query, "./")
		if strings.HasSuffix(query, "/") {
			var rows []action
			for _, a := range m.palette {
				if strings.HasPrefix(a.id, query) {
					rows = append(rows, a)
				}
			}
			return rows
		}
		if rows, ok := m.quickPositionActions(query); ok {
			return rows
		}
		if len(m.palette) > 0 && m.quickFilesFilter.base == &m.palette[0] {
			return m.matchQuickFiles(query)
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

type quickFilesFilter struct {
	base  *action
	query string
	rows  []action
}

func (m *Model) matchQuickFiles(query string) []action {
	query = strings.ToLower(query)
	if query == "" {
		return m.palette
	}
	filter := &m.quickFilesFilter
	cached := len(m.palette) > 0 && filter.base == &m.palette[0]
	if cached && filter.query == query {
		return filter.rows
	}
	candidates := m.palette
	if cached && filter.query != "" && strings.HasPrefix(query, filter.query) {
		candidates = filter.rows
	}
	var rows []action
	if len(query) == 1 {
		// A single character commonly matches most paths in a large worktree.
		rows = make([]action, 0, len(candidates))
	}
	for _, file := range candidates {
		if strings.Contains(strings.ToLower(file.id), query) {
			rows = append(rows, file)
		}
	}
	if cached {
		filter.query, filter.rows = query, rows
	}
	return rows
}

func (m *Model) prompt(kind, title, value string) tea.Cmd {
	m.showDialog(kind, title)
	m.input.SetValue(value)
	m.input.Placeholder = ""
	m.input.Prompt = "> "
	switch kind {
	case "commit":
		m.input.Placeholder = m.tr("Commit message")
		m.input.Prompt = ""
	case "stash":
		m.input.Placeholder = m.tr("Optional message")
		m.input.Prompt = ""
	}
	if m.isPicker() {
		m.input.Prompt = ""
		m.input.Placeholder = title
	}
	m.input.CursorEnd()
	m.pickerIndex = 0
	m.syncPicker()
	return m.input.Focus()
}

func (m *Model) updateModal(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if m.modal == "searchOptions" {
		if matchesShortcut(k, dialogKeys.Toggle) && m.input.Value() == "" {
			return m.selectPickerChoice()
		}
	}

	if m.confirmationDialog() {
		return m.updateConfirmation(k)
	}
	if m.isPicker() {
		if command := m.widgetWorkbenchCommand(k, commandFiles, commandPalette); command != "" {
			return m.executeCommand(command, "")
		}
	}
	if key.Matches(msg, dismissKey) {
		restore := m.cancelThemePreview()
		m.rememberCommitDraft()
		m.dismissDialog()
		return restore
	}
	if m.isPicker() {
		if m.hasPickerIgnoreToggle() && key.Matches(msg, pickerKeys.ToggleIgnore) {
			return m.togglePickerIgnore()
		}
		if key.Matches(msg, pickerKeys.Accept) {
			return m.selectPickerChoice()
		}
		if m.pickerKey(msg) {
			return m.previewSelectedTheme()
		}
	}

	if matchesShortcut(k, pickerKeys.Accept) && !m.isPicker() {
		if m.modal == "commit" && strings.TrimSpace(m.input.Value()) == "" {
			return nil
		}
		m.rememberCommitDraft()
		kind, value := m.modal, strings.TrimSpace(m.input.Value())
		m.dismissDialog()
		switch kind {
		case "commit":
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
		case "branch":
			if value != "" {
				return m.createBranch(value)
			}
		case "stash":
			args := []string{"stash", "push"}
			if m.stashOption != "" {
				args = append(args, m.stashOption)
			}
			if value != "" {
				args = append(args, "-m", value)
			}
			return m.gitOperationInRepository(m.stashRepository, "Stash", args...)
		case "newFile":
			return m.createEntry(value, false)
		case "pasteFile":
			return m.pasteEntry(value)
		case "newFolder":
			return m.createEntry(value, true)
		}
		return nil
	}
	return m.updateDialogInput(msg)
}

func (m *Model) updateDialogInput(msg tea.Msg) tea.Cmd {
	before := m.input.Value()
	cmd := m.input.update(msg)
	if m.input.Value() != before {
		m.syncQuickPicker()
		m.selectPickerIndex(0)
	}
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
	return m.startOperationResult(title, func() operationMsg {
		_, err := run()
		return operationMsg{err: err}
	})
}

func (m *Model) startOperationResult(title string, run func() operationMsg) tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation")
		return nil
	}
	m.busy = true
	m.notice = ""
	m.statusFeedback = nil
	m.operationProgress = m.tr(title) + "…"
	m.refreshSeq++
	if m.refreshCancel != nil {
		m.refreshCancel()
		m.refreshCancel = nil
	}
	m.loading = false
	for _, t := range m.tabs {
		if d := t.diff; d != nil {
			d.cancelLoad()
		}
	}
	fetchDone := m.autoFetch.done
	if fetchDone != nil {
		m.autoFetch.cancel()
	}
	return func() tea.Msg {
		if fetchDone != nil {
			<-fetchDone
		}
		result := run()
		result.title = title
		return result
	}
}

func (m *Model) gitOperationInRepository(target, title string, args ...string) tea.Cmd {
	if (title == "Pull" || title == "Stash") &&
		(target == "" && m.hasDirtyTabs() || target != "" && m.hasDirtyTabsInRepository(target)) {
		m.notice = m.tr("Save or close edited files before this operation")
		return nil
	}
	return m.operation(title, func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.Run(m.ctx, args...)
	})
}

func (m *Model) fetch(target string) tea.Cmd {
	return m.showRemoteProgress(m.gitOperationInRepository(target, "Fetch", "fetch", "--all", "--prune"), target, "sync")
}

func (m *Model) pull(target string) tea.Cmd {
	return m.showRemoteProgress(m.gitOperationInRepository(target, "Pull", "pull", "--ff-only"), target, "pull")
}

func (m *Model) push(target string) tea.Cmd {
	return m.showRemoteProgress(m.gitOperationInRepository(target, "Push", "push"), target, "push")
}

func (m *Model) refreshView() tea.Cmd {
	var reload tea.Cmd
	if !m.sidebar && m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		if t := &m.tabs[m.activeTab]; t.diff == nil {
			reload = m.reloadWatchedFiles([]string{t.path()})
		}
	}
	if m.editorOnly {
		return reload
	}
	if m.sidebar && m.section == search {
		return m.runSearch()
	}
	if m.fileHistory != nil && m.section == explorer {
		m.setFileHistory(m.fileHistory.path, m.fileHistory.anchor, m.fileHistory.focused)
	}
	if !m.snapshot.NoRepository {
		if m.sidebar && m.section == pullRequests {
			return m.forceRefreshPullRequests()
		}
		if !m.sidebar && m.diffOpen && m.diffPR.Number != 0 {
			return m.fetchPullRequestDiff(m.pullRequestReviewFor(m.diffPR), true)
		}
	}
	if !m.busy {
		return tea.Batch(reload, m.refresh())
	}
	return reload
}

func (m *Model) changesKey(k string) (bool, tea.Cmd) {
	switch {
	case matchesShortcut(k, gitKeys.Stage):
		if entry := m.current(); entry != nil {
			id := "stage"
			if entry.staged {
				id = "unstage"
			}
			return true, m.runChangeRowAction(m.selected, id)
		}
		return true, nil
	case matchesShortcut(k, gitKeys.StageScope):
		return true, m.stageRepository(m.selectedRepository(), m.stageScopeUnstages(m.current()))
	case matchesShortcut(k, gitKeys.Commit):
		return true, m.commit()
	case matchesShortcut(k, gitKeys.Discard):
		if entry := m.current(); entry != nil && !entry.staged && !entry.repoHeader {
			return true, m.runChangeRowAction(m.selected, "discard")
		}
		return true, nil
	case matchesShortcut(k, gitKeys.DiscardScope):
		entry := m.current()
		if reason := m.discardScopeReason(entry); reason != "" {
			m.notice = reason
			return true, nil
		}
		return true, m.runScopeAction(item{repoHeader: true, repository: entry.repository}, "discard")
	case matchesShortcut(k, gitKeys.Stash):
		return true, m.stash(m.selectedRepository())
	case matchesShortcut(k, gitKeys.Fetch):
		return true, m.fetch(m.selectedRepository())
	case matchesShortcut(k, gitKeys.Pull):
		return true, m.pull(m.selectedRepository())
	case matchesShortcut(k, gitKeys.Push):
		return true, m.push(m.selectedRepository())
	default:
		return false, nil
	}
}

func (m *Model) stageFileReason(c gitservice.Change, unstage bool) string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
	}
	if !unstage && c.GitlinkDirty {
		return m.tr("Submodule has file changes; stage and commit them in its repository")
	}
	if !unstage && m.fileDirty(c.Path) {
		return m.tr("Save this file before staging")
	}
	if unstage && !c.Staged() {
		return m.tr("This file has no staged changes")
	}
	if !unstage && !c.Unstaged() {
		return m.tr("This file has no unstaged changes")
	}
	return ""
}

func (m *Model) stageFile(c gitservice.Change, unstage bool) tea.Cmd {
	if reason := m.stageFileReason(c, unstage); reason != "" {
		m.notice = reason
		return nil
	}
	title := m.tr("Stage %s", c.Path)
	if unstage {
		title = m.tr("Unstage %s", c.Path)
	}
	return m.stageChanges([]gitservice.Change{c}, unstage, title)
}

func (m *Model) stageChanges(scope []gitservice.Change, unstage bool, title string) tea.Cmd {
	return m.indexOperation(scope, unstage, title, func() error {
		return m.repo.StageChanges(m.ctx, scope, unstage)
	})
}

func (m *Model) indexOperation(scope []gitservice.Change, unstage bool, title string, apply func() error) tea.Cmd {
	if m.snapshot.NoRepository {
		m.notice = m.tr("This folder is not a Git repository")
		return nil
	}
	sources := make(map[[2]string]*diffState)
	for _, t := range m.tabs {
		if d := t.diff; d != nil && d.diffChange != nil && d.diffStaged == unstage {
			sources[[2]string{d.diffRepository, d.diffPath}] = d
		}
	}
	var affected []*diffState
	for _, c := range scope {
		if d := sources[[2]string{c.Repository, c.Path}]; d != nil {
			affected = append(affected, d)
		}
	}
	return m.startOperationResult(title, func() operationMsg {
		if err := apply(); err != nil {
			return operationMsg{err: err}
		}
		return operationMsg{indexDiffs: affected}
	})
}

func (m *Model) stageScopeUnstages(entry *item) bool {
	if entry == nil || !entry.repoHeader {
		return entry != nil && entry.staged
	}
	staged := false
	for _, c := range m.snapshot.Changes {
		if c.Repository == entry.repository {
			if c.Unstaged() && !c.GitlinkDirty {
				return false
			}
			staged = staged || c.Staged()
		}
	}
	return staged
}

func (m *Model) stageRepository(target string, unstage bool) tea.Cmd {
	entry := item{repoHeader: true, repository: target, staged: unstage}
	if unstage {
		return m.runScopeAction(entry, "unstage")
	}
	return m.runScopeAction(entry, "stage")
}

func (m *Model) repositoryStageReason(target string, unstage bool) string {
	entry := item{repoHeader: true, repository: target, staged: unstage}
	id := "stage"
	if unstage {
		id = "unstage"
	}
	return m.changeActionDisabled(entry, id, m.scopedChanges(entry))
}

func (m *Model) stageDiffLines(reverse bool, title string, selected func(*diff.Line) bool) tea.Cmd {
	if reason := m.partialStageReason(reverse, true); reason != "" {
		m.notice = reason
		return nil
	}
	c, source, workspace := *m.diffChange, m.patch, m.repo
	return m.indexOperation([]gitservice.Change{c}, reverse, title, func() error {
		current, err := workspace.Diff(m.ctx, c, reverse)
		if err != nil {
			return err
		}
		if current != source.Raw {
			return fmt.Errorf("Diff changed; refresh before staging")
		}
		repo, local, err := workspace.ForChange(m.ctx, c)
		if err != nil {
			return err
		}
		patch, err := source.StagingPatch(local.Path, reverse, selected)
		if err != nil {
			return err
		}
		return repo.ApplyIndexPatch(m.ctx, patch)
	})
}

func (m *Model) discardDiffLines(title string, selected func(*diff.Line) bool) tea.Cmd {
	if reason := m.partialDiscardReason(true); reason != "" {
		m.notice = reason
		return nil
	}
	c, source, workspace := *m.diffChange, m.patch, m.repo
	return m.askDiscard(title, func() tea.Cmd {
		if m.fileDirty(c.Path) {
			m.notice = m.tr("Save or close edited files before discarding")
			return nil
		}
		return m.operation("Discard", func() (string, error) {
			return workspace.DiscardLines(m.ctx, c, source, selected)
		})
	})
}

func (m *Model) commit() tea.Cmd { return m.commitInRepository(m.selectedRepository()) }

func (m *Model) openSelected() tea.Cmd {
	if state := m.repositoryEmptyState(); state.action != nil {
		return state.action()
	}
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	return m.activateSidebarEntry(*entry, false)
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
	if m.diffPR.Number != 0 {
		m.notice = m.tr("PR diffs are read-only")
		return nil
	}
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

func (m *Model) discardFileReason(c gitservice.Change) string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
	}
	if c.Gitlink {
		return m.tr("Cannot discard a repository directory as a file")
	}
	if c.Conflicted() || !c.Unstaged() {
		return m.tr("Select a non-conflicted working-tree file to discard")
	}
	if m.hasDirtyTabsInRepository(c.Repository) {
		return m.tr("Save or close edited files before discarding")
	}
	return ""
}

func (m *Model) discardFile(c gitservice.Change, expectedDiff string) tea.Cmd {
	if reason := m.discardFileReason(c); reason != "" {
		m.notice = reason
		return nil
	}
	title := m.tr("Permanently discard unstaged edits to %s?", c.Path)
	if c.Untracked() {
		title = m.tr("Delete untracked file %s?", c.Path)
	}
	cmd := m.askDiscard(title, func() tea.Cmd {
		if reason := m.discardFileReason(c); reason != "" {
			m.notice = reason
			return nil
		}
		return m.operation("Discard", func() (string, error) {
			if expectedDiff != "" {
				current, err := m.repo.Diff(m.ctx, c, false)
				if err != nil {
					return "", err
				}
				if current != expectedDiff {
					return "", fmt.Errorf("Diff changed; refresh before discarding")
				}
			}
			return m.repo.Discard(m.ctx, c)
		})
	})
	if expectedDiff != "" {
		m.confirmationButton = 1
	}
	if c.Untracked() {
		m.confirmationDetail = m.tr("This file cannot be recovered by Git.")
	}
	return cmd
}

func (m *Model) askDiscard(title string, run func() tea.Cmd) tea.Cmd {
	return m.ask(title, m.tr("Discard"), run)
}

func (m *Model) quit() tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation before quitting")
		return nil
	}
	if m.hasDirtyTabs() {
		return m.confirmLeavingWorkspace(m.tr("Save changes before quitting?"), func() tea.Cmd { return tea.Quit })
	}
	return tea.Quit
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
