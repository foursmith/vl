package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

// A pending command also keeps its Bubble Tea program and model alive. Unlike
// tea.Tick, workspace delays must stop when their owner closes.
func delayedMessage(ctx context.Context, delay time.Duration, message func(time.Time) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case now := <-timer.C:
			if ctx.Err() != nil {
				return nil
			}
			return message(now)
		}
	}
}
