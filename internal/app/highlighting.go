package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/highlight"
)

type highlightJob struct {
	text    string
	seq     uint64
	pending bool
	cancel  context.CancelFunc
}

func (j *highlightJob) stop() {
	if j.cancel != nil {
		j.cancel()
		j.cancel = nil
	}
	j.pending = false
}

type highlightRequest struct {
	doc        *editor.Document
	text, path string
	seq        uint64
	ctx        context.Context
}
type highlightStartMsg struct{ request highlightRequest }
type highlightResultMsg struct {
	request highlightRequest
	spans   [][]highlight.Span
}

func (m *Model) scheduleHighlight() tea.Cmd {
	if !m.showEditor || m.document() == nil {
		return nil
	}
	t := &m.tabs[m.activeTab]
	text := t.doc.Text
	if t.highlightText == text {
		t.highlightJob.stop()
		return nil
	}
	if len(text) > highlight.MaxBytes || text == "" {
		t.highlightJob.stop()
		t.highlightText, t.spans = text, nil
		return nil
	}
	if t.highlightJob.pending && t.highlightJob.text == text {
		return nil
	}
	t.highlightJob.stop()
	ctx, cancel := context.WithCancel(m.ctx)
	t.highlightJob.seq++
	t.highlightJob.text, t.highlightJob.pending, t.highlightJob.cancel = text, true, cancel
	request := highlightRequest{t.doc, text, t.doc.Path, t.highlightJob.seq, ctx}
	// Quiet typing bursts before spending CPU on a whole-file tokenizer.
	delay := 30 * time.Millisecond
	if len(text) > 32<<10 {
		delay = 200 * time.Millisecond
	}
	if !t.doc.Dirty() {
		delay = 0
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return highlightStartMsg{request} })
}

func (m *Model) highlightTarget(request highlightRequest) *tab {
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc == request.doc && t.doc.Text == request.text && t.highlightJob.seq == request.seq && t.highlightJob.pending && request.ctx.Err() == nil {
			return t
		}
	}
	return nil
}

func (m *Model) startHighlight(request highlightRequest) tea.Cmd {
	if m.highlightTarget(request) == nil {
		return nil
	}
	return func() tea.Msg {
		return highlightResultMsg{request, highlight.Lines(request.ctx, request.path, request.text)}
	}
}

func (m *Model) finishHighlight(msg highlightResultMsg) {
	if t := m.highlightTarget(msg.request); t != nil {
		t.highlightJob.stop()
		t.highlightText, t.spans = msg.request.text, msg.spans
		t.renderedRows = nil
	}
}
