package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/highlight"
)

func TestHighlightSessionSurvivesEditsAndUndo(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	complete := func() {
		t.Helper()
		request := m.scheduleHighlight()().(highlightStartMsg).request
		m.finishHighlight(m.startHighlight(request)().(highlightResultMsg))
		want := new(highlight.Session).Lines(m.ctx, highlight.Language(m.document().Path), request.text, request.theme)
		if !reflect.DeepEqual(m.tabs[0].spans, want) {
			t.Fatal("incremental app result differs from fresh highlight")
		}
	}
	complete()
	session := m.tabs[0].highlighter
	m.document().Insert("/*\n")
	complete()
	m.document().Undo()
	complete()
	if m.tabs[0].highlighter != session {
		t.Fatal("editing replaced the incremental session")
	}
}

func TestHighlightingRejectsCancelledAndStaleText(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	first := m.scheduleHighlight()
	if first == nil || !m.tabs[0].highlightJob.pending {
		t.Fatal("highlight work was not deferred")
	}
	old := first().(highlightStartMsg).request
	m.document().Insert("// new\n")
	next := m.scheduleHighlight()
	if old.ctx.Err() == nil {
		t.Fatal("new text did not cancel old tokenization")
	}
	stale := [][]highlight.Span{{{Start: 0, End: 1, Color: "#FF0000"}}}
	m.finishHighlight(highlightResultMsg{old, stale})
	if m.tabs[0].highlightText == old.text {
		t.Fatal("stale highlight replaced current syntax")
	}
	request := next().(highlightStartMsg).request
	work := m.startHighlight(request)
	if work == nil {
		t.Fatal("latest highlight request was rejected")
	}
	m.tabs[0].top = 1
	result := work().(highlightResultMsg)
	m.finishHighlight(result)
	if m.tabs[0].highlightText != m.document().Text || len(m.tabs[0].spans) == 0 {
		t.Fatal("latest syntax did not reach the document")
	}
	if m.tabs[0].top != 1 {
		t.Fatal("syntax completion moved scroll position")
	}
}

func TestClosingTabCancelsHighlightJob(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	request := m.scheduleHighlight()().(highlightStartMsg).request
	m.removeTab(request.doc)
	if request.ctx.Err() == nil || m.startHighlight(request) != nil {
		t.Fatal("closed tab retained background tokenization")
	}
	m.finishHighlight(highlightResultMsg{request, nil})
	if len(m.tabs) != 0 {
		t.Fatal("background result reopened a closed tab")
	}
}

func TestInsertRetainsUnchangedHighlightUntilNormal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	source := "# Heading\n\n```go\nvar x = 1\n```\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	d := m.document()
	d.SetPosition(1, 0)
	m.ensureEditor()
	drain(t, m, m.scheduleHighlight())
	// This comparison covers token styling, independent of the moving line border.
	m.colors.CurrentLineBorder = ""
	editorText := func(line int) string {
		return ansi.Cut(m.editorLines()[line], 7, m.mainWidth())
	}
	heading, code := editorText(0), editorText(3)
	if len(m.tabs[0].spans[0]) == 0 || len(m.tabs[0].spans[3]) == 0 {
		t.Fatal("fixture did not start with highlighting")
	}

	drain(t, m, press(m, "i"))
	_, cmd := m.Update(tea.PasteMsg{Content: "typing\n"})
	drain(t, m, cmd)
	if tab := &m.tabs[0]; !tab.vim.InsertMode || tab.highlightJob.pending || tab.highlightText != source {
		t.Fatal("inserting text started another highlight pass")
	}
	if got := editorText(0); got != heading {
		t.Fatalf("highlight above edit changed: %q, want %q", got, heading)
	}
	if got := editorText(4); got != code {
		t.Fatalf("highlight below inserted line did not follow it: %q, want %q", got, code)
	}

	drain(t, m, press(m, "esc"))
	if tab := &m.tabs[0]; tab.vim.InsertMode || tab.highlightText != d.Text {
		t.Fatal("returning to Normal did not publish updated highlighting")
	}
}
