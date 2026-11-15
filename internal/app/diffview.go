package app

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

// Both review sources share request cancellation, result identity and viewport state.
func (m *Model) requestDiff(path string, reset bool, read func(context.Context) (string, error)) tea.Cmd {
	if m.diffCancel != nil {
		m.diffCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.diffCancel = cancel
	m.diffSeq++
	seq := m.diffSeq
	m.diffOpen, m.diffLoading, m.showEditor = true, true, false
	m.diffPath = path
	if reset {
		m.clearDiffContent()
		m.diffHunk = -1
		m.viewport.GotoTop()
	} else if m.diffSyntaxCancel != nil {
		m.diffSyntaxCancel()
		m.diffSyntaxCancel = nil
	}
	m.ensureTabVisible()
	return func() tea.Msg {
		defer cancel()
		raw, err := read(ctx)
		return diffMsg{seq: seq, path: path, raw: raw, err: err}
	}
}

type diffSyntaxMsg struct {
	seq   int
	raw   string
	ctx   context.Context
	spans map[*diff.Line][]highlight.Span
}

type diffRange struct{ start, end int }

// Cache intraline emphasis with the patch layout, never during scrolling.
func diffEmphasis(pairs []diff.Pair) map[*diff.Line]diffRange {
	ranges := make(map[*diff.Line]diffRange)
	for _, pair := range pairs {
		left, right := pair.Left, pair.Right
		if left == nil && right != nil && right.Kind == diff.Added {
			ranges[right] = diffRange{0, utf8.RuneCountInString(right.Text[1:]) + 1}
		} else if right == nil && left != nil && left.Kind == diff.Deleted {
			ranges[left] = diffRange{0, utf8.RuneCountInString(left.Text[1:]) + 1}
		} else if left != nil && right != nil && left.Kind == diff.Deleted && right.Kind == diff.Added {
			a, b := []rune(left.Text[1:]), []rune(right.Text[1:])
			start, endA, endB := 0, len(a), len(b)
			for start < min(endA, endB) && a[start] == b[start] {
				start++
			}
			for endA > start && endB > start && a[endA-1] == b[endB-1] {
				endA--
				endB--
			}
			ranges[left], ranges[right] = diffRange{start, endA}, diffRange{start, endB}
		}
	}
	return ranges
}

func (m *Model) highlightDiff() tea.Cmd {
	if m.diffSyntaxCancel != nil {
		m.diffSyntaxCancel()
	}
	m.diffSyntax = nil
	if m.patch.Raw == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.diffSyntaxCancel = cancel
	patch, path, seq := m.patch, m.diffPath, m.diffSeq
	return func() tea.Msg {
		return diffSyntaxMsg{seq, patch.Raw, ctx, diffSpans(ctx, patch, path)}
	}
}

// Tokenize each side separately so removed code cannot change the new side's
// string/comment state. Bound all syntax work by the editor's byte budget.
func diffSpans(ctx context.Context, patch diff.Patch, path string) map[*diff.Line][]highlight.Span {
	if len(patch.Raw) > highlight.MaxBytes || ctx.Err() != nil {
		return nil
	}
	result := make(map[*diff.Line][]highlight.Span)
	var old, added []*diff.Line
	flush := func() {
		for _, side := range [][]*diff.Line{old, added} {
			if len(side) == 0 {
				continue
			}
			var source strings.Builder
			for _, line := range side {
				source.WriteString(line.Text[1:])
				source.WriteByte('\n')
			}
			spans := highlight.Lines(ctx, path, source.String())
			for i, line := range side {
				if i < len(spans) {
					result[line] = spans[i]
				}
			}
		}
		old, added = nil, nil
	}
	for i := range patch.Lines {
		if ctx.Err() != nil {
			return nil
		}
		line := &patch.Lines[i]
		switch line.Kind {
		case diff.Header:
			if strings.HasPrefix(line.Text, "diff --git ") {
				flush()
			}
			if strings.HasPrefix(line.Text, "+++ b/") {
				path = strings.TrimPrefix(line.Text, "+++ b/")
			}
			if strings.HasPrefix(line.Text, "--- a/") {
				path = strings.TrimPrefix(line.Text, "--- a/")
			}
		case diff.HunkHeader:
			// Hunks omit source between them; don't leak lexical state across that gap.
			flush()
		case diff.Context:
			old = append(old, line)
			added = append(added, line)
		case diff.Deleted:
			old = append(old, line)
		case diff.Added:
			added = append(added, line)
		}
	}
	flush()
	return result
}

func (m *Model) diffView() []string {
	var lines []string
	w := m.mainWidth()
	for i := m.viewport.YOffset(); i < min(len(m.diffRows), m.viewport.YOffset()+m.contentHeight()); i++ {
		pair := m.diffRows[i]
		if !m.diffLayout.split || pair.Left != nil && (pair.Left.Kind == diff.Header || pair.Left.Kind == diff.HunkHeader) {
			lines = append(lines, m.diffRow(pair.Left, w, false))
		} else {
			left := (w - 1) / 2
			lines = append(lines, m.diffRow(pair.Left, left, false)+row("│", 1, theme.Border, theme.Background)+m.diffRow(pair.Right, w-left-1, true))
		}
	}
	return lines
}

func (m *Model) diffRow(line *diff.Line, width int, right bool) string {
	gutter := min(width, max(6, m.diffGutter))
	if line == nil {
		return row("", gutter, theme.LineNumber, theme.Background) + row(strings.Repeat("╱", max(0, width-gutter)), width-gutter, theme.DiffPlaceholder, theme.Background)
	}
	fg, bg := theme.Foreground, theme.Background
	text := ""
	switch line.Kind {
	case diff.Header:
		text = safe(line.Text)
		fg = theme.Muted
		if m.diffPath != "" && strings.HasPrefix(line.Text, "Binary files ") {
			text = "  二进制文件已更改"
		}
	case diff.HunkHeader:
		fg, bg = theme.Muted, theme.Panel
		marker := " "
		if line.Hunk == m.diffHunk {
			marker = m.symbol(symbolRight)
			fg = theme.Foreground
		}
		text = " " + marker + " " + hunkLabel(line.Text)
	case diff.Added:
		bg = theme.AddedBackground
	case diff.Deleted:
		bg = theme.DeletedBackground
	}
	if line.Kind != diff.Added && line.Kind != diff.Deleted && line.Kind != diff.Context {
		return row(text, width, fg, bg)
	}
	number := line.Old
	if right || line.Kind == diff.Added {
		number = line.New
	}
	marker := " "
	if line.Kind == diff.Added {
		marker = "+"
	} else if line.Kind == diff.Deleted {
		marker = "−"
	}
	label := fmt.Sprintf("%*d%s ", gutter-2, number, marker)
	code := line.Text[1:]
	emphasis := ""
	if line.Kind == diff.Added {
		emphasis = theme.AddedText
	} else if line.Kind == diff.Deleted {
		emphasis = theme.DeletedText
	}
	return row(label, gutter, theme.LineNumber, theme.Background) + diffCode(code, m.diffSyntax[line], width-gutter, bg, emphasis, m.diffEmphasis[line])
}

// Clip before painting and render token runs, keeping work proportional to the
// visible columns rather than the length of a source line.
func diffCode(code string, spans []highlight.Span, width int, bg, emphasis string, changed diffRange) string {
	if width <= 0 {
		return ""
	}
	code = ansi.Truncate(code, width, "")
	if len(spans) == 0 && changed.start == changed.end {
		return row(safe(code), width, theme.Foreground, bg)
	}
	var out, run strings.Builder
	color := theme.Foreground
	var font highlight.FontStyle
	runBG := bg
	spanIndex, column := 0, 0
	flush := func() {
		if run.Len() > 0 {
			out.WriteString(syntaxStyle(color, runBG, font).Render(run.String()))
			run.Reset()
		}
	}
	for _, r := range code {
		for spanIndex < len(spans) && column >= spans[spanIndex].End {
			spanIndex++
		}
		next := theme.Foreground
		var nextFont highlight.FontStyle
		if spanIndex < len(spans) && column >= spans[spanIndex].Start {
			next = spans[spanIndex].Color
			nextFont = spans[spanIndex].Font
		}
		nextBG := bg
		if emphasis != "" && column >= changed.start && column < changed.end {
			nextBG = emphasis
		}
		if next != color || nextBG != runBG || nextFont != font {
			flush()
			color, font = next, nextFont
			runBG = nextBG
		}
		run.WriteString(safe(string(r)))
		column++
	}
	flush()
	if emphasis != "" && changed.end > column {
		bg = emphasis
	}
	return row(out.String(), width, theme.Foreground, bg)
}
