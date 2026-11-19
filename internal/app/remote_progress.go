package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

var remoteProgressFrames = [...]string{"◜", "◝", "◞", "◟"}

type remoteProgress struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	target, action, hint string
	frame                int
}

type remoteProgressFrameMsg struct{ progress *remoteProgress }

func (p *remoteProgress) tick(delay time.Duration) tea.Cmd {
	return delayedMessage(p.ctx, delay, func(time.Time) tea.Msg {
		return remoteProgressFrameMsg{progress: p}
	})
}

func (m *Model) showRemoteProgress(cmd tea.Cmd, target, action string) tea.Cmd {
	if cmd == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	progress := &remoteProgress{ctx: ctx, cancel: cancel, target: target, action: action, hint: m.notice, frame: -1}
	m.remoteProgress = progress
	// Keep the normal icon for operations that finish almost immediately.
	return tea.Batch(cmd, progress.tick(150*time.Millisecond))
}
