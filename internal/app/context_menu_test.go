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

func TestContextMenuSplitsCompactFolders(t *testing.T) {
	for _, open := range []bool{false, true} {
		t.Run(map[bool]string{false: "collapsed", true: "expanded"}[open], func(t *testing.T) {
			m := explorerWorkspace(t)
			for _, path := range []string{"parent/chain/leaf", "other/chain"} {
				if err := os.MkdirAll(filepath.Join(m.repo.Root, path), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(m.repo.Root, "parent/chain/leaf/file.go"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			m.expanded["parent/chain/leaf"] = open
			drain(t, m, m.refreshTree())
			for i, entry := range m.items {
				if entry.folder == "parent/chain/leaf" {
					m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: headerHeight + i, Button: tea.MouseRight})
					break
				}
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModShift})
			drain(t, m, cmd)
			assertSplit := func() {
				t.Helper()
				for path, depth := range map[string]int{"parent": 1, "parent/chain": 2, "parent/chain/leaf": 3} {
					found := false
					for _, entry := range m.items {
						if entry.folder == path && entry.label == filepath.Base(path) && entry.depth == depth {
							found = true
						}
					}
					if !found {
						t.Fatalf("missing split directory %s: %+v", path, m.items)
					}
				}
				fileVisible, otherCompact := false, false
				for _, entry := range m.items {
					fileVisible = fileVisible || entry.file == "parent/chain/leaf/file.go"
					otherCompact = otherCompact || entry.label == "other/chain"
				}
				if fileVisible != open || !otherCompact || m.current().folder != "parent/chain/leaf" {
					t.Fatal("split changed the leaf state, selection, or unrelated chain")
				}
			}
			assertSplit()
			m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
			for _, a := range m.contextMenu.actions {
				if a.label == "Split Folders" || a.label == "Open / Expand" {
					t.Fatalf("%q offered for an ordinary folder", a.label)
				}
			}
			press(m, "esc")
			drain(t, m, m.refreshTree())
			assertSplit()
			drain(t, m, m.refresh())
			assertSplit()
			drain(t, m, m.openSelected())
			drain(t, m, m.openSelected())
			assertSplit()
			drain(t, m, m.collapseFolders())
			for _, entry := range m.items {
				if entry.depth > 1 || entry.folder == "parent/chain/leaf" {
					t.Fatal("collapse all exposed children or recompressed the split chain")
				}
			}
			reopened := New(t.Context(), m.repo, false)
			t.Cleanup(reopened.Close)
			reopened.section = explorer
			drain(t, reopened, reopened.refreshTree())
			found := false
			for _, entry := range reopened.items {
				found = found || entry.folder == "parent/chain/leaf" && entry.label == "parent/chain/leaf"
			}
			if !found {
				t.Fatal("reopened workspace retained the split state")
			}
		})
	}
}

func chooseContextAction(t *testing.T, m *Model, label string) tea.Cmd {
	t.Helper()
	if m.contextMenu == nil {
		t.Fatal("menu not open")
	}
	for i, a := range m.contextMenu.actions {
		if a.label == label {
			m.contextMenu.selected = i
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return cmd
		}
	}
	t.Fatalf("missing action %q", label)
	return nil
}

func TestContextMenuPreservesSelectionAndConsumesInput(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.document().SelectAll()
	selection := m.document().SelectedText()
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	if m.contextMenu == nil || m.document().SelectedText() != selection {
		t.Fatal("right click changed text selection")
	}
	before := m.document().Text
	press(m, "x")
	m.Update(tea.PasteMsg{Content: "unexpected paste"})
	if m.document().Text != before {
		t.Fatal("menu input reached editor")
	}
	lines := strings.Split(m.View().Content, "\n")
	layout := m.contextMenuLayout()
	top := plain(ansi.Cut(lines[layout.y], layout.x, layout.x+layout.width))
	bottom := plain(ansi.Cut(lines[layout.y+layout.count+1], layout.x, layout.x+layout.width))
	if !strings.HasPrefix(top, "╭") || !strings.HasSuffix(top, "╮") ||
		!strings.HasPrefix(bottom, "╰") || !strings.HasSuffix(bottom, "╯") {
		t.Fatalf("context menu has no rounded frame: %q / %q", top, bottom)
	}
	m.Update(tea.MouseMotionMsg{X: layout.x + 2, Y: layout.y + 2})
	if m.contextMenu.selected != 1 {
		t.Fatal("menu hover did not select the second action")
	}
	assertFreshHoverFrame(t, m)
	m.Update(tea.MouseMotionMsg{X: layout.x + 3, Y: layout.y + 2})
	assertFreshHoverFrame(t, m)
	press(m, "esc")
	if m.contextMenu != nil {
		t.Fatal("escape did not dismiss menu")
	}
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	m.Update(tea.MouseClickMsg{X: 0, Y: m.height - 1, Button: tea.MouseLeft})
	if m.contextMenu != nil || m.sidebar {
		t.Fatal("outside click leaked through")
	}
}

func TestContextMenuHorizontalAliasesAndWorkbenchLaunchers(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	calls := 0
	open := func() {
		m.contextMenu = &contextMenu{actions: []action{{label: "Test", run: func() tea.Cmd {
			calls++
			return nil
		}}}}
	}
	open()
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if calls != 0 || m.contextMenu != nil {
		t.Fatal("Left did not close the context menu")
	}
	open()
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if calls != 1 || m.contextMenu != nil {
		t.Fatal("Right did not run the selected context-menu action")
	}
	open()
	press(m, "ctrl+g")
	if m.contextMenu != nil || m.modal != "quick" || m.commandPicker() {
		t.Fatal("Ctrl+G did not leave the context menu and open Go to File")
	}
}

func TestContextMenuRunsDisplayedEditorShortcuts(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	m.document().Insert("// saved from menu\n")
	m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
	if m.contextMenu == nil {
		t.Fatal("missing editor menu")
	}
	press(m, "ctrl+c")
	if m.contextMenu == nil || m.notice != "Select text first" {
		t.Fatal("Ctrl+C bypassed the disabled copy action")
	}
	drain(t, m, press(m, "ctrl+s"))
	saved, err := os.ReadFile(m.document().Path)
	if err != nil || m.contextMenu != nil || m.document().Dirty() || string(saved) != m.document().Text {
		t.Fatalf("Ctrl+S did not save from the menu, read error: %v", err)
	}
}

func TestSelectAllSharesEditorCommandAcrossEntries(t *testing.T) {
	for _, entry := range []string{"shortcut", "menu", "palette"} {
		t.Run(entry, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			m.openFile("file0.go")
			press(m, "d")
			switch entry {
			case "shortcut":
				press(m, "ctrl+a")
			case "menu":
				m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
				chooseContextAction(t, m, "Select All")
			case "palette":
				press(m, "ctrl+p")
				press(m, "edit.selectAll")
				press(m, "enter")
			}
			d := m.document()
			if d.SelectedText() != d.Text || m.tabs[m.activeTab].vim.CommandPending() {
				t.Fatal("Select All did not select the document and clear the pending Vim command")
			}
		})
	}
}

func TestContextMenuFitsAndScrolls(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.width, m.height = 30, 8
	m.sidebar = false
	m.resize()
	m.Update(tea.MouseClickMsg{X: 29, Y: 6, Button: tea.MouseRight})
	if m.contextMenu == nil {
		t.Fatal("menu missing at edge")
	}
	press(m, "end")
	l := m.contextMenuLayout()
	if l.x < 0 || l.y < 0 || l.x+l.width > m.width || l.y+l.count+2 > m.height || m.contextMenu.top == 0 {
		t.Fatalf("invalid layout: %+v", l)
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatalf("menu frame overflow: %q", plain(line))
		}
	}
	press(m, "esc")
	m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
	if m.contextMenu == nil {
		t.Fatal("keyboard menu did not open")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	if m.contextMenu != nil {
		t.Fatal("resize retained menu")
	}
}

func TestContextMenuTargetsClickedTabAndConfirmsDirtyClose(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.openFile("file0.go")
	first := m.document()
	first.Insert("dirty")
	m.openFile("file1.go")
	second := m.document()
	m.sidebar = true
	slot := m.tabLayout()[0]
	m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseRight})
	if m.document() != second || !m.sidebar {
		t.Fatal("opening a tab menu changed the active tab or focus")
	}
	press(m, "ctrl+w")
	if m.contextMenu != nil || !hasSaveConfirmation(m) || len(m.closeQueue) != 1 || m.closeQueue[0] != first || len(m.tabs) != 2 {
		t.Fatal("close did not confirm closing the clicked dirty tab")
	}
	m.chooseClose("cancel")
	m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseRight})
	chooseContextAction(t, m, "Close Others")
	if len(m.tabs) != 1 || m.document() != first {
		t.Fatal("close others did not keep the clicked tab")
	}
}

func TestRepositoryContextMenuOpensBranchPicker(t *testing.T) {
	m, _ := childWorkspace(t)
	// This test exercises the PR repository menu without fetching PRs.
	m.snapshot.Remotes = []gitservice.Remote{{Name: "origin", URL: "https://github.com/example/repo.git"}}
	for _, tt := range []struct {
		name       string
		section    section
		key        string
		repository string
	}{
		{"Changes", changes, repositoryKey("child space"), "child space"},
		{"History", history, historyRepositoryKey("child space"), "child space"},
		{"Pull Requests", pullRequests, pullRequestRepositoryKey, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m.selectSection(tt.section)
			m.selected = m.sidebarRowWithKey(tt.key)
			if m.selected < 0 {
				t.Fatal("missing repository row")
			}
			m.ensureSidebar()
			m.Update(tea.MouseClickMsg{X: m.sideX() + 3, Y: m.sidebarSelectionY(), Button: tea.MouseRight})
			if m.contextMenu == nil || m.contextMenu.actions[0].label != "Switch Branch…" {
				t.Fatal("repository context menu does not start with Switch Branch")
			}
			for _, a := range m.contextMenu.actions {
				if a.label == "Open / Expand" {
					t.Fatal("repository context menu includes Open / Expand")
				}
			}
			drain(t, m, chooseContextAction(t, m, "Switch Branch…"))
			if m.modal != "refs" || m.branchRepository != tt.repository {
				t.Fatal("context menu did not open the selected repository branch picker")
			}
			press(m, "esc")
		})
	}
}

func TestContextMenuChangeActionsAndStaleTargets(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	_, y := hoverAction(t, m, "cmd/main.go", "stage")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	if m.contextMenu == nil {
		t.Fatal("missing change row context menu")
	}
	for _, a := range m.contextMenu.actions {
		if a.label == "Open Changes" {
			t.Fatal("change row context menu repeats the default diff action")
		}
	}
	drain(t, m, chooseContextAction(t, m, "Stage Changes"))
	if m.current() == nil || !m.current().staged || m.current().change.Path != "cmd/main.go" {
		t.Fatal("stage targeted wrong row")
	}
	_, y = hoverAction(t, m, "README.md", "discard")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	press(m, "d")
	if m.modal != "confirm" {
		t.Fatal("discard bypassed confirmation")
	}
	m.modal = ""
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	m.items = nil
	if cmd := chooseContextAction(t, m, "Stage Changes"); cmd != nil {
		t.Fatal("stale menu scheduled mutation")
	}
	if !strings.Contains(m.notice, "Target changed") {
		t.Fatal("stale target not explained")
	}
}

func TestDiffContextMenuStagesAndUnstagesMouseSelection(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	run := func(args ...string) string {
		t.Helper()
		out, err := m.repo.Run(m.ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.test")
	path := filepath.Join(m.repo.Root, "README.md")
	before, after := "first\nsecond\nlast\n", "FIRST\nSECOND\nlast\n"
	if err := os.WriteFile(path, []byte(before), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "README.md")
	run("commit", "--no-gpg-sign", "-m", "initial")
	if err := os.WriteFile(path, []byte(after), 0644); err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 32})
	drain(t, m, m.refresh())
	drain(t, m, m.selectSection(changes))
	open := func(staged bool) {
		t.Helper()
		m.sidebar = true
		selectChanged(t, m, "README.md", staged)
		drain(t, m, m.openSelected())
	}
	selectLine := func(line string) (int, int) {
		t.Helper()
		if !m.diffLayout.split {
			t.Fatal("expected a split diff")
		}
		for i, pair := range m.diffRows {
			if pair.Right == nil || pair.Right.Text != line {
				continue
			}
			pane, _ := m.diffPane(true)
			x := m.mainX() + pane + m.diffGutter
			y := m.contentRow() + i - m.viewport.YOffset()
			m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			m.Update(tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
			return x + 2, y
		}
		t.Fatalf("diff row not found: %s", line)
		return 0, 0
	}
	menuAt := func(line string) {
		x, y := selectLine(line)
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	}
	clickButton := func(x, y int, discard bool, hint string) tea.Cmd {
		t.Helper()
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		buttons := m.hoveredDiffButtons()
		if !buttons.visible {
			t.Fatal("hovered hunk has no buttons")
		}
		pos, glyph := buttons.stage, "+"
		if discard {
			pos, glyph = buttons.discard, "→"
		} else if m.diffStaged {
			glyph = "−"
			if buttons.discard.width != 0 {
				t.Fatal("staged hunk exposed a discard button")
			}
		}
		x, y = m.mainX()+pos.x, pos.y
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		line := strings.Split(m.View().Content, "\n")[y]
		if strings.TrimSpace(plain(ansi.Cut(line, x, x+pos.width))) != glyph || m.hoverHint() != hint {
			t.Fatalf("unexpected diff button or hint at %d,%d: %q / %q", x, y, plain(line), m.hoverHint())
		}
		_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		return cmd
	}
	open(false)
	x, y := selectLine("+FIRST")
	drain(t, m, clickButton(x, y, false, "Stage Selected Ranges"))
	if got := run("show", ":README.md"); got != "FIRST\nsecond\nlast\n" {
		t.Fatalf("index after staging selection: %q", got)
	}
	open(true)
	x, y = selectLine("+FIRST")
	drain(t, m, clickButton(x, y, false, "Unstage Selected Ranges"))
	if got := run("show", ":README.md"); got != before {
		t.Fatalf("index after unstaging selection: %q", got)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != after {
		t.Fatalf("working file changed: %q, %v", got, err)
	}

	open(false)
	selectLine("+FIRST")
	drain(t, m, press(m, "s"))
	if got := run("show", ":README.md"); got != "FIRST\nsecond\nlast\n" {
		t.Fatalf("shortcut staged outside the selection: %q", got)
	}
	open(true)
	menuAt("+FIRST")
	drain(t, m, chooseContextAction(t, m, "Unstage Selected Ranges"))
	open(false)
	menuAt("+FIRST")
	drain(t, m, chooseContextAction(t, m, "Stage Selected Ranges"))
	selectLine("+SECOND")
	drain(t, m, press(m, "d"))
	if m.modal != "confirm" || m.busy {
		t.Fatal("partial discard must require confirmation")
	}
	drain(t, m, press(m, "esc"))
	if got, err := os.ReadFile(path); err != nil || string(got) != after {
		t.Fatalf("cancel changed the working file: %q, %v", got, err)
	}
	drain(t, m, clickButton(m.mainX()+2, m.contentRow(), true, "Discard Selected Ranges"))
	drain(t, m, press(m, "enter"))
	const staged = "FIRST\nsecond\nlast\n"
	if got, err := os.ReadFile(path); err != nil || string(got) != staged {
		t.Fatalf("working file after discarding hunk: %q, %v", got, err)
	}
	if got := run("show", ":README.md"); got != staged {
		t.Fatalf("discard changed the index: %q", got)
	}
	if err := os.WriteFile(path, []byte(after), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	open(false)
	if cmd := press(m, "D"); cmd != nil || m.modal != "confirm" || m.busy {
		t.Fatal("whole-file discard did not wait for confirmation")
	}
	press(m, "left")
	drain(t, m, press(m, "enter"))
	if got, err := os.ReadFile(path); err != nil || string(got) != staged {
		t.Fatalf("whole-file discard did not restore the index version: %q, %v", got, err)
	}
}

func TestContextMenuMouseAndDisabledActions(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.Update(tea.MouseClickMsg{X: m.mainX() + 3, Y: 4, Button: tea.MouseRight})
	chooseContextAction(t, m, "Copy")
	if m.contextMenu == nil || m.notice != "Select text first" {
		t.Fatal("disabled action executed or lost its explanation")
	}
	for i, a := range m.contextMenu.actions {
		if a.label != "Select All" {
			continue
		}
		l := m.contextMenuLayout()
		m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 1 + i, Button: tea.MouseLeft})
		if m.contextMenu != nil || m.document().SelectedText() == "" {
			t.Fatal("mouse did not execute menu action")
		}
		return
	}
	t.Fatal("select all missing")
}

func TestExplorerShiftLSplitsCompactFolder(t *testing.T) {
	m := explorerWorkspace(t)
	t.Cleanup(m.Close)
	if err := os.MkdirAll(filepath.Join(m.repo.Root, "parent", "child"), 0755); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refreshTree())
	for i, entry := range m.items {
		if entry.label == "parent/child" {
			m.selected = i
			break
		}
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModShift})
	drain(t, m, cmd)
	if entry := m.current(); entry == nil || entry.label != "child" || entry.depth != 2 || m.expanded["parent/child"] {
		t.Fatal("Shift+L did not split the compact folder while preserving the selected leaf")
	}
}
