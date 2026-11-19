package app

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	tableast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.Table)).Parser()

// Padding is indexed by byte offset in the source line. All other editor
// features continue to use the source text and its rune offsets.
type tablePadding map[int]int

type markdownLayout struct {
	source string
	rows   map[int]tablePadding
	images []markdownImageRef
}

func (t *tab) tableRows() map[int]tablePadding {
	t.syncMarkdown()
	return t.markdown.rows
}

func (t *tab) syncMarkdown() {
	switch strings.ToLower(filepath.Ext(t.doc.Path)) {
	case ".md", ".markdown", ".mdown", ".mkdn", ".mkd":
	default:
		return
	}
	cache := &t.markdown
	if cache.source == t.doc.Text {
		return
	}
	*cache = markdownLayout{source: t.doc.Text}
	if !strings.Contains(cache.source, "|") && !strings.Contains(cache.source, "![") {
		return
	}

	source := t.doc.Lines()
	starts := make([]int, len(source))
	for i := 1; i < len(source); i++ {
		starts[i] = starts[i-1] + len(source[i-1]) + 1
	}
	doc := markdownParser.Parse(text.NewReader([]byte(cache.source)))
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if image, ok := node.(*ast.Image); ok {
			pos := image.Pos()
			line := sort.Search(len(starts), func(i int) bool { return starts[i] > pos }) - 1
			if path, remote := markdownImagePath(t.doc.Path, image.Destination); line >= 0 && path != "" {
				cache.images = append(cache.images, markdownImageRef{line, path, remote})
			}
			return ast.WalkSkipChildren, nil
		}
		table, ok := node.(*tableast.Table)
		if !ok {
			return ast.WalkContinue, nil
		}
		var rows []markdownTableRow
		for child := table.FirstChild(); child != nil; child = child.NextSibling() {
			pos := child.Pos()
			line := sort.Search(len(starts), func(i int) bool { return starts[i] > pos }) - 1
			rows = append(rows, splitMarkdownTableRow(line, source[line], pos-starts[line]))
			if child == table.FirstChild() {
				// Goldmark records the header and body positions. The delimiter
				// immediately follows the header, after any container prefix.
				delimiter := source[line+1]
				start := len(delimiter) - len(strings.TrimLeft(delimiter, " \t>"))
				rows = append(rows, splitMarkdownTableRow(line+1, delimiter, start))
			}
		}
		alignMarkdownTable(rows, len(table.Alignments))
		for _, row := range rows {
			if len(row.padding) != 0 {
				if cache.rows == nil {
					cache.rows = make(map[int]tablePadding)
				}
				cache.rows[row.line] = row.padding
			}
		}
		return ast.WalkContinue, nil
	})
}

type markdownTableRow struct {
	line, start, stop int
	source            string
	pipes             []int
	padding           tablePadding
	visual            int
}

func splitMarkdownTableRow(line int, source string, start int) markdownTableRow {
	start += len(source[start:]) - len(strings.TrimLeft(source[start:], " \t"))
	stop := len(strings.TrimRight(source, " \t"))
	if start < stop && source[start] == '|' {
		start++
	}
	row := markdownTableRow{line: line, source: source, start: start, stop: stop}
	row.visual = displayWidth(source[:start], 0, nil)
	for i := start; i < stop; i++ {
		if source[i] == '|' && (i == 0 || source[i-1] != '\\') {
			row.pipes = append(row.pipes, i)
		}
	}
	return row
}

func (r *markdownTableRow) pad(offset, width int) {
	if width > 0 {
		if r.padding == nil {
			r.padding = make(tablePadding)
		}
		r.padding[offset] += width
	}
}

func alignMarkdownTable(rows []markdownTableRow, columns int) {
	first := 0
	for _, row := range rows {
		first = max(first, row.visual)
	}
	for i := range rows {
		row := &rows[i]
		row.pad(row.start, first-row.visual)
		row.visual = first
	}

	for col := 0; col < columns; col++ {
		right := 0
		for i := range rows {
			row := &rows[i]
			if col > len(row.pipes) {
				continue
			}
			end := row.stop
			hasPipe := col < len(row.pipes)
			if hasPipe {
				end = row.pipes[col]
			}
			if row.start < end {
				cell := row.source[row.start:end]
				if row.start > 0 && row.source[row.start-1] == '|' && cell[0] != ' ' && cell[0] != '\t' {
					row.pad(row.start, 1)
					row.visual++
				}
				row.visual += displayWidth(cell, row.visual, nil)
			}
			// Leave a space inside each separator for compact source tables.
			minimum := row.visual
			if hasPipe && (row.start == end || row.source[end-1] != ' ' && row.source[end-1] != '\t') {
				minimum++
			}
			right = max(right, minimum)
		}
		for i := range rows {
			row := &rows[i]
			if col < len(row.pipes) {
				pipe := row.pipes[col]
				row.pad(pipe, right-row.visual)
				row.start = pipe + 1
				row.visual = right + 1
			}
		}
	}
}

// The offset belongs to the source glyph immediately following virtual space.
func displayWidth(source string, start int, padding tablePadding) int {
	visual := start
	iter := graphemes.FromString(source)
	for iter.Next() {
		visual += padding[iter.Start()]
		visual += ansi.StringWidth(displayCluster(iter.Value(), visual))
	}
	return visual - start
}

func (t *tab) editorColumn(line, col int) int {
	padding := t.tableRows()[line]
	if len(padding) == 0 && t.doc.PlainASCII(line) {
		return col
	}
	start := t.doc.ByteOffset(t.doc.LineOffset(line))
	end := t.doc.ByteOffset(t.doc.Cursor)
	return displayWidth(t.doc.Text[start:end], 0, padding) + padding[end-start]
}
