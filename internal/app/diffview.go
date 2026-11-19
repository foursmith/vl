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
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/theme"
)

type diffState struct {
	review           *editor.Document
	reviewRows       []int
	reviewLine       map[*diff.Line]int
	patch            diff.Patch
	diffPath         string
	diffOriginal     string
	diffRepository   string
	diffChange       *gitservice.Change
	diffStaged       bool
	followIndex      bool
	diffHunk         int
	diffRowHunks     []int
	diffRows         []diff.Pair
	diffGutter       int
	diffLeft         int
	diffWidth        int
	diffLayout       diffLayoutKey
	diffSelection    diffSelection
	diffLoading      bool
	diffStale        bool
	diffCancel       context.CancelFunc
	diffSyntaxCancel context.CancelFunc
	diffSyntax       map[*diff.Line][]highlight.Span
	diffEmphasis     map[*diff.Line]diffRange
	diffOpen         bool
	diffCommit       string
	diffPR           gitservice.PullRequest
	diffPRVersion    string
	diffSeq          int
	viewport         viewport.Model
	read             func(context.Context) (string, error)
	images           *diffImages
}

func newDiffState() *diffState {
	view := viewport.New()
	view.KeyMap = viewport.KeyMap{} // Text navigation belongs to the shared editor.
	return &diffState{viewport: view, diffHunk: -1}
}

func (m *Model) splitDiff() bool {
	return m.split && m.mainWidth() >= 68 && !m.patch.NewFile && !m.patch.DeletedFile
}

func (m *Model) setDiffScrollRows() {
	rows := len(m.diffRows)
	if rows > 0 {
		height := m.viewport.Height()
		rows += height - height/2 - 1
	}
	// The viewport tracks scroll geometry, including the blank space below the last row.
	m.viewport.SetContentLines(make([]string, rows))
}

func (d *diffState) clearContent() {
	if d.diffSyntaxCancel != nil {
		d.diffSyntaxCancel()
		d.diffSyntaxCancel = nil
	}
	d.diffSyntax, d.diffEmphasis = nil, nil
	d.patch = diff.Patch{}
	d.images = nil
	d.diffRows, d.diffRowHunks = nil, nil
	d.diffLayout = diffLayoutKey{}
	d.diffSelection = diffSelection{}
	d.review = nil
	d.reviewRows = nil
	d.reviewLine = nil
	d.diffLeft, d.diffWidth = 0, 0
	d.diffHunk = -1
	d.viewport.SetContent("")
	d.viewport.GotoTop()
}

func (d *diffState) cancelLoad() {
	d.diffSeq++
	if d.diffCancel != nil {
		d.diffCancel()
		d.diffCancel = nil
	}
	d.diffLoading = false
}

func (d *diffState) close() {
	d.diffOpen = false
	d.cancelLoad()
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
	pr                       gitservice.PullRequest
	prVersion                string
}

func (d *diffState) matches(target diffTarget) bool {
	return d.diffPath == target.path && d.diffRepository == target.repository && d.diffCommit == target.commit &&
		d.diffStaged == target.staged && d.diffPR.URL == target.pr.URL
}

func (m *Model) requestDiff(target diffTarget, read func(context.Context) (string, error)) tea.Cmd {
	d := m.openDiffTab(target)
	// Update refreshes an outdated Changes diff after selection.
	if d.diffChange != nil && (d.diffStale || d.diffLoading || d.patch.Raw != "") {
		return nil
	}
	d.read = read
	return m.loadDiff(d, true)
}

func (m *Model) openDiffTab(target diffTarget) *diffState {
	index := -1
	for i, t := range m.tabs {
		if t.diff != nil && t.diff.matches(target) {
			index = i
			break
		}
	}
	var d *diffState
	if index < 0 {
		d = newDiffState()
		d.diffOpen = true
		d.diffPath, d.diffRepository, d.diffCommit = target.path, target.repository, target.commit
		d.diffStaged = target.staged
	} else {
		d = m.tabs[index].diff
		if !target.preview {
			m.tabs[index].preview = false
		}
	}
	d.diffPR, d.diffPRVersion = target.pr, target.prVersion
	d.diffChange = target.change
	if target.change != nil {
		d.diffOriginal = target.change.Original
	}
	// Commit metadata must not enable worktree staging or editing actions.
	if target.commit != "" || target.pr.Number != 0 {
		d.diffChange = nil
	}
	if index < 0 {
		index = m.insertTab(tab{diff: d, preview: target.preview})
	}
	// Opening selects a tab; refreshing an existing tab is handled separately.
	m.selectTab(index)
	return d
}

func (m *Model) loadDiff(d *diffState, reset bool) tea.Cmd {
	read := d.read
	return m.loadVersionedDiff(d, reset, func(ctx context.Context) (string, string, error) {
		raw, err := read(ctx)
		return raw, "", err
	})
}

func (m *Model) loadVersionedDiff(d *diffState, reset bool, read func(context.Context) (string, string, error)) tea.Cmd {
	d.cancelLoad()
	ctx, cancel := context.WithCancel(m.ctx)
	d.diffCancel = cancel
	d.diffLoading = true
	d.diffStale = false
	if reset {
		m.setDiffImages(d, nil)
		d.clearContent()
	}
	seq := d.diffSeq
	imageRead := m.diffImageReader(d)
	return func() tea.Msg {
		defer cancel()
		raw, version, err := read(ctx)
		msg := diffMsg{target: d, seq: seq, raw: raw, prVersion: version, err: err}
		if err == nil && raw != "" && imageRead != nil {
			msg.images = imageRead(ctx)
		}
		return msg
	}
}

func (m *Model) finishDiff(msg diffMsg) tea.Cmd {
	d := msg.target
	if d == nil || !d.diffOpen || msg.seq != d.diffSeq {
		return nil
	}
	d.diffLoading = false
	if d.diffStale {
		return nil
	}
	if d.diffPR.Number != 0 {
		d.read = nil
	}
	if msg.err != nil {
		m.notice = m.tr(msg.err.Error())
		return nil
	}
	if msg.prVersion != "" {
		d.diffPRVersion = msg.prVersion
	}
	followIndex := d.followIndex
	d.followIndex = false
	if msg.raw == "" {
		for i, t := range m.tabs {
			if t.diff == d {
				if followIndex && canFollowIndex(d) {
					m.followIndexDiff(i)
					return nil
				}
				m.removeTabAt(i)
				break
			}
		}
		return nil
	}
	bound := m.diffState
	m.diffState = d
	defer func() { m.diffState = bound }()
	m.setDiffImages(d, msg.images)
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
	if m.images != nil {
		return nil
	}
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
	colors := m.colorTheme
	return func() tea.Msg {
		return diffSyntaxMsg{target: target, seq: seq, raw: patch.Raw, ctx: ctx, spans: diffSpans(ctx, patch, path, colors)}
	}
}

// Tokenize each side separately so removed code cannot change the new side's
// string/comment state. Bound all syntax work by the editor's byte budget.
func diffSpans(ctx context.Context, patch diff.Patch, path string, colors *theme.Theme) map[*diff.Line][]highlight.Span {
	if len(patch.Raw) > highlight.MaxBytes || ctx.Err() != nil {
		return nil
	}
	result := make(map[*diff.Line][]highlight.Span)
	// One session shares exact line/start-state tokens across the two sides.
	// Each call still starts lexical state at the beginning of its own text.
	var syntax highlight.Session
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
			spans := syntax.Lines(ctx, path, source.String(), colors)
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

const diffSeparatorWidth = 3

func (m *Model) diffPane(right bool) (start, width int) {
	w := m.mainWidth()
	if !m.diffLayout.split {
		return 0, w
	}
	left := (w - diffSeparatorWidth) / 2
	if right {
		start = left + diffSeparatorWidth
		return start, w - start
	}
	return 0, left
}

func (m *Model) diffView() []string {
	if m.images != nil {
		return m.diffImageLines()
	}
	var lines []string
	w := m.mainWidth()
	_, left := m.diffPane(false)
	_, right := m.diffPane(true)
	for i := m.viewport.YOffset(); i < min(len(m.diffRows), m.viewport.YOffset()+m.contentHeight()); i++ {
		pair := m.diffRows[i]
		if !m.diffLayout.split || pair.Left != nil && (pair.Left.Kind == diff.Header || pair.Left.Kind == diff.HunkHeader) {
			lines = append(lines, m.diffRow(pair.Left, w, false, m.diffSelectedRange(i, false)))
		} else {
			lines = append(lines, m.diffRow(pair.Left, left, false, m.diffSelectedRange(i, false))+row(" │ ", diffSeparatorWidth, m.colors.Border, m.colors.Background)+m.diffRow(pair.Right, right, true, m.diffSelectedRange(i, true)))
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
			text = m.tr("  Binary file changed")
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
	emphasis, border := "", ""
	if line.Kind == diff.Added {
		emphasis, border = m.colors.AddedText, m.colors.AddedBorder
	} else if line.Kind == diff.Deleted {
		emphasis, border = m.colors.DeletedText, m.colors.DeletedBorder
	}
	overlay := diffCodeOverlay{changeBorder: border}
	if logical, ok := m.reviewLine[line]; ok && right == m.diffLayout.split && m.review != nil {
		overlay.offset = m.review.LineOffset(logical)
		overlay.matches = m.fileSearchRowRanges(m.review, logical)
		if m.flashValid() {
			overlay.flash = m.flash
		}
	}
	return row(label, gutter, theme.Readable(m.colors.LineNumber, m.colors.Background), m.colors.Background) + diffCode(m.colors, code, m.diffSyntax[line], width-gutter, m.diffLeft, bg, emphasis, m.diffEmphasis[line], selected, overlay)
}

type diffCodeOverlay struct {
	matches      [][2]int
	offset       int
	flash        *flashState
	changeBorder string
}

// Paint token runs only up to the visible right edge, preserving source rune
// offsets for syntax and emphasis while clipping by terminal display columns.
func diffCode(colors theme.Palette, code string, spans []highlight.Span, width, left int, bg, emphasis string, changed diffRange, selected diffRange, overlays ...diffCodeOverlay) string {
	if width <= 0 {
		return ""
	}
	overlay := diffCodeOverlay{}
	if len(overlays) > 0 {
		overlay = overlays[0]
	}
	matches := overlay.matches
	matchIndex := 0
	var out, run strings.Builder
	color := colors.Foreground
	var font theme.FontStyle
	runBG := bg
	runUnderline := ""
	spanIndex, column, visual := 0, 0, 0
	flush := func() {
		if run.Len() > 0 {
			out.WriteString(theme.CodeStyle(color, runBG, runUnderline, font).Render(run.String()))
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
		nextBG := bg
		var nextFont theme.FontStyle
		if spanIndex < len(spans) && runeColumn >= spans[spanIndex].Start {
			next = spans[spanIndex].Color
			nextFont = spans[spanIndex].Font
			if bg == colors.Background && spans[spanIndex].Background != "" {
				nextBG = theme.Composite(spans[spanIndex].Background, bg)
			}
		}
		underline := ""
		if runeColumn >= changed.start && runeColumn < changed.end {
			if emphasis != "" {
				nextBG = emphasis
			}
			underline = overlay.changeBorder
		}
		isSelected := runeColumn >= selected.start && runeColumn < selected.end
		if isSelected {
			if colors.EditorSelection != "" {
				nextBG = theme.Composite(colors.EditorSelection, nextBG)
			}
			if colors.EditorSelectionForeground != "" {
				next = colors.EditorSelectionForeground
			}
		}
		for matchIndex < len(matches) && matches[matchIndex][1] <= runeColumn {
			matchIndex++
		}
		if matchIndex < len(matches) && matches[matchIndex][0] < column {
			var matchBorder string
			next, nextBG, matchBorder = colors.FindColors(next, nextBG, isSelected)
			if matchBorder != "" {
				underline = matchBorder
			}
		}
		if f := overlay.flash; f != nil && f.matches[overlay.offset+runeColumn] {
			nextBG = colors.PopupSelection
			if label := f.marks[overlay.offset+runeColumn]; label != "" {
				display = label + strings.Repeat(" ", max(0, ansi.StringWidth(display)-1))
				next, nextBG, nextFont = colors.Background, colors.Accent, theme.Bold
			}
		}
		if next != color || nextBG != runBG || underline != runUnderline || nextFont != font {
			flush()
			color, font = next, nextFont
			runBG, runUnderline = nextBG, underline
		}
		run.WriteString(ansi.Cut(display, max(0, left-start), min(visual-start, left+width-start)))
	}
	flush()
	if emphasis != "" && changed.end > column {
		bg = emphasis
	}
	return row(out.String(), width, colors.Foreground, bg)
}
