package highlight

import (
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
func Lines(path, text string) [][]Span {
	if len(text) > 256<<10 {
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
		color := color(token.Type)
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
	case t.InSubCategory(chroma.LiteralString):
		return "#CE9178"
	case t.InSubCategory(chroma.LiteralNumber):
		return "#B5CEA8"
	case t == chroma.KeywordType || t == chroma.NameClass || t == chroma.NameBuiltin:
		return "#4EC9B0"
	case t == chroma.KeywordNamespace || t == chroma.KeywordDeclaration:
		return "#C586C0"
	case t.InCategory(chroma.Keyword):
		return "#569CD6"
	case t.InSubCategory(chroma.NameFunction):
		return "#DCDCAA"
	case t.InCategory(chroma.Name):
		return "#9CDCFE"
	}
	return ""
}
