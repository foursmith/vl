package app

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

const autoFetchInterval = 3 * time.Minute

type autoFetchFinishedMsg struct{ interrupted bool }

type autoFetchState struct {
	next   time.Time
	cancel context.CancelFunc
	done   chan struct{}
}

func (m *Model) startAutoFetch() tea.Cmd {
	if m.autoFetch.next.IsZero() || time.Now().Before(m.autoFetch.next) ||
		m.terminalBlurred || m.busy || m.openingRepository || m.nextRepository != nil || m.autoFetch.done != nil {
		return nil
	}

	var targets []string
	if len(m.snapshot.Remotes) > 0 {
		targets = append(targets, "")
	}
	for path, state := range m.snapshot.RepositoryStates {
		if len(state.Remotes) > 0 {
			targets = append(targets, path)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	workspace := m.repo
	ctx, cancel := context.WithCancel(m.ctx)
	done := make(chan struct{})
	m.autoFetch.cancel, m.autoFetch.done = cancel, done
	return func() tea.Msg {
		defer close(done)
		var requests sync.WaitGroup
		for _, target := range targets {
			requests.Go(func() {
				repo, err := workspace.RepositoryAt(ctx, target)
				if err == nil && repo.HasRepository() {
					repo.FetchInBackground(ctx)
				}
			})
		}
		requests.Wait()
		return autoFetchFinishedMsg{interrupted: ctx.Err() != nil}
	}
}

func (m *Model) finishAutoFetch(msg autoFetchFinishedMsg) {
	m.autoFetch.cancel()
	m.autoFetch.cancel, m.autoFetch.done = nil, nil
	if !msg.interrupted {
		m.autoFetch.next = time.Now().Add(autoFetchInterval)
	}
	if !m.busy {
		m.watchDirty = true
	}
}
