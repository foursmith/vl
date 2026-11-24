package app

import (
	"context"
	"os/exec"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) wantsEnglishInput() bool {
	preference := m.autoSwitchInputMethod
	if !preference.Enable || len(preference.Cmd) == 0 || m.terminalBlurred {
		return false
	}
	switch {
	case m.contextMenu != nil:
		return true
	case m.modal != "":
		return !m.input.Focused()
	case m.flash != nil || m.fileFindFocused():
		return false
	case m.sidebar:
		return m.section != search || m.searchField(m.search.focus) == nil
	default:
		return !m.textInserting()
	}
}

type inputMethodSwitcher struct {
	cancel context.CancelFunc
	mu     sync.Mutex
}

func (s *inputMethodSwitcher) update(m *Model, english bool) tea.Cmd {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if !english {
		return nil
	}
	args := m.autoSwitchInputMethod.Cmd
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Second)
	s.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		// Finish canceling the previous process before starting its replacement.
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = exec.CommandContext(ctx, args[0], args[1:]...).Run()
		return nil
	}
}
