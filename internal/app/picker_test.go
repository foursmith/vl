package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestQuickFilesToggleIgnoreKeepsQuery(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	const path = "examples/artifact.txt"
	if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("example"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: "artifact"})
	if len(m.filteredActions()) != 0 {
		t.Fatal("ignored file appeared with ignore rules enabled")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF4})
	if !m.fileListLoading || m.input.Value() != "artifact" {
		t.Fatal("toggling ignore did not start a scan with the same query")
	}
	drain(t, m, cmd)
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != path {
		t.Fatalf("ignored file did not appear: %+v", rows)
	}
	control := m.pickerIgnoreControl(m.pickerLayout(rows))
	lines := strings.Split(plain(m.View().Content), "\n")
	if !strings.Contains(lines[control.y], m.symbol(symbolIgnoreRules)) {
		t.Fatal("ignore rules control did not render in the query row")
	}
	m.Update(tea.MouseMotionMsg{X: control.x, Y: control.y})
	if !strings.Contains(m.hoverHint(), "F4") {
		t.Fatal("ignore rules control did not advertise its shortcut")
	}
	_, cmd = m.Update(tea.MouseClickMsg{X: control.x, Y: control.y, Button: tea.MouseLeft})
	drain(t, m, cmd)
	if m.fileListIncludeIgnored || m.input.Value() != "artifact" || len(m.filteredActions()) != 0 {
		t.Fatal("clicking the toggle did not restore ignore rules")
	}
}

func TestQuickFileRevealsTreeAndKeepsEditorFocus(t *testing.T) {
	m := explorerWorkspace(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 8})
	path := "cmd/nested/main.go"
	if err := os.MkdirAll(filepath.Join(m.repo.Root, "cmd/nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.section = changes
	m.sidebarVisible = false
	m.snapshot.Files = []string{path}
	m.openQuickFiles()
	drain(t, m, press(m, "enter"))
	if m.section != explorer || !m.sidebarVisible || m.sidebar || !m.expanded["cmd"] || !m.expanded["cmd/nested"] {
		t.Fatal("file search did not reveal ancestors while preserving editor focus")
	}
	if entry := m.current(); entry == nil || entry.file != path {
		t.Fatalf("tree selection: %+v", entry)
	}
	if m.selected < m.sideTop || m.selected >= m.sideTop+m.bodyHeight()-headerHeight {
		t.Fatal("revealed file is outside the sidebar viewport")
	}
	// A later navigation must win over an in-flight reveal.
	cmd := m.openFileInExplorer(path)
	m.selectSection(history)
	drain(t, m, cmd)
	if m.section != history || m.revealFile != "" {
		t.Fatal("late tree response restored the old reveal")
	}
}

func TestFailedFileOpenDoesNotRevealPreviousDocument(t *testing.T) {
	m := explorerWorkspace(t)
	m.openFile("README.md")
	doc := m.document()
	drain(t, m, m.selectSection(changes))
	if err := os.WriteFile(filepath.Join(m.repo.Root, "binary.dat"), []byte{0, 1, 2}, 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"missing.go", "binary.dat"} {
		if cmd := m.openFileInExplorer(path); cmd != nil || m.section != changes || m.document() != doc || m.revealFile != "" {
			t.Fatalf("failed open changed Explorer state: %s", path)
		}
	}
}

func TestQuickPickerShortcutsAndFileSelection(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.snapshot.Files = []string{"file0.go", "file1.go"}
	press(m, "ctrl+g")
	if m.modal != "quick" || m.input.Prompt != "" {
		t.Fatal("Ctrl+G must open file search")
	}
	if !strings.Contains(plain(m.View().Content), m.tr("Go to File (name or path, add :line or :line:column)")) {
		t.Fatal("file picker must display its name and line/column syntax")
	}
	m.input.SetValue("file1")
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "file1.go" {
		t.Fatalf("file filtering: %+v", rows)
	}
	m.View()
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 1, Button: tea.MouseLeft})
	if m.modal != "quick" {
		t.Fatal("clicking input dismissed file search")
	}
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
	if m.modal != "" || m.document() == nil || !strings.HasSuffix(m.document().Path, "file1.go") {
		t.Fatal("file result click did not open the filtered file")
	}
	press(m, "ctrl+p")
	if m.modal != "quick" || m.input.Prompt != "" || m.input.Value() != ">" || len(m.palette) != len(m.actions()) {
		t.Fatal("Ctrl+P must open commands from editor focus")
	}
}

func TestQuickPickerGeometryAndScrolledClick(t *testing.T) {
	m := tabWorkspace(t, 0)
	for _, width := range []int{30, 48, 120} {
		for _, height := range []int{8, 24} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: height})
			m.openPalette()
			m.selectPickerIndex(len(m.palette) - 1)
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != width {
					t.Fatalf("picker width at %dx%d: %d", width, height, ansi.StringWidth(line))
				}
			}
		}
	}
	executed := false
	m.palette[len(m.palette)-1] = action{label: "Last command", run: func() tea.Cmd { executed = true; return nil }}
	l := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3 + l.count - 1, Button: tea.MouseLeft})
	if !executed || m.modal != "" {
		t.Fatal("scrolled picker click executed the wrong command")
	}
}

func TestCommandPaletteGitHintsRequireChangesFocus(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	hints := map[string]string{
		"git.commit": gitKeys.Commit.Help().Key,
		"git.stash":  gitKeys.Stash.Help().Key,
		"git.fetch":  gitKeys.Fetch.Help().Key,
		"git.pull":   gitKeys.Pull.Help().Key,
		"git.push":   gitKeys.Push.Help().Key,
	}
	for _, view := range []struct {
		section section
		sidebar bool
	}{{changes, true}, {changes, false}, {explorer, true}} {
		m.section, m.sidebar = view.section, view.sidebar
		for _, action := range m.commandActions() {
			want, checked := hints[action.id]
			if !checked {
				continue
			}
			if m.section != changes || !m.sidebar {
				want = ""
			}
			if action.key != want {
				t.Errorf("%s advertises %q in section %v with sidebar focus %t, want %q", action.id, action.key, view.section, view.sidebar, want)
			}
		}
	}
}

func TestCommandPaletteOmitsLocalChangeAndDiffActions(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	change := &gitservice.Change{Path: "file0.go", Index: ' ', Worktree: 'M'}
	m.section, m.sidebar, m.selected = changes, true, 0
	m.items = []item{{key: "file0.go", change: change}}
	for _, a := range m.commandActions() {
		switch a.id {
		case "git.stage", "git.unstage", "git.discard", "git.discardAll", "file.open", "explorer.collapseAll", "search.options":
			t.Errorf("local action %q listed in the command palette", a.id)
		}
	}
	showDiffFixture(m)
	m.sidebar, m.diffChange = false, change
	for _, a := range m.commandActions() {
		if strings.HasPrefix(a.id, "diff.") && a.id != string(commandDiffLayout) {
			t.Errorf("local diff action %q listed in the command palette", a.id)
		}
	}
}

func TestPRFilterContextMenuKeepsListPosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.section, m.sidebar, m.sideTop = pullRequests, true, 7
	m.items = make([]item, 8)
	m.items[7] = item{repoHeader: true, key: "repo"}

	m.openPalette()
	m.input.SetValue(">Filter Pull Requests")
	if rows := m.filteredActions(); len(rows) != 0 {
		t.Fatalf("PR filter still listed in the command palette: %+v", rows)
	}
	press(m, "esc")
	m.openContextMenu(m.sideX()+2, headerHeight)
	drain(t, m, chooseContextAction(t, m, "Filter Pull Requests"))
	if m.modal != "pullRequestFilter" {
		t.Fatal("context menu did not open the PR filter")
	}
	press(m, "esc")
	if m.sideTop != 7 {
		t.Fatalf("opening and cancelling the PR filter reset the list scroll to %d", m.sideTop)
	}
}

func TestCommandPaletteAlphabeticalOrderAndLastUsed(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	checkSorted := func(rows []action) {
		t.Helper()
		for i := 1; i < len(rows); i++ {
			if strings.ToLower(rows[i-1].label) > strings.ToLower(rows[i].label) {
				t.Fatalf("commands out of order: %q before %q", rows[i-1].label, rows[i].label)
			}
		}
	}
	m.lastCommand = "removed.command"
	m.openPalette()
	checkSorted(m.filteredActions())
	m.input.SetValue(">Appearance: Toggle Sidebar Visibility")
	m.selectPickerChoice()
	if m.lastCommand != string(commandSidebar) {
		t.Fatalf("executed command was not remembered: %q", m.lastCommand)
	}
	m.openPalette()
	rows := m.filteredActions()
	if rows[0].id != string(commandSidebar) {
		t.Fatalf("last command not first: %q", rows[0].id)
	}
	checkSorted(rows[1:])
	m.input.SetValue(">file.save")
	m.selectPickerChoice()
	if m.lastCommand != string(commandSidebar) {
		t.Fatal("disabled command replaced last used command")
	}
	m.snapshot.Files = []string{"file0.go"}
	m.openQuickFiles()
	m.input.SetValue("file0.go")
	m.selectPickerChoice()
	if m.lastCommand != string(commandSidebar) {
		t.Fatal("file picker selection replaced last used command")
	}
}

func TestQuickPickerSwitchesWithEditableCommandPrefix(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.snapshot.Files = []string{"file0.go"}
	m.openPalette()
	m.selectPickerIndex(5)
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	rows := m.filteredActions()
	if m.modal != "quick" || m.input.Value() != "" || m.input.Prompt != "" || m.pickerIndex != 0 || len(rows) != 1 || rows[0].id != "file0.go" {
		t.Fatal("deleting > did not switch to file search and reset selection")
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "file0"})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "file0.go" {
		t.Fatal("typing did not filter file results")
	}
	drain(t, m, press(m, "enter"))
	if m.document() == nil || m.lastCommand != "" {
		t.Fatal("file selection did not open the file or changed command history")
	}
	m.openQuickFiles()
	m.Update(tea.KeyPressMsg{Code: '>', Text: ">"})
	if !m.commandPicker() || len(m.filteredActions()) != len(m.actions()) {
		t.Fatal("typing > did not switch to commands")
	}
	m.Update(tea.PasteMsg{Content: "Appearance: Toggle Sidebar Visibility"})
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != string(commandSidebar) {
		t.Fatal("command prefix was included in the search query")
	}
	drain(t, m, press(m, "enter"))
	if m.lastCommand != string(commandSidebar) {
		t.Fatal("command selection did not update history")
	}
	m.openQuickFiles()
	m.Update(tea.PasteMsg{Content: ">Appearance: Toggle Sidebar Visibility"})
	if rows := m.filteredActions(); !m.commandPicker() || len(rows) != 1 || rows[0].id != string(commandSidebar) {
		t.Fatal("pasting > did not switch modes")
	}
	press(m, "ctrl+g")
	if m.commandPicker() || m.input.Value() != "" {
		t.Fatal("Ctrl+G did not reset to file search")
	}
	press(m, "ctrl+p")
	if !m.commandPicker() || m.input.Value() != ">" {
		t.Fatal("Ctrl+P did not reset to command search")
	}
}
