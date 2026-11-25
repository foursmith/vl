package app

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/foursmith/vl/internal/editor"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var (
	markdownQuotePrefix = regexp.MustCompile(`^(?:[ \t]*>[ \t]?)+`)
	markdownListPrefix  = regexp.MustCompile(`^([ \t]*)([-+*]|([0-9]{1,9})[.)])([ \t]+)(?:\[([ xX])\]([ \t]+|$))?`)
)

func continueMarkdownLine(d *editor.Document) bool {
	if d.ReadOnly || !isMarkdownPath(d.Path) {
		return false
	}
	row, col := d.Position()
	start, end := d.Selection()
	if start != end || col != d.LineRuneCount(row) {
		return false
	}
	line := d.Lines()[row]
	quote := markdownQuotePrefix.FindString(line)
	body := line[len(quote):]
	list := markdownListPrefix.FindStringSubmatch(body)
	if quote == "" && list == nil {
		return false
	}
	lineStart := d.ByteOffset(d.LineOffset(row))
	lineEnd := lineStart + len(line)
	if markdownLiteralBlockAt(d.Text, lineStart, lineEnd) {
		return false
	}

	next := quote
	empty := strings.Trim(body, " \t") == ""
	if list != nil {
		empty = strings.Trim(body[len(list[0]):], " \t") == ""
		marker := list[2]
		if list[3] != "" {
			n, _ := strconv.Atoi(list[3])
			if n < 999_999_999 {
				marker = strconv.Itoa(n+1) + marker[len(marker)-1:]
			}
		}
		next += list[1] + marker + list[4]
		if list[5] != "" {
			spacing := list[6]
			if spacing == "" {
				spacing = " "
			}
			next += "[ ]" + spacing
		}
	}
	if empty {
		keep := quote
		if list == nil {
			last := strings.LastIndexByte(quote, '>')
			keep = quote[:last]
		}
		d.Edit(d.Text[:lineStart] + keep + d.Text[lineEnd:])
		d.SetPosition(row, len(keep))
		return true
	}
	d.Insert("\n" + next)
	return true
}

func markdownLiteralBlockAt(source string, start, end int) bool {
	doc := markdownParser.Parse(text.NewReader([]byte(source)))
	code := false
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.Kind() {
		case ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindHTMLBlock:
			lines := node.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				if line.Start <= end && line.Stop > start {
					code = true
					return ast.WalkStop, nil
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return code
}
