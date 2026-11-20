package highlight

import (
	"time"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/foursmith/vl/internal/theme"
)

// Preview highlights consecutive lines with only the current lexical state.
type Preview struct {
	grammar *textmate.Grammar
	state   *textmate.StateStack
	mapper  spanMapper
	budget  time.Duration

	// Only memoize the most recent visible line, and only when tokenizing
	// it kept the same lexical state. Repeated lines then need no regex work.
	lastLine  string
	lastState *textmate.StateStack
	lastSpans []Span
}

func NewPreview(language LanguageInfo, colors *theme.Theme) *Preview {
	scope := language.Scope
	if scope == "" {
		return nil
	}
	syntax := registry
	if scope == "text.html.markdown" {
		// Markdown references dozens of other grammars for fenced code. A
		// short-lived source preview only needs the Markdown grammar itself.
		syntax = textmate.NewRegistry(textmate.RegistryOptions{LoadGrammar: func(name string) (*textmate.RawGrammar, error) {
			if name != scope {
				return nil, nil
			}
			return grammars.Load(name)
		}})
	}
	grammar, err := syntax.LoadGrammar(scope)
	if err != nil || grammar == nil {
		return nil
	}
	// TextMate lazily compiles the root grammar and its scanner on first use.
	// Warm them on fixed input before starting the preview budget, then begin
	// the actual document with a fresh lexical state.
	grammar.TokenizeLine("", nil)
	return &Preview{grammar: grammar, mapper: spanMapper{colors: colors}, budget: 50 * time.Millisecond}
}

// Line advances lexical state. Context lines need no presentation spans.
// Callers must treat returned spans as read-only.
func (p *Preview) Line(line string, visible bool) []Span {
	if p.budget <= 0 {
		return nil
	}
	if p.lastState != nil && p.lastState == p.state && p.lastLine == line {
		if visible {
			return p.lastSpans
		}
		return nil
	}
	startState := p.state
	var result textmate.LineResult
	for attempt := 0; attempt < 3 && p.budget > 0; attempt++ {
		start := time.Now()
		result = p.grammar.TokenizeLineWithOptions(line, p.state, textmate.TokenizeOptions{
			TimeLimit: min(p.budget, 5*time.Millisecond), MaxLineBytes: 8 << 10,
		})
		p.budget -= time.Since(start)
		// Matching can load additional nested patterns. Retry with the same
		// starting state while there is still time in the preview budget.
		if result.StoppedReason != textmate.StopReasonTimeLimit {
			break
		}
	}
	if !result.Stopped {
		p.state = result.RuleStack
	}
	if visible {
		spans := p.mapper.spans(line, result.Tokens)
		if !result.Stopped && startState == p.state {
			p.lastLine, p.lastState, p.lastSpans = line, p.state, spans
		}
		return spans
	}
	return nil
}
