package highlight

import (
	"time"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"

	"github.com/ourongxing/vl/internal/theme"
)

// Preview highlights consecutive lines with only the current lexical state.
type Preview struct {
	grammar *textmate.Grammar
	state   *textmate.StateStack
	mapper  spanMapper
	budget  time.Duration
}

func NewPreview(path string, colors *theme.Theme) *Preview {
	scope := grammars.ScopeForFilename(path)
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
func (p *Preview) Line(line string, visible bool) []Span {
	if p.budget <= 0 {
		return nil
	}
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
		return p.mapper.spans(line, result.Tokens)
	}
	return nil
}
