package highlight

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/ourongxing/elegit/internal/theme"
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

func Lines(ctx context.Context, path, text string, colors *theme.Theme) [][]Span {
	if len(text) > MaxBytes || ctx.Err() != nil {
		return nil
	}
	scope := grammars.ScopeForFilename(path)
	if scope == "" {
		return nil
	}
	grammar, err := registry.LoadGrammar(scope)
	if err != nil || grammar == nil || ctx.Err() != nil {
		return nil
	}
	source := strings.Split(text, "\n")
	lines := make([][]Span, len(source))
	var state *textmate.StateStack
	styles := make(map[*textmate.ScopeStack]theme.TokenStyle)
	for i, line := range source {
		if ctx.Err() != nil {
			return nil
		}
		result := grammar.TokenizeLineWithOptions(line, state, tokenizeOptions)
		// Cold lazy compilation can consume a line's soft budget. Retry from
		// the same immutable start state, as Document does for transient stops.
		for retry := 0; result.StoppedReason == textmate.StopReasonTimeLimit && retry < 2 && ctx.Err() == nil; retry++ {
			result = grammar.TokenizeLineWithOptions(line, state, tokenizeOptions)
		}
		if ctx.Err() != nil {
			return nil
		}
		// An incomplete line cannot supply trustworthy lexical state to later lines.
		// Retain the completed prefix and leave the remainder plain.
		if result.Stopped {
			return lines
		}
		state = result.RuleStack
		length := utf8.RuneCountInString(line)
		for _, token := range result.Tokens {
			style, ok := styles[token.ScopeStack]
			if !ok {
				style = colors.Resolve(token.Scopes)
				styles[token.ScopeStack] = style
			}
			start, end := min(token.Start, length), min(token.End, length)
			if start >= end || style.Color == colors.Palette().Foreground && style.Font == 0 && style.Background == "" {
				continue
			}
			spans := lines[i]
			if n := len(spans); n > 0 && spans[n-1].End == start && spans[n-1].Color == style.Color && spans[n-1].Font == style.Font && spans[n-1].Background == style.Background {
				spans[n-1].End = end
			} else {
				spans = append(spans, Span{Start: start, End: end, Color: style.Color, Font: style.Font, Background: style.Background})
			}
			lines[i] = spans
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return lines
}
