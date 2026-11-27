package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestWorkbenchShortcutActions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	ctrl := func(code rune) tea.KeyPressMsg {
		return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
	}
	ctrlShift := func(code rune) tea.KeyPressMsg {
		return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl | tea.ModShift}
	}
	keypress := func(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }
	shift := func(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Mod: tea.ModShift} }
	startsOperation := func(_ *Model, cmd tea.Cmd) bool { return cmd != nil }
	confirmsDiscard := func(m *Model, _ tea.Cmd) bool { return m.confirmationDialog() }
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
	type shortcutCase struct {
		command commandID
		sidebar bool
		keys    []tea.KeyPressMsg
		check   func(*Model, tea.Cmd) bool
	}
	cases := []shortcutCase{
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
		{commandToggleComment, false, []tea.KeyPressMsg{ctrl('/')}, func(m *Model, _ tea.Cmd) bool {
			return m.document().Text == "// first\nsecond"
		}},
		{commandFiles, false, []tea.KeyPressMsg{ctrl('g')}, func(m *Model, _ tea.Cmd) bool {
			return m.modal == "quick" && !m.commandPicker()
		}},
		{commandRecent, false, []tea.KeyPressMsg{ctrl('r')}, func(m *Model, _ tea.Cmd) bool { return m.modal == "recent" }},
		{commandPalette, false, []tea.KeyPressMsg{ctrl('p')}, func(m *Model, _ tea.Cmd) bool {
			return m.commandPicker()
		}},
		{commandOpenConfig, true, []tea.KeyPressMsg{ctrl(',')}, func(m *Model, _ tea.Cmd) bool {
			return m.document() != nil && filepath.Base(m.document().Path) == "vl.config.json" &&
				m.document().Text == "{\n}\n" && !m.sidebar && !m.tabs[m.activeTab].preview
		}},
		{commandOpenKeybindings, true, nil, func(m *Model, _ tea.Cmd) bool {
			return m.document() != nil && filepath.Base(m.document().Path) == "keybindings.json" &&
				m.document().Text == "[\n]\n" && !m.sidebar && !m.tabs[m.activeTab].preview
		}},
		{commandSearch, false, []tea.KeyPressMsg{ctrlShift('f'), ctrlShift('F')}, sidebarSection(search)},
		{sidebarTabCommands[0], false, []tea.KeyPressMsg{ctrl('1')}, sidebarSection(explorer)},
		{sidebarTabCommands[1], false, []tea.KeyPressMsg{ctrl('2')}, sidebarSection(search)},
		{sidebarTabCommands[2], false, []tea.KeyPressMsg{ctrl('3')}, sidebarSection(changes)},
		{sidebarTabCommands[3], false, []tea.KeyPressMsg{ctrl('4')}, sidebarSection(history)},
		{sidebarTabCommands[4], false, []tea.KeyPressMsg{ctrl('5')}, sidebarSection(pullRequests)},
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
		{commandPreviousTab, false, []tea.KeyPressMsg{ctrlShift('t'), ctrlShift('T')}, func(m *Model, _ tea.Cmd) bool {
			return m.activeTab == 1
		}},
		{commandNextTab, false, []tea.KeyPressMsg{ctrl('t')}, func(m *Model, _ tea.Cmd) bool {
			return m.activeTab == 0
		}},
		{commandCloseOthers, false, []tea.KeyPressMsg{ctrlShift('w'), ctrlShift('W')}, tabs(1)},
		{commandCloseAll, false, nil, tabs(0)},
		{commandFind, false, []tea.KeyPressMsg{ctrl('f')}, func(m *Model, _ tea.Cmd) bool { return m.fileFindFocused() }},
		{commandContextMenu, false, []tea.KeyPressMsg{keypress(tea.KeyF7)}, func(m *Model, _ tea.Cmd) bool {
			return m.contextMenu != nil
		}},
		{commandPreviousHunk, false, []tea.KeyPressMsg{keypress('[')}, func(m *Model, _ tea.Cmd) bool { return m.diffHunk == 0 }},
		{commandNextHunk, false, []tea.KeyPressMsg{keypress(']')}, func(m *Model, _ tea.Cmd) bool { return m.diffHunk == 1 }},
		{commandToggleDiffContext, false, []tea.KeyPressMsg{keypress(tea.KeyEnter)}, func(m *Model, _ tea.Cmd) bool {
			return len(m.contextFolds[0].lines) != 0
		}},
		{commandToggleDiffStage, false, []tea.KeyPressMsg{keypress('s')}, startsOperation},
		{commandToggleDiffFileStage, false, []tea.KeyPressMsg{keypress('S'), shift('s')}, startsOperation},
		{commandDiscardDiffChanges, false, []tea.KeyPressMsg{keypress('d')}, confirmsDiscard},
		{commandDiscardDiffFile, false, []tea.KeyPressMsg{keypress('D'), shift('d')}, confirmsDiscard},
		{commandDiffLayout, false, nil, func(m *Model, _ tea.Cmd) bool { return m.split && m.diffLayout.split }},
	}
	covered := make(map[commandID]map[string]bool)
	customCovered := make(map[string]bool)
	run := func(t *testing.T, test shortcutCase, msg tea.KeyPressMsg, custom bool) {
		t.Helper()
		m := tabWorkspace(t, 2)
		defer m.Close()
		m.openFile("file0.go")
		m.openFile("file1.go")
		m.sidebar = test.sidebar
		if defaultWorkbenchKeymap.byID[string(test.command)].has(diffInput) || test.command == commandDiffLayout {
			showShortcutTestDiff(m)
		}
		switch test.command {
		case commandDiffLayout:
			m.split = false
			m.Update(tea.WindowSizeMsg{Width: 180, Height: 40})
		case commandPreviousHunk:
			m.nextHunk(1)
		case commandNextTab:
			m.setVimDisabled(true)
		case commandPreviousTab:
			m.activateTab(0)
			press(m, "i")
		case commandOpenConfig:
			m.SetPreferencesStore(config.NewPreferencesStore(filepath.Join(t.TempDir(), "vl", "vl.config.json")))
		case commandQuit:
			m.document().Insert("unsaved")
		case commandCopy, commandCut:
			m.document().Insert("unsaved")
			m.document().SelectAll()
		case commandDeleteToLineStart, commandDeleteToLineEnd:
			m.document().Edit("first\n删除整行\nlast")
			m.document().Cursor = 8
		case commandToggleComment:
			m.document().Edit("first\nsecond")
			m.document().SetPosition(0, 0)
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
		name := msg.String()
		if custom {
			id := string(test.command)
			if len(test.keys) != 0 {
				original := m.resolveWorkbenchShortcut(name)
				if original.command != test.command {
					t.Fatalf("%s resolved to %q before overriding, want %q", name, original.command, test.command)
				}
				id = original.ruleID()
			}
			customCovered[id] = true
			load := func(keys []string) {
				t.Helper()
				data, err := json.Marshal([]struct {
					Command string   `json:"command"`
					Keys    []string `json:"keys"`
				}{{Command: id, Keys: keys}})
				if err != nil {
					t.Fatal(err)
				}
				m.workbenchKeymap = testKeybindings(t, data)
			}
			load([]string{})
			if m.activeWorkbenchKeymap().byID[id].binding.Enabled() {
				t.Fatalf("%s remained bound after disabling it", id)
			}
			load([]string{"F8", "F9"})
			for _, original := range test.keys {
				if m.resolveWorkbenchShortcut(original.String()).ruleID() == id {
					t.Errorf("%s still uses its old key %s", id, original.String())
				}
			}
			if got := m.resolveWorkbenchShortcut("f9").ruleID(); got != id {
				t.Fatalf("alternate key resolved to %q, want %q", got, id)
			}
			msg = keypress(tea.KeyF8)
			name = msg.String()
		}
		if got := m.resolveWorkbenchShortcut(name).command; got != test.command {
			t.Fatalf("resolved to %q instead of %q", got, test.command)
		}
		_, cmd := m.Update(msg)
		if !test.check(m, cmd) {
			t.Fatalf("%s did not produce the expected action", name)
		}
	}
	for _, test := range cases {
		for _, msg := range test.keys {
			name := msg.String()
			if covered[test.command] == nil {
				covered[test.command] = make(map[string]bool)
			}
			covered[test.command][name] = true
			t.Run("default/"+string(test.command)+"/"+name, func(t *testing.T) {
				run(t, test, msg, false)
			})
		}
		var original tea.KeyPressMsg
		name := "unbound"
		if len(test.keys) != 0 {
			original = test.keys[0]
			name = original.String()
		}
		t.Run("custom/"+string(test.command)+"/"+name, func(t *testing.T) {
			run(t, test, original, true)
		})
	}
	for _, rule := range workbenchKeybindings {
		if !customCovered[rule.ruleID()] {
			t.Errorf("custom binding %s has no action test", rule.ruleID())
		}
		for _, name := range rule.binding.Keys() {
			if !covered[rule.command][name] {
				t.Errorf("default binding %q for %s has no action test", name, rule.ruleID())
			}
		}
	}
}

func testKeybindings(t *testing.T, data []byte) *workbenchKeymap {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keybindings.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	keymap, err := loadWorkbenchKeymap(path)
	if err != nil {
		t.Fatal(err)
	}
	return keymap
}

func showShortcutTestDiff(m *Model) {
	m.diffPath = "file0.go"
	m.diffChange = &gitservice.Change{Path: m.diffPath, Worktree: 'M'}
	m.patch = diff.Parse("diff --git a/file0.go b/file0.go\n--- a/file0.go\n+++ b/file0.go\n@@ -2 +2 @@\n-old\n+OLD\n@@ -5 +5 @@\n-old again\n+OLD again\n")
	m.initContextFolds("first\nold\nthird\nfourth\nold again\nlast\n")
	showDiffFixture(m)
	m.renderDiff()
	m.nextHunk(0)
}

func TestConfiguredDiffShortcutsUseFocusedDiffAndMenuTarget(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.workbenchKeymap = testKeybindings(t, []byte(`[
		{"command": "diff.nextHunk", "keys": "F8"},
		{"command": "diff.toggleStageSelectionOrHunk", "keys": "S"},
		{"command": "diff.toggleStageFile", "keys": "sHiFt+s"},
		{"command": "diff.discardSelectionOrHunk", "keys": ["F9", "F10"]}
	]`))
	showShortcutTestDiff(m)
	if m.resolveWorkbenchShortcut("s").command != commandToggleDiffStage || m.commandShortcutHint(commandToggleDiffStage) != "s" {
		t.Fatal("a capital letter in the config changed the unmodified binding")
	}
	for _, name := range []string{"S", "shift+s", "shift+S"} {
		if m.resolveWorkbenchShortcut(name).command != commandToggleDiffFileStage {
			t.Fatalf("explicit Shift binding did not match terminal event %q", name)
		}
	}
	press(m, "]")
	if m.diffHunk != 0 {
		t.Fatal("replaced default key still moved to the next hunk")
	}
	m.openFileFind()
	m.Update(tea.KeyPressMsg{Code: tea.KeyF8})
	if m.diffHunk != 0 || !m.fileFindFocused() {
		t.Fatal("diff shortcut intercepted input in Find")
	}
	m.closeFileFind()
	m.Update(tea.KeyPressMsg{Code: tea.KeyF8})
	if m.diffHunk != 1 {
		t.Fatal("configured shortcut did not move to the next hunk")
	}
	m.contextMenu = &contextMenu{actions: m.reviewActions()}
	for _, action := range m.contextMenu.actions {
		if action.id == string(commandDiscardDiffChanges) && action.key != "F9" {
			t.Fatalf("discard action displays %q instead of F9", action.key)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF10})
	if !m.confirmationDialog() || m.contextMenu != nil {
		t.Fatal("alternate shortcut did not open the menu target's discard confirmation")
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
	m.workbenchKeymap = testKeybindings(t, []byte(`[
		{"command": "edit.copy", "keys": "F8"},
		{"command": "editor.closeOtherTabs", "keys": "F8"},
		{"command": "edit.findInFile", "keys": []},
		{"command": "editor.closeAllTabs", "keys": "F9"}
	]`))

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
	for _, focus := range []string{"ctrl+f", "ctrl+2"} {
		t.Run(focus, func(t *testing.T) {
			m := tabWorkspace(t, 3)
			defer m.Close()
			m.openFile("file0.go")
			m.openFile("file1.go")
			m.openFile("file2.go")
			press(m, focus)
			press(m, "x")
			press(m, "ctrl+w")
			if len(m.tabs) != 2 {
				t.Fatal("Ctrl+W did not close the active editor from search")
			}
			press(m, focus)
			m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl | tea.ModShift})
			if len(m.tabs) != 1 {
				t.Fatal("Ctrl+Shift+W did not close the other editor from search")
			}
			press(m, focus)
			press(m, "ctrl+r")
			if m.modal != "recent" {
				t.Fatal("Ctrl+R did not open recent workspaces from search")
			}
			press(m, "ctrl+p")
			if !m.commandPicker() || m.fileFindFocused() {
				t.Fatal("Ctrl+P did not switch pickers")
			}
			press(m, "ctrl+w")
			if !m.commandPicker() || len(m.tabs) != 1 {
				t.Fatal("Ctrl+W escaped the command palette")
			}
			press(m, "ctrl+r")
			if m.modal != "recent" {
				t.Fatal("Ctrl+R did not switch pickers")
			}
		})
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
	if m.search.focus != 2 || m.sidebar || !m.fileFindFocused() || m.findWidget.query.Value() != "" {
		t.Fatal("Ctrl+F did not open independent file find from workspace results")
	}
	for _, code := range []rune{'f', 'F'} {
		m.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl | tea.ModShift})
		if m.search.focus != 0 || !m.sidebar || !m.findWidget.visible || m.fileFindFocused() || m.search.query.Value() != "package" {
			t.Fatal("Ctrl+Shift+F did not focus the workspace query")
		}
		press(m, "ctrl+f")
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
		!m.fileFindFocused() || !m.findWidget.query.Focused() ||
		m.findWidget.query.Value() != "package" {
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
	m.workbenchKeymap = testKeybindings(t, []byte(`[{"command": "workbench.openCommandPalette", "keys": "j"}]`))
	if hint := m.commandShortcutHint(commandPalette); hint != "" {
		t.Fatalf("command palette advertises sidebar navigation key %q", hint)
	}
	press(m, "j")
	if m.commandPicker() {
		t.Fatal("j opened the command palette from the sidebar")
	}
}

func TestConfiguredSidebarTabShortcuts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	m := tabWorkspace(t, 1)
	defer m.Close()
	path, err := config.KeybindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureKeybindingsFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`[
		{"command": "sidebar.showExplorer", "keys": "Alt+1"},
		{"command": "sidebar.showSearch", "keys": ["F8", "Alt+2"]},
		{"command": "sidebar.showChanges", "keys": []},
		{"command": "sidebar.showPullRequests", "keys": "Shift+Ctrl+P"},
	]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadKeybindings(); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	m.Update(tea.KeyPressMsg{Code: tea.KeyF8})
	if m.section != search || !m.sidebar || m.search.focus != 0 {
		t.Fatal("F8 did not open Search with query focus")
	}
	for _, k := range []string{"ctrl+2", "ctrl+3", "ctrl+5"} {
		if cmd := m.resolveWorkbenchShortcut(k).command; cmd != "" {
			t.Fatalf("replaced shortcut %s still resolves to %s", k, cmd)
		}
	}
	m.Update(tea.KeyPressMsg{Code: '1', Mod: tea.ModAlt})
	if m.section != explorer || !m.sidebar {
		t.Fatal("Alt+1 did not switch from the search field to Explorer")
	}
	m.Update(tea.KeyPressMsg{Code: '2', Mod: tea.ModAlt})
	if m.section != search {
		t.Fatal("alternate shortcut Alt+2 did not open Search")
	}
	press(m, "ctrl+4")
	if m.section != history {
		t.Fatal("unspecified sidebar binding lost its default")
	}
	for _, code := range []rune{'p', 'P'} {
		m.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModShift | tea.ModCtrl})
		if m.section != pullRequests {
			t.Fatalf("configured chord with key code %q did not switch to Pull Requests", code)
		}
		press(m, "ctrl+4")
	}
	press(m, "ctrl+p")
	for _, action := range m.filteredActions() {
		if action.id == string(commandExplorer) && action.key == "Alt+1" {
			return
		}
	}
	t.Fatal("command palette did not advertise Alt+1 for Explorer")
}

func TestKeybindingsExampleMatchesDefaults(t *testing.T) {
	var example string
	for _, name := range []string{"KEYBINDINGS.md", "zh.CN/KEYBINDINGS.md"} {
		doc, err := os.ReadFile(filepath.Join("..", "..", "docs", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		_, rest, start := strings.Cut(string(doc), "```json\n")
		data, _, end := strings.Cut(rest, "\n```")
		if !start || !end || data == "" {
			t.Fatalf("%s is missing the JSON example", name)
		}
		if example != "" && data != example {
			t.Fatalf("%s has a different JSON example", name)
		}
		example = data
	}
	var entries []struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(example), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(workbenchKeybindings) {
		t.Fatalf("example has %d bindings, want %d", len(entries), len(workbenchKeybindings))
	}
	keymap := testKeybindings(t, []byte(example))
	rules := slices.Clone(workbenchKeybindings)
	slices.SortFunc(rules, func(a, b workbenchShortcut) int {
		return strings.Compare(a.ruleID(), b.ruleID())
	})
	for i, rule := range rules {
		id := rule.ruleID()
		if entries[i].Command != id {
			t.Errorf("example entry %d is %q, want %q", i+1, entries[i].Command, id)
		}
		binding := keymap.byID[id].binding
		if binding.Enabled() != rule.binding.Enabled() {
			t.Errorf("example changed the default shortcut for %q", id)
		}
		for _, name := range rule.binding.Keys() {
			if !slices.Contains(binding.Keys(), name) {
				t.Errorf("example omitted default key %q for %q", name, id)
			}
		}
	}
}

func TestInvalidConfiguredKeybindingsKeepValidOverrides(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	path := filepath.Join(t.TempDir(), "keybindings.json")
	if err := os.WriteFile(path, []byte(`[
		{"command": "sidebar.showUnknown", "keys": "F8"},
		{"command": "sidebar.showExplorer", "keys": "Alt+"},
		{"command": "sidebar.showSearch", "keys": "F8"},
		{"command": "sidebar.showSearch", "keys": "F9"},
		{"command": "sidebar.showChanges", "keys": "F10", "when": "sidebarFocus"}
	]`), 0600); err != nil {
		t.Fatal(err)
	}
	keymap, err := loadWorkbenchKeymap(path)
	if err == nil || !strings.Contains(err.Error(), "skipped 4 entries") {
		t.Fatalf("expected invalid entries to be reported, got %v", err)
	}
	m.workbenchKeymap = keymap
	if m.resolveWorkbenchShortcut("ctrl+1").command != sidebarTabCommands[0] ||
		m.resolveWorkbenchShortcut("f8").command != sidebarTabCommands[1] ||
		m.resolveWorkbenchShortcut("ctrl+3").command != sidebarTabCommands[2] ||
		m.resolveWorkbenchShortcut("f9").command != "" ||
		m.resolveWorkbenchShortcut("f10").command != "" {
		t.Fatal("invalid entries interfered with defaults or valid overrides")
	}
}
