package app

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/theme"
)

type indentGuide struct {
	column int
	parent *indentGuide
}

type bracketPosition struct {
	line, offset int
}

type bracketPair struct {
	open, close bracketPosition
	depth       int
}

type bracketGuide struct {
	start, end, column int
	color              string
}

type editorGuides struct {
	path, text string
	indents    []int
	rows       []*indentGuide
	pairs      []bracketPair
	depths     map[int]int
	edit       bracketEdit
}

type bracketSnapshot struct {
	text   string
	pairs  []bracketPair
	depths map[int]int
}

// Translate surviving bracket positions while new syntax is pending.
type bracketEdit struct {
	start, oldEnd, newEnd, lineShift int
}

func (t *tab) editorGuides(source []string) *editorGuides {
	if t.guides != nil && t.guides.path == t.doc.Path && t.guides.text == t.doc.Text {
		return t.guides
	}
	g := &editorGuides{path: t.doc.Path, text: t.doc.Text}
	t.guides = g
	if len(t.doc.Text) > maxEditorHighlightBytes || t.documentLanguage().ID == "text" {
		return g
	}
	g.indents = make([]int, len(source))
	g.rows = make([]*indentGuide, len(source))
	hasSyntax := t.highlightText == t.doc.Text && t.bracketSpans != nil
	if hasSyntax {
		g.depths = make(map[int]int)
	}
	root := &indentGuide{}
	indent := root
	previous := -1
	type opening struct {
		bracketPosition
		close rune
	}
	var stack []opening
	for line, text := range source {
		column := 0
		prefix := strings.TrimLeft(text, " \t")
		for _, space := range text[:len(text)-len(prefix)] {
			column += len(displayCluster(string(space), column))
		}
		if prefix != "" {
			for indent.column > column {
				indent = indent.parent
			}
			if indent.column < column {
				indent = &indentGuide{column, indent}
			}
			g.indents[line], g.rows[line] = column, indent.parent
			if previous >= 0 {
				shallower := line
				if g.indents[previous] < column {
					shallower = previous
				}
				for blank := previous + 1; blank < line; blank++ {
					g.indents[blank], g.rows[blank] = g.indents[shallower], g.rows[shallower]
				}
			}
			previous = line
		}

		if !hasSyntax || !strings.ContainsAny(prefix, "()[]{}") {
			continue
		}
		spans := t.bracketSpans[line]
		span, runeColumn := 0, -1
		for _, r := range text {
			runeColumn++
			if !strings.ContainsRune("()[]{}", r) {
				continue
			}
			for span < len(spans) && spans[span].End <= runeColumn {
				span++
			}
			if span < len(spans) && spans[span].Start <= runeColumn && spans[span].IgnoreBrackets {
				continue
			}
			pos := bracketPosition{line, t.doc.LineOffset(line) + runeColumn}
			switch r {
			case '(':
				stack = append(stack, opening{pos, ')'})
			case '[':
				stack = append(stack, opening{pos, ']'})
			case '{':
				stack = append(stack, opening{pos, '}'})
			default:
				if n := len(stack); n > 0 && stack[n-1].close == r {
					open, depth := stack[n-1].bracketPosition, n-1
					g.pairs = append(g.pairs, bracketPair{open, pos, depth})
					g.depths[open.offset], g.depths[pos.offset] = depth, depth
					stack = stack[:n-1]
				}
			}
		}
	}
	if hasSyntax {
		t.lastBrackets = bracketSnapshot{g.text, g.pairs, g.depths}
	} else if last := t.lastBrackets; len(last.pairs) != 0 {
		g.pairs, g.depths = last.pairs, last.depths
		start, oldEnd, newEnd := editor.ChangedByteRange(last.text, g.text)
		prefix := utf8.RuneCountInString(g.text[:start])
		before, after := last.text[start:oldEnd], g.text[start:newEnd]
		g.edit = bracketEdit{
			start: prefix, oldEnd: prefix + utf8.RuneCountInString(before),
			newEnd:    prefix + utf8.RuneCountInString(after),
			lineShift: strings.Count(after, "\n") - strings.Count(before, "\n"),
		}
	}
	return g
}

func (g *editorGuides) bracketDepth(offset int) (int, bool) {
	if offset >= g.edit.start && offset < g.edit.newEnd {
		return 0, false
	}
	if offset >= g.edit.newEnd {
		offset += g.edit.oldEnd - g.edit.newEnd
	}
	depth, ok := g.depths[offset]
	return depth, ok
}

func (g *editorGuides) activeBracket(cursor int, colors theme.Palette) bracketGuide {
	oldCursor := cursor
	if cursor >= g.edit.start {
		oldCursor = g.edit.oldEnd + max(0, cursor-g.edit.newEnd)
	}
	first := sort.Search(len(g.pairs), func(i int) bool { return g.pairs[i].close.offset >= oldCursor })
	for _, pair := range g.pairs[first:] {
		open, openOK := g.edit.position(pair.open)
		close, closeOK := g.edit.position(pair.close)
		if openOK && closeOK && open.offset <= cursor && close.offset >= cursor {
			return bracketGuide{
				start:  open.line,
				end:    close.line,
				column: min(g.indents[open.line], g.indents[close.line]),
				color:  colors.BracketGuide(pair.depth),
			}
		}
	}
	return bracketGuide{}
}

func (e bracketEdit) position(pos bracketPosition) (bracketPosition, bool) {
	if pos.offset < e.start {
		return pos, true
	}
	if pos.offset < e.oldEnd {
		return bracketPosition{}, false
	}
	return bracketPosition{pos.line + e.lineShift, pos.offset + e.newEnd - e.oldEnd}, true
}

func (g *editorGuides) decorate(rendered string, line, gutter, left int, bracket bracketGuide, colors theme.Palette) string {
	if line >= len(g.rows) {
		return rendered
	}
	active := bracket.color != "" && bracket.column >= left && line > bracket.start && line < bracket.end
	indent := g.rows[line]
	if indent == nil && !active {
		return rendered
	}
	var columns []int
	for guide := indent; guide != nil && guide.column >= left; guide = guide.parent {
		columns = append(columns, guide.column)
	}
	slices.Reverse(columns)
	x := -gutter + left
	var out strings.Builder
	var pen uv.Style
	for _, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
		content, style := cell.Content, cell.Style
		if x >= left {
			for len(columns) > 0 && columns[0] < x {
				columns = columns[1:]
			}
			var color string
			switch {
			case active && x == bracket.column:
				color = bracket.color
			case len(columns) > 0 && x == columns[0]:
				color = colors.IndentGuide
			}
			if content == " " && color != "" {
				bg := colors.Background
				if style.Bg != nil {
					r, g, b, _ := style.Bg.RGBA()
					bg = fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
				}
				content = "│"
				style.Fg = lipgloss.Color(theme.Composite(color, bg))
			}
		}
		out.WriteString(style.Diff(&pen))
		out.WriteString(content)
		pen = style
		x += cell.Width
	}
	out.WriteString("\x1b[0m")
	return out.String()
}
