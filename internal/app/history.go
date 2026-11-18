package app

import (
	"context"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/theme"
)

type historyFilesMsg struct {
	repository string
	seq        int
	id         string
	files      []gitservice.Change
	err        error
}

type historyState struct {
	repository      string
	files           []gitservice.Change
	expanded        bool
	collapsed       map[string]bool
	loaded, loading bool
	seq             int
	cancel          context.CancelFunc
}

func (m *Model) historyExpanded(id string) bool {
	state := m.historyStates[id]
	return state != nil && state.expanded
}

func (m *Model) historyTree(id string) []item {
	state := m.historyStates[id]
	if state == nil {
		return nil
	}
	rows, _ := changeTree(state.files, state.repository, true, state.collapsed, id+":")
	for i := range rows {
		rows[i].historyID = id
		rows[i].historyChange, rows[i].change = rows[i].change, nil
	}
	return rows
}

func (m *Model) loadHistoryCommit(entry item, keep bool) tea.Cmd {
	objectID := entry.commit.ID
	id := historyKey(entry.repository, objectID)
	changed := id != m.historyCommit
	m.historyCommit = id
	state := m.historyStates[id]
	if state == nil {
		state = &historyState{repository: entry.repository, expanded: true, collapsed: map[string]bool{}}
		m.historyStates[id] = state
	}
	m.pendingHistory, m.keepPendingHistory = id, keep
	m.rebuildItems()
	if state.loaded {
		if (keep || changed || !m.diffOpen || m.diffCommit != objectID || m.diffRepository != entry.repository) && m.historyExpanded(id) {
			return m.openHistoryFirst(id, keep)
		}
		return nil
	}
	if state.loading {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.historySeq++
	seq := m.historySeq
	state.seq, state.cancel, state.loading = seq, cancel, true
	workspace, repoPath := m.repo, entry.repository
	return func() tea.Msg {
		defer cancel()
		repo, err := workspace.RepositoryAt(ctx, repoPath)
		var files []gitservice.Change
		if err == nil {
			files, err = repo.CommitFiles(ctx, objectID)
		}
		for i := range files {
			files[i].Repository = repoPath
			files[i].Path = filepath.ToSlash(filepath.Join(repoPath, files[i].Path))
			if files[i].Original != "" {
				files[i].Original = filepath.ToSlash(filepath.Join(repoPath, files[i].Original))
			}
		}
		return historyFilesMsg{repository: repoPath, seq: seq, id: objectID, files: files, err: err}
	}
}

func (m *Model) requestHistoryDiff(entry item) tea.Cmd {
	c := *entry.historyChange
	id := strings.TrimPrefix(entry.historyID, repositoryPrefix(entry.repository))
	m.historyCommit = entry.historyID
	return m.requestDiff(diffTarget{path: c.Path, repository: c.Repository, commit: id, change: &c, preview: true}, func(ctx context.Context) (string, error) {
		return m.repo.CommitFileDiff(ctx, id, c)
	})
}

func (m *Model) applyHistoryFiles(msg historyFilesMsg) tea.Cmd {
	key := historyKey(msg.repository, msg.id)
	state := m.historyStates[key]
	if state == nil || !state.loading || msg.seq != state.seq {
		return nil
	}
	state.loading = false
	state.cancel = nil
	if msg.err != nil {
		m.log(m.tr(msg.err.Error()))
		return nil
	}
	state.files, state.loaded = msg.files, true
	if m.section != history {
		return nil
	}
	m.rebuildItems()
	entry := m.current()
	if m.pendingHistory != key || !m.historyExpanded(key) || key != m.historyCommit || entry == nil || entry.commit == nil || historyKey(entry.repository, entry.commit.ID) != key {
		return nil
	}
	return m.openHistoryFirst(key, m.keepPendingHistory)
}

func (m *Model) openHistoryFirst(id string, keep bool) tea.Cmd {
	for _, entry := range m.historyTree(id) {
		if entry.historyChange != nil {
			return m.openSidebarEntry(entry, !keep)
		}
	}
	m.notice = m.tr("This commit has no visible file changes")
	return nil
}

func (m *Model) openHistoryRow(entry item, keep bool) tea.Cmd {
	if entry.historyPage != nil {
		return m.loadHistoryPage(entry.historyPage)
	}
	if entry.repoHeader {
		m.changeCollapsed[entry.key] = !m.changeCollapsed[entry.key]
		m.rebuildItems()
		return nil
	}
	if entry.commit != nil {
		key := historyKey(entry.repository, entry.commit.ID)
		if m.historyExpanded(key) {
			m.historyStates[key].expanded = false
			m.rebuildItems()
			return nil
		}
		if state := m.historyStates[key]; state != nil {
			state.expanded = true
		}
		return m.loadHistoryCommit(entry, keep)
	}
	if entry.folder != "" {
		state := m.historyStates[entry.historyID]
		if state == nil {
			return nil
		}
		state.collapsed[entry.key] = !state.collapsed[entry.key]
		m.rebuildItems()
		return nil
	}
	if entry.historyChange == nil {
		return nil
	}
	return m.openSidebarEntry(entry, !keep)
}

func (m *Model) historyRow(entry item, w int, bg string) (string, string) {
	if entry.repoHeader {
		return m.explorerRow(entry, w, bg)
	}
	if entry.commit == nil && entry.historyChange == nil && entry.folder == "" {
		return row("   "+safe(entry.label), w, m.colors.Muted, bg), m.colors.Muted
	}
	if entry.commit == nil {
		indent := "  "
		text, fg := m.explorerRow(entry, max(0, w-2-len(indent)), bg)
		return indent + theme.Style(m.colors.Accent, bg).Render("│ ") + text, fg
	}
	c := entry.commit
	marker := "●"
	if !m.nerd {
		marker = "*"
	}
	if m.historyExpanded(historyKey(entry.repository, c.ID)) {
		marker = "◉"
		if !m.nerd {
			marker = "o"
		}
	}
	badges, tags := "", ""
	for _, ref := range m.repositoryState(entry.repository).Refs {
		if ref.TargetOID != c.ID {
			continue
		}
		if ref.Kind == gitservice.Tag {
			tags += theme.Style(m.colors.GitModified, m.colors.Input).Bold(true).Render(" "+safe(ref.Name)+" ") + " "
		} else {
			badges += " [" + safe(ref.Name) + "]"
		}
	}
	indent := strings.Repeat("  ", entry.depth)
	w = max(0, w-len(indent))
	tagWidth := min(ansi.StringWidth(tags), max(0, w/2))
	tags = ansi.Truncate(tags, tagWidth, "…")
	badgeWidth := min(ansi.StringWidth(badges), max(0, (w-tagWidth)/2))
	label := ansi.Truncate(safe(c.Subject), max(0, w-3-tagWidth-badgeWidth), "…")
	text := theme.Style(m.colors.Accent, bg).Render(marker+" ") + tags + label
	text += theme.Style(m.colors.Accent, bg).Render(ansi.Truncate(badges, badgeWidth, "…"))
	return indent + text, m.colors.Foreground
}

func historyKey(repo, id string) string       { return repositoryPrefix(repo) + id }
func historyRepositoryKey(repo string) string { return "history:" + repositoryKey(repo) }

func (m *Model) historyRepositoryItems() []item {
	var result []item
	for _, repo := range append([]string{""}, m.snapshot.Repositories...) {
		header := m.repositoryHeader(repo, historyRepositoryKey(repo))
		result = append(result, header)
		if m.changeCollapsed[header.key] {
			continue
		}
		data := m.repositoryState(repo)
		if data.MetadataError != "" {
			result = append(result, item{key: historyRepositoryKey(repo) + ":error", label: data.MetadataError, repository: repo})
		}
		if len(data.Commits) == 0 && data.MetadataError == "" {
			result = append(result, item{key: historyRepositoryKey(repo) + ":empty", label: m.tr("No commits yet"), repository: repo})
		}
		commits := data.Commits
		var page *historyPageState
		if len(commits) > 0 && data.MetadataError == "" {
			page = m.historyPage(repo, commits)
			commits = page.commits
		}
		for i := range commits {
			c := &commits[i]
			key := historyKey(repo, c.ID)
			result = append(result, item{label: c.Subject, key: key, commit: c, historyID: key, repository: repo, depth: 1})
			if m.historyExpanded(key) {
				result = append(result, m.historyTree(key)...)
			}
		}
		if page != nil && !page.done {
			label := m.tr("Load more commits…")
			if page.err != "" {
				label = m.tr("Failed to load; press Enter to retry: %s", page.err)
			}
			result = append(result, item{key: historyRepositoryKey(repo) + ":more", repository: repo, label: label, historyPage: page})
		}
	}
	return result
}

func (m *Model) reconcileHistoryRepositories() {
	present := map[string]bool{"": true}
	for _, repo := range m.snapshot.Repositories {
		present[repo] = true
	}
	for repo, page := range m.historyPages {
		if !present[repo] {
			if page.cancel != nil {
				page.cancel()
			}
			delete(m.historyPages, repo)
		}
	}
	for key, state := range m.historyStates {
		if present[state.repository] {
			continue
		}
		if state.cancel != nil {
			state.cancel()
		}
		delete(m.historyStates, key)
		if m.historyCommit == key {
			m.historyCommit = ""
		}
		if m.pendingHistory == key {
			m.pendingHistory = ""
		}
	}
	for i := len(m.tabs) - 1; i >= 0; i-- {
		if d := m.tabs[i].diff; d != nil && d.diffCommit != "" && !present[d.diffRepository] {
			m.removeTabAt(i)
		}
	}
}
