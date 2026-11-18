package highlight

import (
	"context"
	"strings"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"
	"github.com/foursmith/vl/internal/theme"
)

// Keep a direct, uncached tokenizer as an independent edit oracle and baseline.
func referenceLines(ctx context.Context, path, text string, colors *theme.Theme) [][]Span {
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
	mapper := spanMapper{colors: colors}
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
		lines[i] = mapper.spans(line, result.Tokens)
	}
	if ctx.Err() != nil {
		return nil
	}
	return lines
}
