package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
)

func TestInputMethodFocusFollowsKeyboardTarget(t *testing.T) {
	m := flashModel(t, "one two")
	m.sidebarVisible = true
	if m.wantsEnglishInput() {
		t.Fatal("switching should be disabled by default")
	}
	m.autoSwitchInputMethod = config.InputMethodSwitch{Cmd: []string{"im-select", "english"}}
	if m.wantsEnglishInput() {
		t.Fatal("a configured command should remain disabled until explicitly enabled")
	}
	m.autoSwitchInputMethod.Enable = true

	check := func(want bool) {
		t.Helper()
		if got := m.wantsEnglishInput(); got != want {
			t.Fatalf("wants English input = %v, want %v", got, want)
		}
	}
	check(true)
	m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	check(false)
	m.Update(tea.BlurMsg{})
	m.Update(tea.FocusMsg{})
	check(false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	check(true)

	m.openFileFind()
	check(false)
	m.blurFileFind()
	check(true)
	m.openSearch()
	check(false)
	m.focusSearch(2)
	check(true)
	m.prompt("quick", "Go to File", "")
	check(false)
	m.dismissDialog()
	check(true)
	m.Update(tea.BlurMsg{})
	check(false)
	m.Update(tea.FocusMsg{})
	check(true)
}

func TestInputMethodDoesNotSwitchBetweenNavigationTargets(t *testing.T) {
	m := flashModel(t, "one two")
	m.autoSwitchInputMethod = config.InputMethodSwitch{Enable: true, Cmd: []string{"im-select", "english"}}
	r := NewRuntime(m, nil)

	r.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if !m.sidebar || r.inputMethod.cancel != nil {
		t.Fatal("focusing the sidebar should not schedule a switch")
	}
	r.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if m.sidebar || r.inputMethod.cancel != nil {
		t.Fatal("returning to the editor in Normal mode should not schedule a switch")
	}
	r.Update(tea.FocusMsg{})
	if r.inputMethod.cancel != nil {
		t.Fatal("focus reporting while already focused should not schedule a switch")
	}

	r.Update(tea.BlurMsg{})
	r.Update(tea.FocusMsg{})
	if r.inputMethod.cancel == nil {
		t.Fatal("returning after terminal blur should schedule a switch")
	}
}
