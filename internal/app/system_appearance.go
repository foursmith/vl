package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/platform"
)

type systemAppearanceMsg struct{ light, known bool }

// Terminal reports take precedence over the system fallback. Run platform I/O
// outside Update, and stop waiting when the workspace closes.
func (m *Model) pollSystemAppearance(wait bool) tea.Cmd {
	if m.appearance.source == "terminal" || !platform.HasSystemAppearance() {
		return nil
	}
	ctx := m.ctx
	return func() tea.Msg {
		if wait {
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
			}
		}
		light, known := platform.SystemAppearance(ctx)
		return systemAppearanceMsg{light, known}
	}
}
