package app

import (
	"os"
	"runtime"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestDefaultWorkbenchShortcutActions(t *testing.T) {
	ctrl := func(code rune) tea.KeyPressMsg {
		return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
	}
	ctrlShift := func(code rune) tea.KeyPressMsg {
		return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl | tea.ModShift}
	}
	keypress := func(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }
	isQuit := func(m *Model, _ tea.Cmd) bool {
		return hasSaveConfirmation(m)
	}
	tabs := func(count int) func(*Model, tea.Cmd) bool {
		return func(m *Model, _ tea.Cmd) bool { return len(m.tabs) == count }
	}
	sidebarSection := func(section section) func(*Model, tea.Cmd) bool {
		return func(m *Model, _ tea.Cmd) bool { return m.sidebar && m.section == section }
	}
	refresh := func(m *Model, cmd tea.Cmd) bool {
		drain(t, m, cmd)
		return len(m.snapshot.Changes) == 2
	}
	copyKeys := []tea.KeyPressMsg{ctrl('c')}
	saveKeys := []tea.KeyPressMsg{ctrl('s')}
	if runtime.GOOS == "darwin" {
		copyKeys = append(copyKeys, tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper})
		saveKeys = append(saveKeys, tea.KeyPressMsg{Code: 's', Mod: tea.ModSuper})
	}
	cases := []struct {
		command commandID
		sidebar bool
		keys    []tea.KeyPressMsg
		check   func(*Model, tea.Cmd) bool
	}{
		{commandQuit, false, []tea.KeyPressMsg{ctrl('q')}, isQuit},
		{commandQuit, true, []tea.KeyPressMsg{ctrl('c')}, isQuit},
		{commandCopy, false, copyKeys, func(m *Model, cmd tea.Cmd) bool {
			return cmd != nil && m.modal == "" && m.document().SelectedText() != ""
		}},
		{commandFiles, false, []tea.KeyPressMsg{ctrl('g')}, func(m *Model, _ tea.Cmd) bool {
			return m.modal == "quick" && !m.commandPicker()
		}},
		{commandRecent, false, []tea.KeyPressMsg{ctrl('r')}, func(m *Model, _ tea.Cmd) bool { return m.modal == "recent" }},
		{commandPalette, false, []tea.KeyPressMsg{ctrl('p')}, func(m *Model, _ tea.Cmd) bool {
			return m.commandPicker()
		}},
		{commandSearch, false, []tea.KeyPressMsg{ctrl('2')}, sidebarSection(search)},
		{commandExplorer, false, []tea.KeyPressMsg{ctrl('1')}, sidebarSection(explorer)},
		{commandChanges, false, []tea.KeyPressMsg{ctrl('3')}, sidebarSection(changes)},
		{commandHistory, false, []tea.KeyPressMsg{ctrl('4')}, sidebarSection(history)},
		{commandPullRequests, false, []tea.KeyPressMsg{ctrl('5')}, sidebarSection(pullRequests)},
		{commandSidebar, false, []tea.KeyPressMsg{ctrl('l')}, func(m *Model, _ tea.Cmd) bool { return !m.sidebarVisible }},
		{commandFocus, false, []tea.KeyPressMsg{keypress(tea.KeyF2)}, sidebarSection(explorer)},
		{commandRefresh, false, []tea.KeyPressMsg{keypress(tea.KeyF5)}, refresh},
		{commandRefresh, true, []tea.KeyPressMsg{keypress('r'), keypress('R'), {Code: 'r', Mod: tea.ModShift}}, refresh},
		{commandSave, false, saveKeys, func(m *Model, _ tea.Cmd) bool {
			data, err := os.ReadFile(m.document().Path)
			return err == nil && !m.document().Dirty() && string(data) == m.document().Text
		}},
		{commandClose, false, []tea.KeyPressMsg{ctrl('w')}, tabs(1)},
		{commandCloseOthers, false, []tea.KeyPressMsg{ctrlShift('w'), ctrlShift('W')}, tabs(1)},
		{commandFind, false, []tea.KeyPressMsg{ctrl('f')}, func(m *Model, _ tea.Cmd) bool { return m.fileFindFocused() }},
		{commandFocusSidebar, false, []tea.KeyPressMsg{keypress(tea.KeyEscape)}, sidebarSection(explorer)},
		{commandContextMenu, false, []tea.KeyPressMsg{keypress(tea.KeyF7)}, func(m *Model, _ tea.Cmd) bool {
			return m.contextMenu != nil
		}},
	}
	covered := make(map[commandID]map[string]bool)
	for _, test := range cases {
		for _, msg := range test.keys {
			name := msg.String()
			if covered[test.command] == nil {
				covered[test.command] = make(map[string]bool)
			}
			covered[test.command][name] = true
			t.Run(string(test.command)+"/"+name, func(t *testing.T) {
				m := tabWorkspace(t, 2)
				defer m.Close()
				m.openFile("file0.go")
				m.openFile("file1.go")
				m.sidebar = test.sidebar
				switch test.command {
				case commandQuit:
					m.document().Insert("unsaved")
				case commandCopy:
					m.document().Insert("unsaved")
					m.document().SelectAll()
				case commandSave:
					m.document().Insert("saved")
					// Save should reach the workbench while a Vim operator is pending.
					press(m, "d")
				case commandFocusSidebar:
					// Text views let Vim handle Escape before workbench dispatch.
					m.closeAllTabs()
					m.sidebar = false
				}
				if got := m.resolveWorkbenchShortcut(name).command; got != test.command {
					t.Fatalf("resolved to %q instead of %q", got, test.command)
				}
				_, cmd := m.Update(msg)
				if !test.check(m, cmd) {
					t.Fatalf("%s did not produce the expected action", name)
				}
			})
		}
	}
	for _, rule := range workbenchKeybindings {
		for _, name := range rule.binding.Keys() {
			if !covered[rule.command][name] {
				t.Errorf("default binding %q for %s has no action test", name, rule.ruleID())
			}
		}
	}
}

func TestWorkbenchKeymapOverridesShareDispatchAndHints(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.openFile("file0.go")
	m.openFile("file1.go")
	active := m.document()
	configured, err := buildWorkbenchKeymap([]workbenchKeyOverride{
		{id: string(commandCopy), binding: binding("F8", "", "f8")},
		{id: string(commandCloseOthers), binding: binding("F8", "", "f8")},
		{id: string(commandFind), binding: key.Binding{}},
		{id: string(commandCloseAll), binding: binding("F9", "", "f9")},
	})
	if err != nil {
		t.Fatal(err)
	}
	m.workbenchKeymap = configured

	for _, old := range []string{"ctrl+c", "ctrl+f", "ctrl+shift+w"} {
		if got := m.resolveWorkbenchShortcut(old).command; got != "" {
			t.Errorf("old binding %s still resolves to %q", old, got)
		}
	}
	if m.matchWorkbenchCommand("ctrl+f", commandFind) || m.commandShortcutHint(commandFind) != "" {
		t.Fatal("removed binding is still visible to find widgets")
	}
	if hint := m.commandShortcutHint(commandCopy); hint != "" {
		t.Fatalf("shadowed binding still displays as %q", hint)
	}
	for _, a := range m.commandActions() {
		if a.id == string(commandCloseOthers) && a.key != "F8" {
			t.Fatalf("Close Others advertises %q", a.key)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF8})
	if len(m.tabs) != 1 || m.document() != active {
		t.Fatal("later override did not win or retain the active tab")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF9})
	if len(m.tabs) != 0 {
		t.Fatal("assigning a key to an unbound command did not make it callable")
	}
}

func TestWorkbenchShortcutUsesLastMatchingContext(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()

	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != commandQuit {
		t.Fatalf("Ctrl+C without a document resolved to %q", got)
	}
	m.openFile("file0.go")
	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != commandCopy {
		t.Fatalf("Ctrl+C in the editor resolved to %q", got)
	}
	m.sidebar = true
	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != commandQuit {
		t.Fatalf("Ctrl+C in the sidebar resolved to %q", got)
	}
	if got := m.commandShortcutHint(commandQuit); got != "Ctrl+Q" {
		t.Fatalf("Quit advertises its fallback key: %q", got)
	}
	if got := m.commandShortcutHint(commandRefresh); got != "r" {
		t.Fatalf("Refresh in the sidebar advertises %q", got)
	}
	m.sidebar = false
	if got := m.commandShortcutHint(commandRefresh); got != "F5" {
		t.Fatalf("Refresh in the editor advertises %q", got)
	}
}

func TestWorkbenchShortcutsWithFocusedSearchInputs(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.openFile("file0.go")
	m.openFile("file1.go")
	active := m.document()

	press(m, "ctrl+f")
	press(m, "x")
	press(m, "ctrl+w")
	if !m.fileFindFocused() || m.search.query.Value() != "" || len(m.tabs) != 2 {
		t.Fatal("file find must own Ctrl+W for deleting a word")
	}
	press(m, "ctrl+p")
	if !m.commandPicker() || m.fileFindFocused() {
		t.Fatal("Ctrl+P must leave file find and open the command palette")
	}
	press(m, "esc")

	press(m, "ctrl+2")
	press(m, "S")
	if !m.sidebar || m.section != search || m.search.query.Value() != "S" {
		t.Fatal("workspace search must receive printable Git keys as text")
	}
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl | tea.ModShift})
	if len(m.tabs) != 1 || m.document() != active {
		t.Fatal("Ctrl+Shift+W must close other tabs with workspace search focused")
	}
}
