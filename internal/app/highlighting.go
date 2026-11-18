package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/editor"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
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
	session    *highlight.Session
	theme      *theme.Theme
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
	if !m.editing() {
		return nil
	}
	t := &m.tabs[m.activeTab]
	text := t.doc.Text
	if t.vim.InsertMode || t.highlightText == text {
		t.highlightJob.stop()
		return nil
	}
	if len(text) > highlight.MaxBytes || text == "" {
		t.clearHighlight()
		t.highlightText, t.highlighter = text, nil
		return nil
	}
	if t.highlightJob.pending && t.highlightJob.text == text {
		return nil
	}
	t.highlightJob.stop()
	ctx, cancel := context.WithCancel(m.ctx)
	t.highlightJob.seq++
	t.highlightJob.text, t.highlightJob.pending, t.highlightJob.cancel = text, true, cancel
	if t.highlighter == nil {
		t.highlighter = &highlight.Session{}
	}
	request := highlightRequest{session: t.highlighter, theme: m.colorTheme, doc: t.doc, text: text, path: t.doc.Path, seq: t.highlightJob.seq, ctx: ctx}
	// Coalesce typing bursts before updating the incremental tokenizer.
	delay := 30 * time.Millisecond
	if len(text) > 32<<10 {
		delay = 200 * time.Millisecond
	}
	if !t.doc.Dirty() {
		delay = 0
	}
	return delayedMessage(ctx, delay, func(time.Time) tea.Msg { return highlightStartMsg{request} })
}

func (m *Model) highlightTarget(request highlightRequest) *tab {
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc != nil && t.doc == request.doc && t.doc.Text == request.text && t.highlightJob.seq == request.seq && t.highlightJob.pending && request.ctx.Err() == nil {
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
		return highlightResultMsg{request, request.session.Lines(request.ctx, request.path, request.text, request.theme)}
	}
}

func (m *Model) finishHighlight(msg highlightResultMsg) {
	if t := m.highlightTarget(msg.request); t != nil {
		t.highlightJob.stop()
		t.highlightText, t.spans = msg.request.text, msg.spans
		t.highlightSource = t.doc.Lines()
		t.renderedRows = nil
	}
}

func (t *tab) clearHighlight() {
	t.highlightJob.stop()
	t.highlightText, t.highlightSource, t.spans, t.renderedRows = "", nil, nil, nil
}

type editorHighlightRows struct {
	spans                      [][]highlight.Span
	prefix, suffixStart, shift int
}

func (t *tab) highlightRows(source []string) editorHighlightRows {
	if t.highlightText == t.doc.Text {
		return editorHighlightRows{spans: t.spans, prefix: len(source), suffixStart: len(source)}
	}
	prefix, suffix := highlight.UnchangedEdges(t.highlightSource, source)
	return editorHighlightRows{
		spans: t.spans, prefix: prefix, suffixStart: len(source) - suffix,
		shift: len(t.highlightSource) - len(source),
	}
}

func (h editorHighlightRows) at(line int) []highlight.Span {
	if line >= h.prefix {
		if line < h.suffixStart {
			return nil
		}
		line += h.shift
	}
	if line >= 0 && line < len(h.spans) {
		return h.spans[line]
	}
	return nil
}
