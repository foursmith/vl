package app

import (
	"context"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/theme"
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

func (m *Model) previewHistory(entry item, reset bool) tea.Cmd {
	if entry.commit != nil {
		objectID := entry.commit.ID
		id := historyKey(entry.repository, objectID)
		changed := id != m.historyCommit
		m.historyCommit = id
		state := m.historyStates[id]
		if state == nil {
			state = &historyState{repository: entry.repository, expanded: true, collapsed: map[string]bool{}}
			m.historyStates[id] = state
		}
		if changed {
			m.closeDiff()
			m.showEditor = false
		}
		m.rebuildItems()
		if state.loaded {
			if (changed || !m.diffOpen) && m.historyExpanded(id) {
				return m.previewHistoryFirst(id)
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
	if entry.historyChange == nil {
		return nil
	}
	if !reset && !m.diffOpen {
		return nil
	}
	c := *entry.historyChange
	id := strings.TrimPrefix(entry.historyID, repositoryPrefix(entry.repository))
	m.historyCommit, m.diffCommit = entry.historyID, id
	m.diffChange = nil
	m.diffStaged = false
	return m.requestDiff(c.Path, reset, func(ctx context.Context) (string, error) {
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
		m.log(msg.err.Error())
		return nil
	}
	state.files, state.loaded = msg.files, true
	if m.section != history {
		return nil
	}
	m.rebuildItems()
	entry := m.current()
	if m.showEditor || !m.historyExpanded(key) || key != m.historyCommit || entry == nil || entry.commit == nil || historyKey(entry.repository, entry.commit.ID) != key {
		return nil
	}
	return m.previewHistoryFirst(key)
}

func (m *Model) previewHistoryFirst(id string) tea.Cmd {
	for _, entry := range m.historyTree(id) {
		if entry.historyChange != nil {
			return m.previewHistory(entry, true)
		}
	}
	m.notice = "This commit has no visible file changes"
	return nil
}

func (m *Model) openHistory(entry item) tea.Cmd {
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
		return m.previewHistory(entry, true)
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
	m.sidebar = false
	return m.previewHistory(entry, true)
}

func (m *Model) selectHistoryParent(entry item) {
	if entry.repoHeader {
		return
	}
	if entry.commit != nil {
		for i, row := range m.items {
			if row.repoHeader && row.repository == entry.repository {
				m.selected = i
				m.ensureSidebar()
				return
			}
		}
		return
	}
	path := entry.folder
	if entry.historyChange != nil {
		path = entry.historyChange.Path
	}
	for i := m.selected - 1; i >= 0; i-- {
		row := m.items[i]
		if row.historyID == entry.historyID && row.folder != "" && strings.HasPrefix(path, row.folder+"/") || row.commit != nil && historyKey(row.repository, row.commit.ID) == entry.historyID {
			m.selected = i
			m.ensureSidebar()
			return
		}
	}
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
	badges := ""
	for _, ref := range m.historyData(entry.repository).Refs {
		if ref.OID == c.ID {
			badges += " [" + safe(ref.Name) + "]"
		}
	}
	indent := strings.Repeat("  ", entry.depth)
	w = max(0, w-len(indent))
	badgeWidth := min(ansi.StringWidth(badges), max(0, w/2))
	label := ansi.Truncate(safe(c.Subject), max(0, w-3-badgeWidth), "…")
	text := theme.Style(m.colors.Accent, bg).Render(marker+" ") + label
	text += theme.Style(m.colors.Accent, bg).Render(ansi.Truncate(badges, badgeWidth, "…"))
	return indent + text, m.colors.Foreground
}

func historyKey(repo, id string) string       { return repositoryPrefix(repo) + id }
func historyRepositoryKey(repo string) string { return "history:" + repositoryKey(repo) }

func (m *Model) historyData(repo string) gitservice.CommitHistory {
	if repo == "" {
		return gitservice.CommitHistory{Branch: m.snapshot.Branch, Commits: m.snapshot.Commits, Refs: m.snapshot.Refs}
	}
	return m.snapshot.RepositoryHistory[repo]
}

func (m *Model) historyRepositoryItems() []item {
	var result []item
	for _, repo := range append([]string{""}, m.snapshot.Repositories...) {
		header := m.repositoryHeader(repo, historyRepositoryKey(repo))
		result = append(result, header)
		if m.changeCollapsed[header.key] {
			continue
		}
		data := m.historyData(repo)
		if data.Error != "" {
			result = append(result, item{key: historyRepositoryKey(repo) + ":error", label: data.Error, repository: repo})
		}
		if len(data.Commits) == 0 && data.Error == "" {
			result = append(result, item{key: historyRepositoryKey(repo) + ":empty", label: "暂无提交", repository: repo})
		}
		for i := range data.Commits {
			c := &data.Commits[i]
			key := historyKey(repo, c.ID)
			result = append(result, item{label: c.Subject, key: key, commit: c, historyID: key, repository: repo, depth: 1})
			if m.historyExpanded(key) {
				result = append(result, m.historyTree(key)...)
			}
		}
	}
	return result
}

func (m *Model) reconcileHistoryRepositories() {
	present := map[string]bool{"": true}
	for _, repo := range m.snapshot.Repositories {
		present[repo] = true
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
			if m.diffCommit != "" {
				m.closeDiff()
			}
		}
	}
}
