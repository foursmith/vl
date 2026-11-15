package highlight

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"
)

type Span struct {
	Start, End int
	Color      string
	Font       FontStyle
}

// Highlighting is optional presentation data; it never rewrites source text.
const MaxBytes = 256 << 10

var registry = textmate.NewRegistry(textmate.RegistryOptions{LoadGrammar: grammars.Load})
var tokenizeOptions = textmate.TokenizeOptions{TimeLimit: 100 * time.Millisecond, MaxLineRunes: 20_000, MaxLineBytes: 80_000}

func Lines(ctx context.Context, path, text string, light bool) [][]Span {
	colors := darkModernTheme
	if light {
		colors = lightModernTheme
	}
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
	styles := make(map[*textmate.ScopeStack]tokenStyle)
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
				style = colors.resolve(token.Scopes)
				styles[token.ScopeStack] = style
			}
			start, end := min(token.Start, length), min(token.End, length)
			if start >= end || style.color == colors.foreground && style.font == 0 {
				continue
			}
			spans := lines[i]
			if n := len(spans); n > 0 && spans[n-1].End == start && spans[n-1].Color == style.color && spans[n-1].Font == style.font {
				spans[n-1].End = end
			} else {
				spans = append(spans, Span{Start: start, End: end, Color: style.color, Font: style.font})
			}
			lines[i] = spans
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return lines
}
