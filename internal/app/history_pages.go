package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
)

type historyPageState struct {
	repository    string
	commits       []gitservice.Commit
	loading, done bool
	err           string
	cancel        context.CancelFunc
}

type historyPageMsg struct {
	target  *historyPageState
	commits []gitservice.Commit
	err     error
}

func (m *Model) historyPage(repo string, initial []gitservice.Commit) *historyPageState {
	page := m.historyPages[repo]
	if page == nil || page.commits[0].ID != initial[0].ID {
		if page != nil && page.cancel != nil {
			page.cancel()
		}
		page = &historyPageState{repository: repo, commits: initial, done: len(initial) < gitservice.CommitPageSize}
		m.historyPages[repo] = page
	}
	return page
}

func (m *Model) loadHistoryPage(page *historyPageState) tea.Cmd {
	if m.historyPages[page.repository] != page || page.loading || page.done {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	page.loading, page.err, page.cancel = true, "", cancel
	head, offset, workspace := page.commits[0].ID, len(page.commits), m.repo
	return func() tea.Msg {
		defer cancel()
		repo, err := workspace.RepositoryAt(ctx, page.repository)
		var commits []gitservice.Commit
		if err == nil {
			commits, err = repo.CommitPage(ctx, head, offset)
		}
		return historyPageMsg{page, commits, err}
	}
}

func (m *Model) loadVisibleHistory() tea.Cmd {
	if m.section != history || m.modal != "" || !m.sidebarVisible || !m.sideShown() && !m.sidebar {
		return nil
	}
	var cmds []tea.Cmd
	end := min(len(m.items), m.sideTop+max(1, m.bodyHeight()-headerHeight))
	for i := m.sideTop; i < end; i++ {
		entry := m.items[i]
		if entry.historyPage != nil && entry.historyPage.err == "" {
			if cmd := m.loadHistoryPage(entry.historyPage); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) applyHistoryPage(msg historyPageMsg) {
	page := msg.target
	if page == nil || m.historyPages[page.repository] != page || !page.loading {
		return
	}
	page.loading, page.cancel = false, nil
	if msg.err != nil {
		page.err = msg.err.Error()
	} else {
		page.commits = append(page.commits, msg.commits...)
		page.done = len(msg.commits) < gitservice.CommitPageSize
	}
	if m.section == history {
		// Continue keyboard selection into the first new commit, not the next load row.
		if entry := m.current(); entry != nil && entry.historyPage == page && msg.err == nil && len(msg.commits) > 0 {
			entry.key = historyKey(page.repository, msg.commits[0].ID)
		}
		m.rebuildItems()
		m.clampSidebarScroll()
	}
}
