package app

import (
	"context"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/theme"
)

type fileHistoryState struct {
	path, anchor, head string
	next               gitservice.FileHistoryCursor
	revisions          []gitservice.FileRevision
	cancel             context.CancelFunc
	err                error
	selected, top      int
	started, focused   bool
	pending            *diffState
	keepPending        bool
}

type fileHistoryMsg struct {
	target    *fileHistoryState
	revisions []gitservice.FileRevision
	next      gitservice.FileHistoryCursor
	err       error
}

func (m *Model) fileHistoryPath() string {
	if m.fileHistoryHasFocus() {
		return m.fileHistory.path
	}
	if m.sidebar {
		if entry := m.sidebarEntry(); entry != nil && entry.isSidebarFile() {
			return entryPath(entry)
		}
		return ""
	}
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab].path()
	}
	return ""
}

func (m *Model) fileHistoryAction(path string) action {
	return action{id: "git.fileHistory", label: m.tr("Show File History"), run: func() tea.Cmd {
		return m.openFileHistory(path)
	}}
}

func (m *Model) fileHistoryAnchor() string {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return ""
	}
	return m.tabs[m.activeTab].clickKey()
}

func (m *Model) workspaceHistoryPath(path string) string {
	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(m.repo.Root, path)
		if err != nil {
			return ""
		}
		path = relative
	}
	if !filepath.IsLocal(path) || filepath.Clean(path) == "." {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func (m *Model) openFileHistory(path string) tea.Cmd {
	path = m.workspaceHistoryPath(path)
	if path == "" {
		m.notice = m.tr("Select a file in the workspace")
		return nil
	}
	var explorerCmd tea.Cmd
	if m.section != explorer {
		explorerCmd = m.selectSection(explorer)
	}
	m.sidebar, m.sidebarVisible = true, true
	m.setExplorerPanel(explorerPanelTimeline)
	anchor := m.fileHistoryAnchor()
	if m.fileHistory == nil || m.fileHistory.path != path {
		m.setFileHistory(path, anchor, true)
	} else {
		m.fileHistory.anchor, m.fileHistory.focused = anchor, true
	}
	m.resize()
	m.clampSidebarScroll()
	return tea.Batch(explorerCmd, m.syncFileHistory())
}

func (m *Model) syncFileHistory() tea.Cmd {
	if m.editorOnly || m.gitUnavailable || m.section != explorer {
		return nil
	}
	anchor := m.fileHistoryAnchor()
	state := m.fileHistory
	var path string
	if state != nil && state.anchor == anchor {
		path = state.path // Keep the source file while previewing its earlier names.
	} else {
		if state != nil {
			state.cancelDiff()
		}
		if anchor != "" {
			path = m.workspaceHistoryPath(m.tabs[m.activeTab].path())
			if m.snapshot.NoRepository && m.fileRepository(path) == "" {
				path = ""
			}
		}
	}
	if path == "" {
		if m.fileHistory != nil {
			m.stopFileHistory()
			m.clampSidebarScroll()
		}
		return nil
	}
	head := m.repositoryState(m.fileRepository(path)).Head
	if state == nil || state.path != path || state.head != head {
		focused := state != nil && state.path == path && state.focused
		m.setFileHistory(path, anchor, focused)
		m.clampSidebarScroll()
	} else {
		state.anchor = anchor
	}
	if !m.fileHistory.started && m.sidebarVisible && (m.sideShown() || m.sidebar) {
		if _, list := m.fileHistoryLayout(); list.height > 0 {
			return m.loadFileHistory()
		}
	}
	return nil
}

func (m *Model) setFileHistory(path, anchor string, focused bool) {
	m.stopFileHistory()
	start := path
	for _, change := range m.snapshot.Changes {
		if change.Path == path && change.Original != "" && (change.Index == 'R' || change.Worktree == 'R') {
			start = change.Original
			break
		}
	}
	m.fileHistory = &fileHistoryState{
		path: path, anchor: anchor, focused: focused,
		head: m.repositoryState(m.fileRepository(path)).Head,
		next: gitservice.FileHistoryCursor{Revision: "HEAD", Path: start},
	}
}

func (state *fileHistoryState) cancelDiff() {
	if state.pending != nil {
		state.pending.close()
		state.pending = nil
	}
}

func (m *Model) stopFileHistory() {
	if state := m.fileHistory; state != nil {
		state.cancelDiff()
		if state.cancel != nil {
			state.cancel()
		}
	}
	m.fileHistory = nil
}

func (m *Model) blurFileHistory() {
	if state := m.fileHistory; state != nil {
		state.focused = false
		state.cancelDiff()
	}
}

func (m *Model) loadFileHistory() tea.Cmd {
	state := m.fileHistory
	if state == nil || state.cancel != nil || state.next.Revision == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	state.started, state.cancel, state.err = true, cancel, nil
	repo, cursor := m.repo, state.next
	return func() tea.Msg {
		defer cancel()
		revisions, next, err := repo.FileHistory(ctx, cursor)
		return fileHistoryMsg{state, revisions, next, err}
	}
}

func (m *Model) applyFileHistory(msg fileHistoryMsg) {
	state := m.fileHistory
	if state == nil || state != msg.target || state.cancel == nil {
		return
	}
	state.cancel, state.err = nil, msg.err
	if msg.err == nil {
		state.revisions = append(state.revisions, msg.revisions...)
		state.next = msg.next
	}
	_, l := m.fileHistoryLayout()
	if l.height > 0 {
		state.selected = min(state.selected, l.count-1)
		l.clampScroll()
	}
}

func (m *Model) fileHistoryLayout() (int, sidebarList) {
	return m.explorerPanelLayout(explorerPanelTimeline)
}

func (m *Model) fileHistoryList() sidebarList {
	state := m.fileHistory
	count := len(state.revisions)
	if count == 0 || state.cancel != nil || state.next.Revision != "" || state.err != nil {
		count++
	}
	return sidebarList{selected: &state.selected, top: &state.top, count: count}
}

func (m *Model) fileHistoryAvailable() bool { return m.fileHistory != nil }

func (m *Model) fileHistoryPanelFocused() bool {
	return m.fileHistory != nil && m.fileHistory.focused
}

func (m *Model) fileHistoryHasFocus() bool {
	if !m.sidebar || !m.fileHistoryPanelFocused() {
		return false
	}
	header, _ := m.fileHistoryLayout()
	return header < m.bodyHeight()
}

func (m *Model) fileHistoryHoverHint() string {
	y := m.explorerPanelHoverY()
	if y < headerHeight {
		return ""
	}
	panel, ok := m.explorerPanelAt(y)
	if !ok || panel.spec.id != explorerPanelTimeline {
		return ""
	}
	header, l := panel.header, panel.list
	state := m.fileHistory
	if y == header {
		return safe(state.path) + " · " + shortcutHint(m.tr("Focus Timeline"), sidebarKeys.FocusFileHistory)
	}
	index := min(*l.top, max(0, l.count-l.height)) + y - l.start
	if index < len(state.revisions) {
		return safe(state.revisions[index].Subject)
	}
	if index < l.count && state.err != nil {
		return safe(state.err.Error())
	}
	return ""
}

func (m *Model) appendFileHistory(lines []string, width int, l sidebarList) []string {
	if l.height == 0 {
		return lines
	}
	state := m.fileHistory
	hover := m.explorerPanelHoverY()
	focused := m.sidebar && state.focused
	top := min(*l.top, max(0, l.count-l.height))
	for i := top; i < min(l.count, top+l.height); i++ {
		bg := m.colors.Panel
		switch {
		case i == state.selected && focused:
			bg = m.colors.SidebarSelection(true)
		case l.start+i-top == hover:
			bg = m.colors.Hover
		case i < len(state.revisions) && m.diffOpen && m.diffCommit == state.revisions[i].ID && m.diffPath == state.revisions[i].Change.Path:
			bg = m.colors.SidebarSelection(false)
		}
		var text string
		if i < len(state.revisions) {
			text = m.fileHistoryRow(state.revisions[i], width, bg)
		} else {
			text = "   " + m.fileHistoryMessage(state)
		}
		if i == state.selected && focused && m.monochrome {
			text = ">" + ansi.Cut(text, 1, width)
		}
		lines = append(lines, row(text, width, theme.Readable(m.colors.Muted, bg), bg))
	}
	return lines
}

func (m *Model) fileHistoryMessage(state *fileHistoryState) string {
	switch {
	case state.err != nil:
		return m.tr("Couldn't load history. Press Enter to retry.")
	case !state.started || state.cancel != nil:
		return m.tr("Loading file history…")
	case state.next.Revision != "":
		return m.tr("Load more commits…")
	default:
		return m.tr("No commit history for this file")
	}
}

func (m *Model) fileHistoryRow(revision gitservice.FileRevision, width int, bg string) string {
	date := m.fileHistoryDate(revision.Date)
	dateWidth := ansi.StringWidth(date) + 2
	if width-dateWidth < 12 {
		date, dateWidth = "", 0
	}
	marker := " ○ "
	if !m.nerd {
		marker = " o "
	}
	subjectWidth := max(0, width-dateWidth-ansi.StringWidth(marker))
	subject := row(ansi.Truncate(safe(revision.Subject), subjectWidth, "…"), subjectWidth, theme.Readable(m.colors.SideForeground, bg), bg)
	text := theme.TextStyle(theme.Readable(m.colors.Muted, bg), bg).Render(marker) + subject
	if dateWidth > 0 {
		text += row(" "+date+" ", dateWidth, theme.Readable(m.colors.Muted, bg), bg)
	}
	return text
}

func (m *Model) clickFileHistory(y int, l sidebarList) tea.Cmd {
	state := m.fileHistory
	l.clampScroll()
	state.focused = true
	state.selected = *l.top + y - l.start
	keep := state.selected < len(state.revisions) && m.doubleClick("timeline:"+state.path+":"+state.revisions[state.selected].ID)
	return m.openFileHistoryRevision(keep)
}

func (m *Model) openFileHistoryRevision(keep bool) tea.Cmd {
	state := m.fileHistory
	if state.selected >= len(state.revisions) {
		return m.loadFileHistory()
	}
	revision := state.revisions[state.selected]
	change := revision.Change
	target := diffTarget{path: change.Path, repository: change.Repository, commit: revision.ID}
	for i, t := range m.tabs {
		d := t.diff
		if d == nil || !d.matches(target) || d.patch.Raw == "" && (i != m.activeTab || !d.diffLoading) {
			continue
		}
		state.cancelDiff()
		if i != m.activeTab {
			m.selectTab(i)
			state.anchor = m.fileHistoryAnchor()
		}
		if keep {
			m.focusEditor()
		}
		return nil
	}
	if state.pending != nil && state.pending.matches(target) {
		state.keepPending = state.keepPending || keep
		return nil
	}
	state.cancelDiff()
	// Prepare the diff offscreen so the editor or previous diff stays visible
	// while Git reads the selected revision.
	d := newDiffState()
	d.diffOpen = true
	d.diffPath, d.diffRepository, d.diffCommit = target.path, target.repository, target.commit
	d.diffOriginal = change.Original
	workspace := m.repo
	d.read = func(ctx context.Context) (string, error) {
		return workspace.CommitFileDiff(ctx, revision.ID, change)
	}
	state.pending, state.keepPending = d, keep
	return m.loadDiff(d, false)
}

func (m *Model) finishFileHistoryDiff(msg diffMsg) tea.Cmd {
	state := m.fileHistory
	d := state.pending
	if msg.seq != d.diffSeq {
		return nil
	}
	anchor := m.fileHistoryAnchor()
	keep := state.keepPending
	if state.anchor != anchor || msg.err != nil || msg.raw == "" {
		state.cancelDiff()
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
		}
		return nil
	}
	state.pending = nil
	index := m.insertTab(tab{diff: d, preview: !keep})
	m.selectTab(index)
	state.anchor = m.fileHistoryAnchor()
	if keep && m.modal == "" && m.contextMenu == nil {
		m.focusEditor()
	}
	return m.finishDiff(msg)
}

func (m *Model) fileHistoryKey(k string) (tea.Cmd, bool) {
	header, l := m.fileHistoryLayout()
	if header == m.bodyHeight() {
		return nil, false
	}
	state := m.fileHistory
	switch {
	case matchesShortcut(k, sidebarKeys.FocusFileHistory):
		state.focused = !state.focused
		if state.focused {
			m.setExplorerPanel(explorerPanelTimeline)
			_, l = m.fileHistoryLayout()
			if l.count > 0 {
				l.ensureSelected()
			}
		}
		m.navigationState = navigationState{}
		return nil, true
	case !state.focused:
		return nil, false
	case k == "esc":
		m.blurFileHistory()
		return nil, true
	case matchesShortcut(k, navigationKeys.Left):
		state.cancelDiff()
		m.explorerPanel = explorerPanelNone
		m.clampSidebarScroll()
		return nil, true
	case matchesShortcut(k, sidebarKeys.Open), matchesShortcut(k, navigationKeys.Right):
		if l.count > 0 {
			return m.openFileHistoryRevision(true), true
		}
		m.setExplorerPanel(explorerPanelTimeline)
		return nil, true
	case l.count == 0:
		return nil, false
	}
	action, count, handled := m.navigationState.key("timeline:"+state.path, k)
	if handled && l.move(action, count) {
		l.ensureSelected()
		l.clampScroll()
		if state.selected < len(state.revisions) {
			return m.openFileHistoryRevision(false), true
		}
	}
	return nil, handled
}

func (m *Model) fileHistoryDate(date time.Time) string {
	age := time.Since(date)
	switch {
	case age < time.Minute:
		return m.tr("now")
	case age < time.Hour:
		return m.tr("%dm", int(age/time.Minute))
	case age < 24*time.Hour:
		return m.tr("%dh", int(age/time.Hour))
	case age < 7*24*time.Hour:
		return m.tr("%dd", int(age/(24*time.Hour)))
	}
	return date.Local().Format("2006-01-02")
}
