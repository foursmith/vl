package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorFoldRenderingNavigationAndReveal(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	text := "func outer() {\n\tif true {\n\t\tprintln(123)\n\t}\n}\nlast"
	d.Edit(text)
	d.SetPosition(1, 0)
	press(m, "z")
	press(m, "a")
	state := &m.tabs[m.activeTab]
	if len(state.folds) != 1 || state.sourceRow(2) != 4 || d.Text != text {
		t.Fatal("za did not fold the inner block without editing source")
	}
	paint := plain(strings.Join(m.editorLines(), "\n"))
	if strings.Contains(paint, "println") || !strings.Contains(paint, "… 2 lines") {
		t.Fatal("folded content or hidden line count rendered incorrectly", paint)
	}
	press(m, "j")
	if row, _ := d.Position(); row != 4 || d.Anchor >= 0 {
		t.Fatal("j did not skip folded lines, or created a selection")
	}
	if cursor := m.editorCursor(); cursor == nil || cursor.Y != m.contentRow()+2 {
		t.Fatal("cursor did not use the folded display row")
	}
	m.positionEditor(m.mainX()+7, m.contentRow()+1, false)
	if row, _ := d.Position(); row != 1 {
		t.Fatal("mouse position did not map to the fold header")
	}
	press(m, "z")
	press(m, "a")
	if len(state.folds) != 0 || state.sourceRow(2) != 2 {
		t.Fatal("za did not reopen the block")
	}
	press(m, "z")
	press(m, "a")
	press(m, "2")
	press(m, "j")
	if row, _ := d.Position(); row != 5 {
		t.Fatal("counted motion did not count visible lines")
	}
	d.SetPosition(1, 0)
	press(m, "v")
	press(m, "j")
	if row, _ := d.Position(); row != 4 || !strings.Contains(d.SelectedText(), "println") {
		t.Fatal("Visual motion did not skip display rows while selecting source text")
	}
	press(m, "esc")
	d.SetPosition(2, 0)
	m.ensureEditor()
	if len(state.folds) != 0 {
		t.Fatal("jump to hidden source did not reveal it")
	}
	d.SetPosition(0, 0)
	press(m, "z")
	press(m, "a")
	press(m, "G")
	press(m, "k")
	if row, _ := d.Position(); row != 0 {
		t.Fatal("reverse motion did not skip the folded outer block")
	}
	press(m, "i")
	press(m, "z")
	if !strings.HasPrefix(d.Text, "zfunc") || len(state.folds) != 0 {
		t.Fatal("Insert z did not insert text and invalidate folds")
	}
}

func TestEditorNestedFoldsRetainState(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Edit("func outer() {\n if true {\n  inner()\n }\n other()\n}\nlast")
	d.SetPosition(1, 0)
	press(m, "z")
	press(m, "a")
	d.SetPosition(0, 0)
	press(m, "z")
	press(m, "a")
	state := &m.tabs[m.activeTab]
	if len(state.folds) != 2 || state.sourceRow(1) != 6 {
		t.Fatal("outer fold did not hide the nested block")
	}
	press(m, "z")
	press(m, "a")
	if len(state.folds) != 1 || state.sourceRow(2) != 4 {
		t.Fatal("reopening the outer block lost the inner fold")
	}
}

func TestEditorFoldsWithSharedClosingAndOpeningLine(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Edit("if ready {\n one()\n} else {\n two()\n}\nafter")
	d.SetPosition(3, 0)
	press(m, "z")
	press(m, "M")
	state := &m.tabs[m.activeTab]
	if len(state.folds) != 2 || state.sourceRow(1) != 5 {
		t.Fatal("zM left a branch visible when its header shares a closing line")
	}
	if row, _ := d.Position(); row != 0 {
		t.Fatal("zM did not move the cursor to the visible branch header")
	}
	press(m, "z")
	press(m, "o")
	if state.sourceRow(2) != 2 || state.sourceRow(3) != 5 {
		t.Fatal("zo did not reveal the second branch's closed header")
	}
}

func TestEditorPythonFoldAndScroll(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Path = "example.py"
	d.Edit("def example():\n    a = 1\n    b = 2\nnext_line\n" + strings.Repeat("tail\n", 40))
	d.SetPosition(2, 0)
	press(m, "z")
	press(m, "a")
	state := &m.tabs[m.activeTab]
	if row, _ := d.Position(); row != 0 || state.sourceRow(1) != 3 {
		t.Fatal("Python fold hid the following sibling or did not select the block header")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelDown})
	if state.top != 5 || m.editorRowAt(0) != 5 {
		t.Fatal("wheel did not advance by visible lines")
	}
	for range 30 {
		m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelDown})
	}
	if state.visibleRow(state.top)+m.contentHeight() != len(state.foldRows) {
		t.Fatal("folded scroll did not clamp to the final full screen")
	}
}

func TestEditorFoldCommandsAndPrefixCancellation(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	text := "func outer() {\n if true {\n  inner()\n }\n other()\n}\nfunc second() {\n next()\n}\nlast"
	d.Edit(text)
	d.SetPosition(0, 0)
	state := &m.tabs[m.activeTab]
	command := func(key string) {
		t.Helper()
		press(m, "z")
		press(m, key)
		if state.vim.CommandPending() || d.Text != text || state.vim.InsertMode {
			t.Fatalf("z%s left a prefix, edited text, or entered Insert", key)
		}
	}
	press(m, "z")
	if !state.vim.CommandPending() || len(state.folds) != 0 || !strings.Contains(state.vim.Label(), "z") {
		t.Fatal("z must wait for its second key and display the prefix")
	}
	press(m, "esc")
	press(m, "j")
	if state.vim.CommandPending() {
		t.Fatal("Escape did not cancel the fold prefix")
	}
	d.SetPosition(0, 0)
	command("c")
	if len(state.folds) != 1 {
		t.Fatal("zc did not close the current level")
	}
	command("o")
	if len(state.folds) != 0 {
		t.Fatal("zo did not open the current level")
	}
	command("C")
	if len(state.folds) != 2 {
		t.Fatal("zC did not close descendants, or affected the sibling function")
	}
	command("o")
	if len(state.folds) != 1 || state.folds[0].start != 1 {
		t.Fatal("zo did not preserve the nested closed block")
	}
	command("O")
	if len(state.folds) != 0 {
		t.Fatal("zO on an open outer block did not reopen its descendants")
	}
	d.SetPosition(2, 0)
	command("M")
	if len(state.folds) != 3 || state.sourceRow(1) != 6 {
		t.Fatal("zM did not close all blocks or relocate the hidden cursor")
	}
	command("o")
	if len(state.folds) != 2 || state.folds[0].start != 1 {
		t.Fatal("zM did not retain nested fold state after zo")
	}
	command("R")
	if len(state.folds) != 0 || state.sourceRow(1) != 1 {
		t.Fatal("zR did not reopen all blocks")
	}
	command("C")
	command("O")
	if len(state.folds) != 0 {
		t.Fatal("zO on a closed block did not reopen all descendants")
	}
	// Unknown suffixes are consumed, rather than interpreted as Insert commands.
	command("i")
	// Pending fold commands cannot survive switching focus to the sidebar.
	press(m, "z")
	press(m, "f6")
	press(m, "f6")
	press(m, "a")
	if !state.vim.InsertMode {
		t.Fatal("fold prefix survived a focus switch")
	}
}
