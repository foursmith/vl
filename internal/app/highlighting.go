package app

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/theme"
)

const maxEditorHighlightBytes = 2048 << 10

type highlightJob struct {
	text   string
	rows   []int
	ctx    context.Context
	cancel context.CancelFunc
}

func (j *highlightJob) stop() {
	if j.cancel != nil {
		j.cancel()
	}
	*j = highlightJob{}
}

type highlightRequest struct {
	session  *highlight.Session
	theme    *theme.Theme
	doc      *editor.Document
	text     string
	source   []string
	rows     []int
	language highlight.LanguageInfo
	ctx      context.Context
}
type highlightStartMsg struct{ request highlightRequest }
type highlightResultMsg struct {
	request highlightRequest
	spans   map[int][]highlight.Span
}
type bracketHighlightStartMsg struct{ request highlightRequest }
type bracketHighlightResultMsg struct {
	request highlightRequest
	spans   map[int][]highlight.Span
}

func (m *Model) scheduleHighlight() tea.Cmd {
	if !m.editing() {
		return nil
	}
	t := &m.tabs[m.activeTab]
	text := t.doc.Text
	language := t.documentLanguage()
	if len(text) > maxEditorHighlightBytes || text == "" || language.Scope == "" {
		if t.highlightText == text {
			return nil
		}
		t.clearHighlight()
		t.highlightText, t.highlighter = text, nil
		return nil
	}
	source := t.doc.Lines()
	top, height := t.editorDisplayTop(), m.contentHeight()
	sticky := m.editorStickyRows()
	currentJob := t.highlightJob.ctx != nil && t.highlightJob.text == text
	covered, pending := t.highlightText == text, currentJob
	for row := range t.highlightRowsAt(top, height, sticky, len(source), 0) {
		_, cached := t.spans[row]
		_, queued := slices.BinarySearch(t.highlightJob.rows, row)
		covered = covered && cached
		pending = pending && queued
		if !covered && !pending {
			break
		}
	}
	if covered && (currentJob || t.bracketSpans != nil || m.colors.BracketColor(0) == "") {
		if t.highlightJob.text != text {
			t.highlightJob.stop()
		}
		return nil
	}
	if pending {
		return nil
	}
	t.highlightJob.stop()
	ctx, cancel := context.WithCancel(m.ctx)
	rows := make([]int, 0, min(len(source), height*2))
	for row := range t.highlightRowsAt(top, height, sticky, len(source), height/2) {
		rows = append(rows, row)
	}
	slices.Sort(rows)
	rows = slices.Compact(rows)
	t.highlightJob = highlightJob{text: text, rows: rows, ctx: ctx, cancel: cancel}
	if t.highlighter == nil {
		t.highlighter = &highlight.Session{}
	}
	request := highlightRequest{
		session: t.highlighter, theme: m.colorTheme, doc: t.doc, text: text, source: source,
		rows: rows, language: language, ctx: ctx,
	}
	return func() tea.Msg { return highlightStartMsg{request} }
}

func (t *tab) highlightRowsAt(top, height int, sticky []int, lines, margin int) func(func(int) bool) {
	return func(yield func(int) bool) {
		for _, row := range sticky {
			if !yield(row) {
				return
			}
		}
		for y := max(0, top-margin); y < top+height+margin; y++ {
			if y >= top && y < top+len(sticky) {
				continue
			}
			line, image, _ := t.editorDisplayAt(y)
			if line >= lines || image == nil && !yield(line) {
				return
			}
		}
	}
}

func (m *Model) highlightTarget(request highlightRequest) *tab {
	if request.ctx.Err() != nil {
		return nil
	}
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.highlightJob.ctx == request.ctx && t.doc == request.doc && t.doc.Text == request.text {
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
		return highlightResultMsg{request, request.session.Rows(request.ctx, request.language, request.source, request.theme, request.rows)}
	}
}

func (m *Model) finishHighlight(msg highlightResultMsg) tea.Cmd {
	t := m.highlightTarget(msg.request)
	if t == nil {
		return nil
	}
	if t.highlightText != msg.request.text {
		t.bracketSpans = nil
	}
	t.highlightText, t.spans = msg.request.text, msg.spans
	t.highlightSource = msg.request.source
	t.renderedRows = nil
	if t.bracketSpans != nil || msg.request.theme.Palette().BracketColor(0) == "" {
		t.highlightJob.stop()
		return nil
	}
	// Bracket pairs can cross the viewport. Let visible text paint first.
	if len(msg.request.text) > 32<<10 && t.doc.Dirty() {
		return delayedMessage(msg.request.ctx, 75*time.Millisecond, func(time.Time) tea.Msg {
			return bracketHighlightStartMsg{msg.request}
		})
	}
	return func() tea.Msg { return bracketHighlightStartMsg{msg.request} }
}

func (m *Model) startBracketHighlight(request highlightRequest) tea.Cmd {
	if m.highlightTarget(request) == nil {
		return nil
	}
	return func() tea.Msg {
		spans := request.session.BracketRows(request.ctx, request.language, request.source, request.theme)
		return bracketHighlightResultMsg{request, spans}
	}
}

func (m *Model) finishBracketHighlight(msg bracketHighlightResultMsg) {
	if t := m.highlightTarget(msg.request); t != nil {
		t.highlightJob.stop()
		t.bracketSpans = msg.spans
		t.guides = nil
		t.renderedRows = nil
	}
}

func (t *tab) clearHighlight() {
	t.highlightJob.stop()
	t.highlightText, t.highlightSource, t.spans, t.bracketSpans, t.renderedRows = "", nil, nil, nil, nil
	t.guides = nil
	t.lastBrackets = bracketSnapshot{}
}

type editorHighlightRows struct {
	spans                      map[int][]highlight.Span
	prefix, suffixStart, shift int
	firstChanged, lastChanged  []highlight.Span
}

func (t *tab) highlightRows(source []string) editorHighlightRows {
	if t.highlightText == t.doc.Text {
		return editorHighlightRows{spans: t.spans, prefix: len(source), suffixStart: len(source)}
	}
	prefix, suffix := highlight.UnchangedEdges(t.highlightSource, source)
	rows := editorHighlightRows{
		spans: t.spans, prefix: prefix, suffixStart: len(source) - suffix,
		shift: len(t.highlightSource) - len(source),
	}
	oldLast, newLast := len(t.highlightSource)-suffix-1, rows.suffixStart-1
	if prefix > oldLast || prefix > newLast || len(t.spans[prefix])+len(t.spans[oldLast]) == 0 {
		return rows
	}
	// Keep styling on the surviving text within the edited rows while the
	// incremental tokenizer catches up, including across a split or join.
	oldFirst, newFirst := t.highlightSource[prefix], source[prefix]
	start, oldEnd, newEnd := editor.ChangedByteRange(oldFirst, newFirst)
	head := utf8.RuneCountInString(newFirst[:start])
	rows.firstChanged = appendHighlightRange(nil, t.spans[prefix], 0, head, 0)
	if oldLast == prefix && newLast == prefix {
		oldAt := head + utf8.RuneCountInString(oldFirst[start:oldEnd])
		newAt := head + utf8.RuneCountInString(newFirst[start:newEnd])
		for _, span := range t.spans[prefix] {
			if span.Start < head && span.End > oldAt && newAt > head {
				// An edit inside one token can use its style for the new text.
				span.Start, span.End = head, newAt
				rows.firstChanged = append(rows.firstChanged, span)
				break
			}
		}
		rows.firstChanged = appendHighlightRange(rows.firstChanged, t.spans[prefix], oldAt, utf8.RuneCountInString(oldFirst), newAt-oldAt)
		return rows
	}

	before, after := t.highlightSource[oldLast], source[newLast]
	oldFloor, newFloor := 0, 0
	if oldLast == prefix {
		oldFloor = start
	}
	if newLast == prefix {
		newFloor = start
	}
	tailBytes := editor.CommonSuffixBytes(before[oldFloor:], after[newFloor:])
	oldAt := utf8.RuneCountInString(before[:len(before)-tailBytes])
	newAt := utf8.RuneCountInString(after[:len(after)-tailBytes])
	if newLast == prefix {
		rows.firstChanged = appendHighlightRange(rows.firstChanged, t.spans[oldLast], oldAt, utf8.RuneCountInString(before), newAt-oldAt)
	} else {
		rows.lastChanged = appendHighlightRange(nil, t.spans[oldLast], oldAt, utf8.RuneCountInString(before), newAt-oldAt)
	}
	return rows
}

func appendHighlightRange(out, spans []highlight.Span, from, to, shift int) []highlight.Span {
	for _, span := range spans {
		start, end := max(span.Start, from), min(span.End, to)
		if start < end {
			span.Start, span.End = start+shift, end+shift
			out = append(out, span)
		}
	}
	return out
}

func (h editorHighlightRows) at(line int) []highlight.Span {
	switch {
	case line < h.prefix:
		return h.spans[line]
	case line >= h.suffixStart:
		return h.spans[line+h.shift]
	case line == h.prefix:
		return h.firstChanged
	case line == h.suffixStart-1:
		return h.lastChanged
	default:
		return nil
	}
}
