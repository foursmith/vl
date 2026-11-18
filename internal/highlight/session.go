package highlight

import (
	"context"
	"strings"
	"sync"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/ourongxing/vl/internal/theme"
)

const sessionCacheLimit = 20_000

// Session retains lexical state across immutable text snapshots. Its zero value
// is ready for use; concurrent requests are serialized, with cancellation checked
// after acquiring ownership. Returned spans remain immutable across later edits.
type Session struct {
	mu     sync.Mutex
	scope  string
	doc    *textmate.Document
	source []string
	mapper spanMapper
	spans  map[*textmate.Token][]Span
}

func (s *Session) Lines(ctx context.Context, path, text string, colors *theme.Theme) [][]Span {
	if len(text) > MaxBytes || ctx.Err() != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return nil
	}
	scope := grammars.ScopeForFilename(path)
	if scope == "" {
		return nil
	}
	if s.doc == nil || s.scope != scope {
		grammar, err := registry.LoadGrammar(scope)
		if err != nil || grammar == nil || ctx.Err() != nil {
			return nil
		}
		s.scope = scope
		s.doc = textmate.NewDocument(grammar, textmate.DocumentOptions{TokenizeOptions: tokenizeOptions, CacheCapacity: sessionCacheLimit})
		s.source = nil
		s.mapper = spanMapper{}
		s.spans = nil
	}
	if s.mapper.colors != colors {
		s.mapper = spanMapper{colors: colors}
		s.spans = nil
	}
	source := strings.Split(text, "\n")
	start, suffix := UnchangedEdges(s.source, source)
	oldEnd, newEnd := len(s.source)-suffix, len(source)-suffix
	if err := s.doc.ReplaceLines(start, oldEnd, source[start:newEnd]); err != nil {
		return nil
	}
	s.source = source
	lines := make([][]Span, len(source))
	if s.spans == nil {
		s.spans = make(map[*textmate.Token][]Span)
	}
	for i, line := range source {
		if ctx.Err() != nil {
			return nil
		}
		result, _ := s.doc.Line(i)
		for retry := 0; result.StoppedReason == textmate.StopReasonTimeLimit && retry < 2 && ctx.Err() == nil; retry++ {
			result, _ = s.doc.Line(i)
		}
		if ctx.Err() != nil {
			return nil
		}
		// Never propagate a partial lexical state to the following line.
		if result.Stopped {
			return lines
		}
		if len(result.Tokens) == 0 {
			continue
		}
		key := &result.Tokens[0]
		spans, ok := s.spans[key]
		if !ok {
			spans = s.mapper.spans(line, result.Tokens)
			if len(s.spans) >= sessionCacheLimit {
				clear(s.spans)
			}
			s.spans[key] = spans
		}
		lines[i] = spans
	}
	return lines
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
