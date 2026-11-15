package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/theme"
)

type historyFilesMsg struct {
	seq   int
	id    string
	files []gitservice.Change
	err   error
}

type historyState struct {
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
	rows, _ := changeTree(state.files, true, state.collapsed, id+":")
	for i := range rows {
		rows[i].historyID = id
		rows[i].historyChange, rows[i].change = rows[i].change, nil
	}
	return rows
}

func (m *Model) previewHistory(entry item, reset bool) tea.Cmd {
	if entry.commit != nil {
		id := entry.commit.ID
		changed := id != m.historyCommit
		m.historyCommit = id
		state := m.historyStates[id]
		if state == nil {
			state = &historyState{expanded: true, collapsed: map[string]bool{}}
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
		return func() tea.Msg {
			defer cancel()
			files, err := m.repo.CommitFiles(ctx, id)
			return historyFilesMsg{seq, id, files, err}
		}
	}
	if entry.historyChange == nil {
		return nil
	}
	if !reset && !m.diffOpen {
		return nil
	}
	c, id := *entry.historyChange, entry.historyID
	m.historyCommit, m.diffCommit = id, id
	m.diffChange = nil
	m.diffStaged = false
	return m.requestDiff(c.Path, reset, func(ctx context.Context) (string, error) {
		return m.repo.CommitFileDiff(ctx, id, c)
	})
}

func (m *Model) applyHistoryFiles(msg historyFilesMsg) tea.Cmd {
	state := m.historyStates[msg.id]
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
	if m.showEditor || !m.historyExpanded(msg.id) || msg.id != m.historyCommit || entry == nil || entry.commit == nil || entry.commit.ID != msg.id {
		return nil
	}
	return m.previewHistoryFirst(msg.id)
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
	if entry.commit != nil {
		if m.historyExpanded(entry.commit.ID) {
			m.historyStates[entry.commit.ID].expanded = false
			m.rebuildItems()
			return nil
		}
		if state := m.historyStates[entry.commit.ID]; state != nil {
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
	m.sidebar = false
	return m.previewHistory(entry, true)
}

func (m *Model) selectHistoryParent(entry item) {
	path := entry.folder
	if entry.historyChange != nil {
		path = entry.historyChange.Path
	}
	parent := ""
	if i := strings.LastIndex(path, "/"); i >= 0 {
		parent = path[:i]
	}
	for i, row := range m.items {
		if parent == "" && row.commit != nil && row.commit.ID == entry.historyID || parent != "" && row.historyID == entry.historyID && row.folder == parent {
			m.selected = i
			m.ensureSidebar()
			return
		}
	}
}

func (m *Model) historyRow(entry item, w int, bg string) (string, string) {
	if entry.commit == nil {
		text, fg := m.explorerRow(entry, max(0, w-2), bg)
		return theme.Style(theme.Accent, bg).Render("│ ") + text, fg
	}
	c := entry.commit
	marker := "●"
	if !m.nerd {
		marker = "*"
	}
	if m.historyExpanded(c.ID) {
		marker = "◉"
		if !m.nerd {
			marker = "o"
		}
	}
	badges := ""
	for _, ref := range m.snapshot.Refs {
		if ref.OID == c.ID {
			badges += " [" + safe(ref.Name) + "]"
		}
	}
	badgeWidth := min(ansi.StringWidth(badges), max(0, w/2))
	label := ansi.Truncate(safe(c.Subject), max(0, w-3-badgeWidth), "…")
	text := theme.Style(theme.Accent, bg).Render(marker+" ") + label
	text += theme.Style(theme.Accent, bg).Render(ansi.Truncate(badges, badgeWidth, "…"))
	return text, theme.Foreground
}
