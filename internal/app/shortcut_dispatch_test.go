package app

import (
	"os"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
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
	cases := []struct {
		command commandID
		sidebar bool
		keys    []tea.KeyPressMsg
		check   func(*Model, tea.Cmd) bool
	}{
		{commandQuit, false, []tea.KeyPressMsg{ctrl('q')}, isQuit},
		{commandCopy, false, []tea.KeyPressMsg{ctrl('c')}, func(m *Model, cmd tea.Cmd) bool {
			return cmd != nil && m.modal == "" && m.document().SelectedText() != ""
		}},
		{commandCut, false, []tea.KeyPressMsg{ctrl('x')}, func(m *Model, cmd tea.Cmd) bool {
			return cmd != nil && m.document().SelectedText() != ""
		}},
		{commandPaste, false, []tea.KeyPressMsg{ctrl('v')}, func(_ *Model, cmd tea.Cmd) bool { return cmd != nil }},
		{commandSelectAll, false, []tea.KeyPressMsg{ctrl('a')}, func(m *Model, _ tea.Cmd) bool {
			return m.document().SelectedText() == m.document().Text
		}},
		{commandDeleteToLineStart, false, []tea.KeyPressMsg{ctrl(tea.KeyBackspace)}, func(m *Model, _ tea.Cmd) bool {
			return m.document().Text == "first\n整行\nlast"
		}},
		{commandDeleteToLineEnd, false, []tea.KeyPressMsg{ctrl(tea.KeyDelete)}, func(m *Model, _ tea.Cmd) bool {
			return m.document().Text == "first\n删除\nlast"
		}},
		{commandUndo, false, []tea.KeyPressMsg{ctrl('z')}, func(m *Model, _ tea.Cmd) bool { return !m.document().Dirty() }},
		{commandRedo, false, []tea.KeyPressMsg{ctrlShift('z'), ctrlShift('Z')}, func(m *Model, _ tea.Cmd) bool {
			return m.document().Dirty()
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
		{commandSidebar, false, []tea.KeyPressMsg{keypress(tea.KeyF12)}, func(m *Model, _ tea.Cmd) bool {
			if m.sidebarVisible {
				return false
			}
			m.Update(ctrl('l'))
			if m.sidebarVisible {
				return false
			}
			m.editorOnly = true
			m.Update(keypress(tea.KeyF12))
			return !m.sidebarVisible
		}},
		{commandFocus, false, []tea.KeyPressMsg{keypress(tea.KeyF2)}, sidebarSection(explorer)},
		{commandRefresh, false, []tea.KeyPressMsg{keypress(tea.KeyF5)}, refresh},
		{commandRefresh, true, []tea.KeyPressMsg{keypress('r'), keypress('R'), {Code: 'r', Mod: tea.ModShift}}, refresh},
		{commandSave, false, []tea.KeyPressMsg{ctrl('s')}, func(m *Model, _ tea.Cmd) bool {
			data, err := os.ReadFile(m.document().Path)
			return err == nil && !m.document().Dirty() && string(data) == m.document().Text
		}},
		{commandClose, false, []tea.KeyPressMsg{ctrl('w')}, tabs(1)},
		{commandCloseOthers, false, []tea.KeyPressMsg{ctrlShift('w'), ctrlShift('W')}, tabs(1)},
		{commandFind, false, []tea.KeyPressMsg{ctrl('f')}, func(m *Model, _ tea.Cmd) bool { return m.fileFindFocused() }},
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
				case commandCopy, commandCut:
					m.document().Insert("unsaved")
					m.document().SelectAll()
				case commandDeleteToLineStart, commandDeleteToLineEnd:
					m.document().Edit("first\n删除整行\nlast")
					m.document().Cursor = 8
				case commandUndo, commandRedo:
					m.document().Insert("unsaved")
					if test.command == commandRedo {
						m.document().Undo()
					}
				case commandSave:
					m.document().Insert("saved")
					// Save should reach the workbench while a Vim operator is pending.
					press(m, "d")
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

func TestDeleteToLineBoundaryFollowsEditorMode(t *testing.T) {
	for _, mode := range []string{"normal", "insert", "visual", "vim disabled", "read only"} {
		t.Run(mode, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			m.openFile("file0.go")
			m.sidebar = false
			d := m.document()
			const text = "first\n删除整行\nlast"
			d.Edit(text)
			d.Cursor = 8
			switch mode {
			case "insert":
				press(m, "i")
			case "visual":
				press(m, "V")
			case "vim disabled":
				m.vimDisabled = true
			case "read only":
				d.ReadOnly = true
			}
			for _, test := range []struct {
				key  rune
				want string
			}{{tea.KeyBackspace, "first\n整行\nlast"}, {tea.KeyDelete, "first\n删除\nlast"}} {
				want := test.want
				if mode == "read only" {
					want = text
				}
				key := tea.KeyPressMsg{Code: test.key, Mod: tea.ModCtrl}
				m.Update(key)
				if d.Text != want {
					t.Fatalf("%s changed the document to %q, want %q", key, d.Text, want)
				}
				if v := m.tabs[m.activeTab].vim; v.InsertMode != (mode == "insert") || v.VisualMode {
					t.Fatalf("unexpected Vim mode after %s: %s", key, v.Label())
				}
				if mode != "read only" {
					press(m, "ctrl+z")
					if d.Text != text {
						t.Fatalf("undo restored %q, want %q", d.Text, text)
					}
				}
			}
		})
	}
}

func TestEscapeKeepsWorkbenchFocus(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()

	m.sidebar = false
	m.sidebarVisible = false
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.sidebar || m.sidebarVisible {
		t.Fatal("Escape opened the sidebar from the empty editor")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if !m.sidebar || !m.sidebarVisible {
		t.Fatal("F2 did not open and focus the sidebar")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.sidebar || !m.sidebarVisible {
		t.Fatal("Escape moved focus out of the sidebar")
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

func TestCommitShortcutRequiresChangesFocus(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	m.snapshot.Changes = []gitservice.Change{{Path: "file0.go", Index: 'M'}}
	for _, view := range []section{explorer, search, history, pullRequests, changes} {
		m.section, m.sidebar = view, view != changes
		press(m, "c")
		if m.modal == "commit" {
			t.Fatalf("commit opened in section %v with sidebar focus %t", view, m.sidebar)
		}
	}
	m.sidebar = true
	press(m, "c")
	if m.modal != "commit" {
		t.Fatal("c did not open the commit dialog with Changes focused")
	}
}

func TestWorkbenchShortcutUsesLastMatchingContext(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()

	copyKey := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != "" || m.key(copyKey) != nil {
		t.Fatalf("Ctrl+C without a document resolved to %q", got)
	}
	m.openFile("file0.go")
	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != commandCopy {
		t.Fatalf("Ctrl+C in the editor resolved to %q", got)
	}
	for _, name := range []string{"super+c", "super+s"} {
		if got := m.resolveWorkbenchShortcut(name).command; got != "" {
			t.Errorf("%s in the editor resolved to %q", name, got)
		}
	}
	m.sidebar = true
	m.document().Insert("unsaved")
	if got := m.resolveWorkbenchShortcut("ctrl+c").command; got != "" || m.key(copyKey) != nil || hasSaveConfirmation(m) {
		t.Fatalf("Ctrl+C in the sidebar resolved to %q", got)
	}
	if got := m.commandShortcutHint(commandQuit); got != "Ctrl+Q" {
		t.Fatalf("Quit advertises %q", got)
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
	if !m.fileFindFocused() || m.search.query.Value() != "x" || len(m.tabs) != 2 {
		t.Fatal("Ctrl+W changed file find or closed an editor")
	}
	press(m, "ctrl+p")
	if !m.commandPicker() || m.fileFindFocused() {
		t.Fatal("Ctrl+P must leave file find and open the command palette")
	}
	press(m, "help")
	press(m, "ctrl+w")
	if m.input.Value() != ">help" || !m.commandPicker() || len(m.tabs) != 2 {
		t.Fatal("Ctrl+W changed the command palette or closed an editor")
	}
	press(m, "esc")

	press(m, "ctrl+2")
	press(m, "S")
	if !m.sidebar || m.section != search || m.search.query.Value() != "xS" {
		t.Fatal("workspace search must receive printable Git keys as text")
	}
	press(m, "ctrl+w")
	if m.search.query.Value() != "xS" || len(m.tabs) != 2 {
		t.Fatal("Ctrl+W changed the workspace query or closed an editor")
	}
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl | tea.ModShift})
	if len(m.tabs) != 1 || m.document() != active {
		t.Fatal("Ctrl+Shift+W must close other tabs with workspace search focused")
	}
	press(m, "esc")
	press(m, "ctrl+w")
	if len(m.tabs) != 0 {
		t.Fatal("Ctrl+W did not close the editor from the workspace results")
	}
}

func TestWorkspaceSearchWorkbenchShortcutsFollowFocus(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	press(m, "ctrl+2")
	m.search.query.SetValue("package")

	for _, focus := range []int{0, 2} {
		m.focusSearch(focus)
		previous := m.search.seq
		if hint := m.commandShortcutHint(commandRefresh); hint != "F5" {
			t.Fatalf("refresh hint with search focus %d = %q", focus, hint)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
		if m.search.seq != previous+1 || m.search.focus != focus || !m.sidebar {
			t.Fatalf("F5 did not refresh workspace search and preserve focus %d", focus)
		}
	}
	press(m, "ctrl+f")
	if m.search.focus != 0 || !m.sidebar || m.fileFindFocused() {
		t.Fatal("Ctrl+F did not focus the workspace query")
	}
}

func TestWorkspaceRefreshReloadsFileAndKeepsFindFocused(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	press(m, "ctrl+f")
	press(m, "package")
	const updated = "package updated\n"
	if err := os.WriteFile(m.document().Path, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	previous := m.refreshSeq
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
	drain(t, m, cmd)
	if m.refreshSeq != previous+1 || m.document().Text != updated ||
		!m.fileFindFocused() || !m.search.query.Focused() ||
		m.search.query.Value() != "package" {
		t.Fatal("F5 did not refresh the workspace and file while keeping focus and query in file find")
	}
}

func TestGoToFileShortcutAcrossFocus(t *testing.T) {
	for _, test := range []struct {
		name string
		keys []string
	}{
		{name: "editor"},
		{name: "explorer", keys: []string{"ctrl+1"}},
		{name: "file find", keys: []string{"ctrl+f"}},
		{name: "workspace search input", keys: []string{"ctrl+2"}},
		{name: "workspace search results", keys: []string{"ctrl+2", "esc"}},
		{name: "command palette", keys: []string{"ctrl+p"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			m.openFile("file0.go")
			for _, k := range test.keys {
				press(m, k)
			}
			press(m, "ctrl+g")
			if m.modal != "quick" || m.input.Value() != "" || m.fileFindFocused() {
				t.Fatal("Ctrl+G did not open Go to File")
			}
		})
	}
}

func TestWorkbenchShortcutHintRespectsSidebarNavigation(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.openFile("file0.go")
	m.sidebar = true
	keymap, err := buildWorkbenchKeymap([]workbenchKeyOverride{
		{id: string(commandPalette), binding: binding("j", "", "j")},
	})
	if err != nil {
		t.Fatal(err)
	}
	m.workbenchKeymap = keymap
	if hint := m.commandShortcutHint(commandPalette); hint != "" {
		t.Fatalf("command palette advertises sidebar navigation key %q", hint)
	}
	press(m, "j")
	if m.commandPicker() {
		t.Fatal("j opened the command palette from the sidebar")
	}
}
