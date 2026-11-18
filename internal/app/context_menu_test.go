package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
			drain(t, m, chooseContextAction(t, m, "Split Folders"))
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
			press(m, "shift+f10")
			for _, a := range m.contextMenu.actions {
				if a.label == "Split Folders" {
					t.Fatal("split action offered for an ordinary folder")
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
	press(m, "shift+f10")
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
	chooseContextAction(t, m, "Close")
	if m.contextMenu != nil || m.modal != "close" || len(m.closeQueue) != 1 || m.closeQueue[0] != first || len(m.tabs) != 2 {
		t.Fatal("close did not confirm closing the clicked dirty tab")
	}
	m.chooseClose("cancel")
	m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseRight})
	chooseContextAction(t, m, "Close Others")
	if len(m.tabs) != 1 || m.document() != first {
		t.Fatal("close others did not keep the clicked tab")
	}
}

func TestContextMenuChangeActionsAndStaleTargets(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	_, y := hoverAction(t, m, "cmd/main.go", "stage")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	drain(t, m, chooseContextAction(t, m, "Stage Changes"))
	if m.current() == nil || !m.current().staged || m.current().change.Path != "cmd/main.go" {
		t.Fatal("stage targeted wrong row")
	}
	_, y = hoverAction(t, m, "README.md", "discard")
	m.Update(tea.MouseClickMsg{X: m.sideX() + 4, Y: y, Button: tea.MouseRight})
	chooseContextAction(t, m, "Delete Untracked File")
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
	menuAt := func(line string) {
		t.Helper()
		if !m.diffLayout.split {
			t.Fatal("expected a split diff")
		}
		for i, pair := range m.diffRows {
			if pair.Right == nil || pair.Right.Text != line {
				continue
			}
			x := m.mainX() + (m.mainWidth()-1)/2 + 1 + m.diffGutter
			y := m.contentRow() + i - m.viewport.YOffset()
			m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			m.Update(tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
			m.Update(tea.MouseClickMsg{X: x + 2, Y: y, Button: tea.MouseRight})
			return
		}
		t.Fatalf("diff row not found: %s", line)
	}
	open(false)
	menuAt("+FIRST")
	drain(t, m, chooseContextAction(t, m, "Stage Selected Ranges"))
	if got := run("show", ":README.md"); got != "FIRST\nsecond\nlast\n" {
		t.Fatalf("index after staging selection: %q", got)
	}
	open(true)
	menuAt("+FIRST")
	drain(t, m, chooseContextAction(t, m, "Unstage Selected Ranges"))
	if got := run("show", ":README.md"); got != before {
		t.Fatalf("index after unstaging selection: %q", got)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != after {
		t.Fatalf("working file changed: %q, %v", got, err)
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
