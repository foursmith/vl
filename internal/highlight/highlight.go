package highlight

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type Span struct {
	Start, End int
	Color      string
}

// Highlighting is optional presentation data; it never rewrites source text.
const MaxBytes = 256 << 10

func Lines(ctx context.Context, path, text string) [][]Span {
	if len(text) > MaxBytes || ctx.Err() != nil {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		return nil
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(&chroma.TokeniseOptions{State: "root", EnsureLF: false}, text)
	if err != nil {
		return nil
	}
	lines := [][]Span{{}}
	col := 0
	for token := iterator(); token != chroma.EOF; token = iterator() {
		if ctx.Err() != nil {
			return nil
		}
		color := color(token.Type)
		// VS Code's Go grammar scopes import as keyword.control.import.go;
		// Chroma groups it with the blue package/namespace declarations.
		if lexer.Config().Name == "Go" && token.Type == chroma.KeywordNamespace && token.Value == "import" {
			color = "#C586C0"
		}
		for i, part := range strings.Split(token.Value, "\n") {
			if i > 0 {
				lines = append(lines, nil)
				col = 0
			}
			length := utf8.RuneCountInString(part)
			if length > 0 && color != "" {
				n := len(lines) - 1
				lines[n] = append(lines[n], Span{col, col + length, color})
			}
			col += length
		}
	}
	return lines
}

func color(t chroma.TokenType) string {
	switch {
	case t.InCategory(chroma.Comment):
		return "#6A9955"
	case t == chroma.LiteralStringEscape:
		return "#D7BA7D"
	case t.InSubCategory(chroma.LiteralString):
		return "#CE9178"
	case t.InSubCategory(chroma.LiteralNumber):
		return "#B5CEA8"
	case t == chroma.KeywordType || t == chroma.NameClass:
		return "#4EC9B0"
	case t == chroma.KeywordNamespace || t == chroma.KeywordDeclaration || t == chroma.KeywordConstant || t == chroma.KeywordPseudo:
		return "#569CD6"
	case t.InCategory(chroma.Keyword):
		return "#C586C0"
	case t.InSubCategory(chroma.NameFunction) || t == chroma.NameBuiltin:
		return "#DCDCAA"
	case t == chroma.NameConstant:
		return "#4FC1FF"
	case t.InCategory(chroma.Operator):
		return "#D4D4D4"
	case t.InCategory(chroma.Name):
		return "#9CDCFE"
	}
	return ""
}
