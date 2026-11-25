package highlight

import (
	"context"
	"strings"
	"sync"

	textmate "github.com/eugenioenko/textmate-go"

	"github.com/foursmith/vl/internal/theme"
)

const sessionCacheLimit = 20_000

// Session retains lexical state across immutable text snapshots. Its zero value
// is ready for use; concurrent requests are serialized, with cancellation checked
// after acquiring ownership. Returned spans remain immutable across later edits.
type Session struct {
	mu      sync.Mutex
	scope   string
	doc     *textmate.Document
	source  []string
	stopped int // one past the first line whose tokenization stopped
	mapper  spanMapper
	spans   map[*textmate.Token][]Span
}

func (s *Session) Lines(ctx context.Context, language LanguageInfo, text string, colors *theme.Theme) [][]Span {
	if len(text) > MaxBytes || ctx.Err() != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source := strings.Split(text, "\n")
	if !s.prepare(ctx, language.Scope, source, colors) {
		return nil
	}
	lines := make([][]Span, len(source))
	for i := range source {
		tokens, ok := s.line(ctx, i)
		if ctx.Err() != nil {
			return nil
		}
		if !ok {
			break
		}
		lines[i] = s.styles(i, tokens)
	}
	return lines
}

// Rows returns spans for the requested source rows in ascending order. It
// materializes incoming lexical states as needed without styling skipped rows.
// The caller holds source immutable and enforces the editor's size limit.
func (s *Session) Rows(ctx context.Context, language LanguageInfo, source []string, colors *theme.Theme, rows []int) map[int][]Span {
	return s.selectedRows(ctx, language, source, colors, rows, false)
}

// BracketRows reports the ranges in strings, comments and regexes that the
// editor must skip when matching brackets across the document.
func (s *Session) BracketRows(ctx context.Context, language LanguageInfo, source []string, colors *theme.Theme) map[int][]Span {
	var rows []int
	for i, line := range source {
		if ctx.Err() != nil {
			return nil
		}
		if strings.ContainsAny(line, "()[]{}") {
			rows = append(rows, i)
		}
	}
	return s.selectedRows(ctx, language, source, colors, rows, true)
}

func (s *Session) selectedRows(ctx context.Context, language LanguageInfo, source []string, colors *theme.Theme, rows []int, bracketsOnly bool) map[int][]Span {
	s.mu.Lock()
	defer s.mu.Unlock()
	spans := make(map[int][]Span, len(rows))
	if !bracketsOnly {
		for _, row := range rows {
			spans[row] = nil
		}
	}
	if len(rows) == 0 {
		return spans
	}
	if !s.prepare(ctx, language.Scope, source, colors) {
		return spans
	}
	for _, row := range rows {
		next := s.nextStateLine()
		for next < row {
			if _, ok := s.line(ctx, next); !ok {
				return spans
			}
			next = s.nextStateLine()
		}
		tokens, ok := s.line(ctx, row)
		if !ok {
			break
		}
		if bracketsOnly {
			for _, token := range tokens {
				if ignoresBrackets(token.Scopes) {
					spans[row] = append(spans[row], Span{Start: token.Start, End: token.End, IgnoreBrackets: true})
				}
			}
		} else {
			spans[row] = s.styles(row, tokens)
		}
	}
	return spans
}

func (s *Session) nextStateLine() int {
	next := max(0, s.doc.MaterializedLines()-1)
	if s.stopped != 0 {
		next = min(next, s.stopped-1)
	}
	return next
}

func (s *Session) prepare(ctx context.Context, scope string, source []string, colors *theme.Theme) bool {
	if scope == "" || ctx.Err() != nil {
		return false
	}
	if s.doc == nil || s.scope != scope {
		grammar, err := registry.LoadGrammar(scope)
		if err != nil || grammar == nil || ctx.Err() != nil {
			return false
		}
		s.scope = scope
		s.doc = textmate.NewDocument(grammar, textmate.DocumentOptions{TokenizeOptions: tokenizeOptions, CacheCapacity: sessionCacheLimit})
		s.source = nil
		s.stopped = 0
		s.mapper = spanMapper{}
		s.spans = nil
	}
	if s.mapper.colors != colors {
		s.mapper = spanMapper{colors: colors}
		s.spans = nil
	}
	if len(s.source) == len(source) && (len(source) == 0 || &s.source[0] == &source[0]) {
		return true
	}
	start, suffix := UnchangedEdges(s.source, source)
	oldEnd, newEnd := len(s.source)-suffix, len(source)-suffix
	if err := s.doc.ReplaceLines(start, oldEnd, source[start:newEnd]); err != nil {
		return false
	}
	if s.stopped > start {
		s.stopped = 0
	}
	s.source = source
	return true
}

func (s *Session) line(ctx context.Context, index int) ([]textmate.Token, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	result, ok := s.doc.Line(index)
	for retry := 0; result.StoppedReason == textmate.StopReasonTimeLimit && retry < 2 && ctx.Err() == nil; retry++ {
		result, ok = s.doc.Line(index)
	}
	// A stopped line cannot provide a reliable start state to later rows.
	if result.Stopped {
		s.stopped = index + 1
		return nil, false
	}
	if s.stopped == index+1 {
		s.stopped = 0
	}
	if !ok || ctx.Err() != nil {
		return nil, false
	}
	return result.Tokens, true
}

func (s *Session) styles(index int, tokens []textmate.Token) []Span {
	if len(tokens) == 0 {
		return nil
	}
	if s.spans == nil {
		s.spans = make(map[*textmate.Token][]Span)
	}
	key := &tokens[0]
	spans, cached := s.spans[key]
	if !cached {
		spans = s.mapper.spans(s.source[index], tokens)
		if len(s.spans) >= sessionCacheLimit {
			clear(s.spans)
		}
		s.spans[key] = spans
	}
	return spans
}

// UnchangedEdges counts matching lines at the beginning and end of two
// snapshots. The two ranges never overlap.
func UnchangedEdges(before, after []string) (prefix, suffix int) {
	limit := min(len(before), len(after))
	for prefix < limit && before[prefix] == after[prefix] {
		prefix++
	}
	for suffix < limit-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}
