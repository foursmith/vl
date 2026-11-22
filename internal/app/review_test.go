package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
)

func reviewWorkspace(t *testing.T, split bool) *Model {
	t.Helper()
	m := tabWorkspace(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 18})
	showDiffFixture(m)
	m.diffPath = "file.go"
	m.split = split
	m.patch = diff.Parse("diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n-old value\n+new value\n+new extra\n shared text\n final text\n@@ -20 +21 @@\n-before end\n+after end\n")
	m.renderDiff()
	m.sidebar = false
	t.Cleanup(m.Close)
	return m
}

func TestReadOnlyDiffSharesVimMotionsSelectionAndSearch(t *testing.T) {
	for _, split := range []bool{false, true} {
		m := reviewWorkspace(t, split)
		original := m.review.Text
		editable := tabWorkspace(t, 1)
		defer editable.Close()
		editable.openFile("file0.go")
		editable.tabs[editable.activeTab].doc = &editor.Document{Text: original, Anchor: -1}
		for _, k := range []string{"w", "e", "b", "2", "j", "G", "g", "g", "v", "w", "l"} {
			press(m, k)
			press(editable, k)
			if m.review.Cursor != editable.document().Cursor || m.review.SelectedText() != editable.document().SelectedText() {
				t.Fatalf("split=%v key=%s: readonly (%d,%q), editable (%d,%q)", split, k, m.review.Cursor, m.review.SelectedText(), editable.document().Cursor, editable.document().SelectedText())
			}
		}
		if m.diffSelectedText() != m.review.SelectedText() {
			t.Fatalf("rendered selection differs: %q != %q", m.diffSelectedText(), m.review.SelectedText())
		}
		selected := m.review.SelectedText()
		press(m, "y")
		if m.vimRegister != selected {
			t.Fatal("Visual yank did not set the unnamed register")
		}
		if m.tabs[m.activeTab].vim.VisualMode || m.diffSelection.active {
			t.Fatal("yank retained Visual selection")
		}
		for _, k := range []string{"i", "X", "d", "d", "p", "u", "ctrl+v", "ctrl+s"} {
			press(m, k)
		}
		m.Update(tea.PasteMsg{Content: "cannot modify"})
		if m.review.Text != original || m.review.Dirty() || m.tabs[m.activeTab].vim.InsertMode || m.busy {
			t.Fatal("read-only view changed source or started a Git operation")
		}
		press(m, "/")
		press(m, "new")
		press(m, "enter")
		if m.modal != "" || m.review.SelectedText() != "new" {
			t.Fatal("shared search did not select source")
		}
		press(m, "n")
		if m.review.SelectedText() != "new" {
			t.Fatal("n did not continue text search")
		}
		press(m, "ctrl+f")
		press(m, "esc")
		if !m.findHidden {
			t.Fatal("closing Find did not hide search highlights")
		}
		if m.View().Cursor == nil {
			t.Fatal("read-only editor has no cursor")
		}
	}
}

func TestDiffKeyboardCursorStaysOnRightWhileSelectingLeftText(t *testing.T) {
	m := reviewWorkspace(t, true)
	m.review.SetPosition(2, 3)
	m.ensureTextCursor()
	initial := m.reviewCursor()
	right, _ := m.diffPane(true)
	if initial == nil || initial.X < m.mainX()+right {
		t.Fatal("side-by-side diff did not start on the right")
	}
	for _, msg := range []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: tea.KeyTab, Mod: tea.ModShift}} {
		m.Update(msg)
		if cursor := m.reviewCursor(); cursor == nil || cursor.Position != initial.Position {
			t.Fatal("Tab moved the diff cursor")
		}
	}
	for i, pair := range m.diffRows {
		if codeLine(pair.Left) != "old value" {
			continue
		}
		x, y := m.mainX()+m.diffGutter, m.contentRow()+i-m.viewport.YOffset()
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		m.Update(tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
		break
	}
	if cursor := m.reviewCursor(); cursor == nil || cursor.Position != initial.Position ||
		m.diffSelectedText() != "old" || m.review.SelectedText() != "" {
		t.Fatal("left mouse selection changed keyboard focus or lost selected text")
	}
	if cmd := m.copy(false); cmd == nil {
		t.Fatal("left mouse selection was not available to copy")
	}
}

func TestReadOnlyDiffFlashAndLeader(t *testing.T) {
	m := reviewWorkspace(t, true)
	original := m.review.Text
	press(m, "f")
	m.Update(tea.PasteMsg{Content: "new"})
	if m.flash == nil || len(m.flash.targets) != 2 {
		t.Fatal("Diff did not use shared Flash targets")
	}
	if !strings.Contains(m.statusLine(), "FLASH") {
		t.Fatal("Flash status is missing")
	}
	m.View() // Paint labels through the Diff source-row adapter.
	offset := m.flash.targets[1].offset
	press(m, m.flash.targets[1].label)
	if m.flash != nil || m.review.Cursor != offset || m.review.Text != original {
		t.Fatal("Flash changed text or failed to navigate")
	}
	press(m, " ")
	if !strings.Contains(m.statusLine(), "LEADER") {
		t.Fatal("Diff leader has no shared state display")
	}
	press(m, "esc")
}

func TestSharedLeaderCancelsAcrossDialogsAndBlur(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		m := reviewWorkspace(t, false)
		if !readOnly {
			m.tabs = append(m.tabs, tab{doc: &editor.Document{Text: "text", Anchor: -1}})
			m.activateTab(len(m.tabs) - 1)
		}
		count := len(m.tabs)
		press(m, " ")
		m.Update(tea.BlurMsg{})
		press(m, "q")
		if len(m.tabs) != count {
			t.Fatal("leader survived window blur")
		}
		press(m, " ")
		press(m, "ctrl+p")
		if m.modal != "quick" || m.leaderTarget != "" {
			t.Fatal("command palette did not cancel leader")
		}
		press(m, "esc")
		press(m, "q")
		if len(m.tabs) != count {
			t.Fatal("leader survived a dialog")
		}
		for _, key := range []string{"y", "p"} {
			original := m.editorDocument().Text
			press(m, " ")
			if cmd := press(m, key); cmd != nil {
				t.Fatal("unknown leader key produced a command")
			}
			if m.editorDocument().Text != original || m.tabs[m.activeTab].vim.CommandPending() {
				t.Fatal("unknown leader key leaked into Vim")
			}
		}
	}
}

func TestReadOnlyDiffHunkNavigationSelectsDeletionWithoutMovingCursorLeft(t *testing.T) {
	m := reviewWorkspace(t, true)
	m.patch = diff.Parse("@@ -1 +1 @@\n-old\n+new\n@@ -20 +19,0 @@\n-deleted only\n")
	m.renderDiff()
	m.diffHunk = 0
	press(m, "]")
	if m.diffHunk != 1 || !strings.Contains(plain(m.View().Content), "deleted only") {
		t.Fatal("hunk without new-side source was not selected and revealed")
	}
	row, _ := m.review.Position()
	if m.review.Lines()[row] != "new" {
		t.Fatal("hunk navigation moved keyboard focus away from the right pane")
	}
}
