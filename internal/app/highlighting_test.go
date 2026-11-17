package app

import (
	"testing"

	"github.com/ourongxing/vl/internal/highlight"
)

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
