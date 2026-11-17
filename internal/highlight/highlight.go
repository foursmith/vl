package highlight

import (
	"time"
	"unicode/utf8"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/ourongxing/vl/internal/theme"
)

type Span struct {
	Start, End int
	Color      string
	Background string
	Font       theme.FontStyle
}

// Highlighting is optional presentation data; it never rewrites source text.
const MaxBytes = 256 << 10

var registry = textmate.NewRegistry(textmate.RegistryOptions{LoadGrammar: grammars.Load})
var tokenizeOptions = textmate.TokenizeOptions{TimeLimit: 100 * time.Millisecond, MaxLineRunes: 20_000, MaxLineBytes: 80_000}

// A mapper belongs to one immutable theme and one serialized highlight session.
type spanMapper struct {
	colors *theme.Theme
	styles map[*textmate.ScopeStack]theme.TokenStyle
}

func (m *spanMapper) spans(line string, tokens []textmate.Token) []Span {
	if m.styles == nil {
		m.styles = make(map[*textmate.ScopeStack]theme.TokenStyle)
	}
	length := utf8.RuneCountInString(line)
	var spans []Span
	for _, token := range tokens {
		style, ok := m.styles[token.ScopeStack]
		if !ok {
			style = m.colors.Resolve(token.Scopes)
			if len(m.styles) >= sessionCacheLimit {
				clear(m.styles)
			}
			m.styles[token.ScopeStack] = style
		}
		start, end := min(token.Start, length), min(token.End, length)
		if start >= end || style.Color == m.colors.Palette().Foreground && style.Font == 0 && style.Background == "" {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].End == start && spans[n-1].Color == style.Color && spans[n-1].Font == style.Font && spans[n-1].Background == style.Background {
			spans[n-1].End = end
		} else {
			spans = append(spans, Span{Start: start, End: end, Color: style.Color, Font: style.Font, Background: style.Background})
		}
	}
	return spans
}
