package app

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/ourongxing/elegit/internal/diff"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

type diffState struct {
	patch            diff.Patch
	diffPath         string
	diffRepository   string
	diffChange       *gitservice.Change
	diffStaged       bool
	diffHunk         int
	diffRowHunks     []int
	diffRows         []diff.Pair
	diffGutter       int
	diffLeft         int
	diffWidth        int
	diffLayout       diffLayoutKey
	diffSelection    diffSelection
	diffLoading      bool
	diffCancel       context.CancelFunc
	diffSyntaxCancel context.CancelFunc
	diffSyntax       map[*diff.Line][]highlight.Span
	diffEmphasis     map[*diff.Line]diffRange
	diffOpen         bool
	diffCommit       string
	diffSeq          int
	viewport         viewport.Model
	read             func(context.Context) (string, error)
}

func newDiffState() *diffState {
	return &diffState{viewport: viewport.New(), diffHunk: -1}
}

func (d *diffState) clearContent() {
	if d.diffSyntaxCancel != nil {
		d.diffSyntaxCancel()
		d.diffSyntaxCancel = nil
	}
	d.diffSyntax, d.diffEmphasis = nil, nil
	d.patch = diff.Patch{}
	d.diffRows, d.diffRowHunks = nil, nil
	d.diffLayout = diffLayoutKey{}
	d.diffSelection = diffSelection{}
	d.diffLeft, d.diffWidth = 0, 0
	d.diffHunk = -1
	d.viewport.SetContent("")
	d.viewport.GotoTop()
}

func (d *diffState) close() {
	d.diffOpen = false
	d.diffSeq++
	if d.diffCancel != nil {
		d.diffCancel()
		d.diffCancel = nil
	}
	d.diffLoading = false
	d.read = nil
	d.clearContent()
}

// A tab identity includes the review source: a document, worktree diff,
// index diff and commit diff of the same path are distinct tabs.
type diffTarget struct {
	path, repository, commit string
	staged                   bool
	preview                  bool
	change                   *gitservice.Change
}

func (d *diffState) matches(target diffTarget) bool {
	return d.diffPath == target.path && d.diffRepository == target.repository && d.diffCommit == target.commit && d.diffStaged == target.staged
}

func (m *Model) requestDiff(target diffTarget, read func(context.Context) (string, error)) tea.Cmd {
	index := -1
	for i, t := range m.tabs {
		if t.diff != nil && t.diff.matches(target) {
			index = i
			break
		}
	}
	if index < 0 {
		d := newDiffState()
		d.diffOpen = true
		d.diffPath, d.diffRepository, d.diffCommit = target.path, target.repository, target.commit
		d.diffStaged = target.staged
		index = m.insertTab(tab{diff: d, preview: target.preview})
	} else if !target.preview {
		m.tabs[index].preview = false
	}
	d := m.tabs[index].diff
	d.diffChange, d.read = target.change, read
	// Opening selects a tab; refreshing an existing tab is handled separately.
	sidebar := m.sidebar
	m.activateTab(index)
	m.sidebar = sidebar
	return m.loadDiff(d, true)
}

func (m *Model) loadDiff(d *diffState, reset bool) tea.Cmd {
	if d.diffCancel != nil {
		d.diffCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	d.diffCancel = cancel
	d.diffSeq++
	d.diffLoading = true
	if reset {
		d.clearContent()
	}
	seq, read := d.diffSeq, d.read
	return func() tea.Msg {
		defer cancel()
		raw, err := read(ctx)
		return diffMsg{target: d, seq: seq, raw: raw, err: err}
	}
}

// Refresh tab-owned sources, never the sidebar selection. Keep focus, tab
// order, scroll and hunk position while new Git output is being fetched.
func (m *Model) refreshDiffs() tea.Cmd {
	var commands []tea.Cmd
	for _, t := range m.tabs {
		d := t.diff
		if d == nil || d.diffCommit != "" || d.read == nil {
			continue
		}
		if d.diffChange != nil {
			change := *d.diffChange
			found := false
			for _, c := range m.snapshot.Changes {
				if c.Path == d.diffPath && c.Repository == d.diffRepository {
					change, found = c, true
					break
				}
			}
			if !found {
				change.Index, change.Worktree = ' ', ' '
			}
			staged := d.diffStaged
			d.diffChange = &change
			workspace := m.repo
			d.read = func(ctx context.Context) (string, error) { return workspace.Diff(ctx, change, staged) }
		}
		commands = append(commands, m.loadDiff(d, false))
	}
	return tea.Batch(commands...)
}

func (m *Model) finishDiff(msg diffMsg) tea.Cmd {
	d := msg.target
	if d == nil || !d.diffOpen || msg.seq != d.diffSeq {
		return nil
	}
	bound := m.diffState
	m.diffState = d
	defer func() { m.diffState = bound }()
	d.diffLoading = false
	if msg.err != nil {
		m.log(msg.err.Error())
		return nil
	}
	if msg.raw != d.patch.Raw {
		d.patch = diff.Parse(msg.raw)
		d.diffLayout = diffLayoutKey{}
		if len(d.patch.Hunks) > 0 {
			d.diffHunk = min(max(0, d.diffHunk), len(d.patch.Hunks)-1)
		} else {
			d.diffHunk = -1
		}
		d.diffSyntax = nil
	}
	d.viewport.SetWidth(m.mainWidth())
	d.viewport.SetHeight(m.contentHeight())
	m.renderDiff()
	if d.diffSyntax == nil {
		return m.highlightDiff()
	}
	return nil
}

type diffSyntaxMsg struct {
	target *diffState
	seq    int
	raw    string
	ctx    context.Context
	spans  map[*diff.Line][]highlight.Span
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
	target := m.diffState
	patch, path, seq := m.patch, m.diffPath, m.diffSeq
	light := m.light
	return func() tea.Msg {
		return diffSyntaxMsg{target: target, seq: seq, raw: patch.Raw, ctx: ctx, spans: diffSpans(ctx, patch, path, light)}
	}
}

// Tokenize each side separately so removed code cannot change the new side's
// string/comment state. Bound all syntax work by the editor's byte budget.
func diffSpans(ctx context.Context, patch diff.Patch, path string, light bool) map[*diff.Line][]highlight.Span {
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
			spans := highlight.Lines(ctx, path, source.String(), light)
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
			lines = append(lines, m.diffRow(pair.Left, w, false, m.diffSelectedRange(i, false)))
		} else {
			left := (w - 1) / 2
			lines = append(lines, m.diffRow(pair.Left, left, false, m.diffSelectedRange(i, false))+row("│", 1, m.colors.Border, m.colors.Background)+m.diffRow(pair.Right, w-left-1, true, m.diffSelectedRange(i, true)))
		}
	}
	return lines
}

func (m *Model) diffRow(line *diff.Line, width int, right bool, selected diffRange) string {
	gutter := min(width, max(6, m.diffGutter))
	if line == nil {
		return row("", gutter, m.colors.LineNumber, m.colors.Background) + row(strings.Repeat("╱", max(0, width-gutter)), width-gutter, m.colors.DiffPlaceholder, m.colors.Background)
	}
	fg, bg := m.colors.Foreground, m.colors.Background
	text := ""
	switch line.Kind {
	case diff.Header:
		text = safe(line.Text)
		fg = m.colors.Muted
		if m.diffPath != "" && strings.HasPrefix(line.Text, "Binary files ") {
			text = "  二进制文件已更改"
		}
	case diff.HunkHeader:
		fg, bg = m.colors.Muted, m.colors.Panel
		marker := " "
		if line.Hunk == m.diffHunk {
			marker = m.symbol(symbolRight)
			fg = m.colors.Foreground
		}
		text = " " + marker + " " + hunkLabel(line.Text)
	case diff.Added:
		bg = m.colors.AddedBackground
	case diff.Deleted:
		bg = m.colors.DeletedBackground
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
		emphasis = m.colors.AddedText
	} else if line.Kind == diff.Deleted {
		emphasis = m.colors.DeletedText
	}
	return row(label, gutter, m.colors.LineNumber, m.colors.Background) + diffCode(m.colors, code, m.diffSyntax[line], width-gutter, m.diffLeft, bg, emphasis, m.diffEmphasis[line], selected)
}

// Paint token runs only up to the visible right edge, preserving source rune
// offsets for syntax and emphasis while clipping by terminal display columns.
func diffCode(colors theme.Palette, code string, spans []highlight.Span, width, left int, bg, emphasis string, changed diffRange, selected diffRange) string {
	if width <= 0 {
		return ""
	}
	var out, run strings.Builder
	color := colors.Foreground
	var font highlight.FontStyle
	runBG := bg
	spanIndex, column, visual := 0, 0, 0
	flush := func() {
		if run.Len() > 0 {
			out.WriteString(syntaxStyle(color, runBG, font).Render(run.String()))
			run.Reset()
		}
	}
	iter := graphemes.FromString(code)
	for iter.Next() {
		if visual >= left+width {
			break
		}
		cluster := iter.Value()
		display := displayCluster(cluster, visual)
		start := visual
		visual += ansi.StringWidth(display)
		runeColumn := column
		column += utf8.RuneCountInString(cluster)
		if visual <= left {
			continue
		}
		for spanIndex < len(spans) && runeColumn >= spans[spanIndex].End {
			spanIndex++
		}
		next := colors.Foreground
		var nextFont highlight.FontStyle
		if spanIndex < len(spans) && runeColumn >= spans[spanIndex].Start {
			next = spans[spanIndex].Color
			nextFont = spans[spanIndex].Font
		}
		nextBG := bg
		if emphasis != "" && runeColumn >= changed.start && runeColumn < changed.end {
			nextBG = emphasis
		}
		if runeColumn >= selected.start && runeColumn < selected.end {
			nextBG = colors.Selection
		}
		if next != color || nextBG != runBG || nextFont != font {
			flush()
			color, font = next, nextFont
			runBG = nextBG
		}
		run.WriteString(ansi.Cut(display, max(0, left-start), min(visual-start, left+width-start)))
	}
	flush()
	if emphasis != "" && changed.end > column {
		bg = emphasis
	}
	return row(out.String(), width, colors.Foreground, bg)
}
