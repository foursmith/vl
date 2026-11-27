package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/theme"
)

func TestDiffWheelCentersLastRowInBothLayouts(t *testing.T) {
	for _, split := range []bool{false, true} {
		m := tabWorkspace(t, 0)
		t.Cleanup(m.Close)
		m.Update(tea.WindowSizeMsg{Width: 160, Height: 15})
		m.split = split
		m.diffPath = "file.txt"
		m.patch = diff.Parse("@@ -1 +1,31 @@\n same\n" + strings.Repeat("+line\n", 29) + "+last\n")
		showDiffFixture(m)
		if m.diffLayout.split != split {
			t.Fatalf("requested split=%v, got split=%v", split, m.diffLayout.split)
		}
		cursor := m.review.Cursor
		for range 30 {
			m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelDown})
		}
		for _, height := range []int{15, 25} {
			m.Update(tea.WindowSizeMsg{Width: 160, Height: height})
			rows := m.diffView()
			if len(rows) != m.contentHeight()/2+1 || !strings.Contains(plain(rows[len(rows)-1]), "last") {
				t.Fatalf("split=%v, height=%d: last diff row did not render in the middle", split, height)
			}
		}
		if m.review.Cursor != cursor {
			t.Fatal("wheel scrolling moved the diff cursor")
		}
	}
}

func TestDiffResizeReusesRowsAndReflowsWhenSplitChanges(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.width, m.height = 160, 20
	m.split = true
	showDiffFixture(m)
	m.diffPath = "file.txt"
	raw := "@@ -1 +1 @@\n-" + strings.Repeat("x", 160) + "\n+" + strings.Repeat("x", 159) + "y\n"
	m.patch = diff.Parse(raw)
	m.renderDiff()
	if !m.diffLayout.split {
		t.Fatal("expected split diff at initial width")
	}
	rows, review := &m.diffRows[0], m.review
	m.scrollHorizontal(1000)
	left := m.diffLeft

	m.width, m.height = 170, 25
	m.renderDiff()
	if &m.diffRows[0] != rows || m.review != review {
		t.Fatal("resizing rebuilt width-independent diff rows")
	}
	if m.diffLeft >= left || m.viewport.Height() != m.contentHeight() {
		t.Fatal("resizing did not update horizontal bounds or viewport height")
	}

	pair := &m.patchMetrics.pairs[0]
	m.width = 60
	m.renderDiff()
	if m.diffLayout.split || &m.diffRows[0] == rows || &m.patchMetrics.pairs[0] != pair {
		t.Fatal("crossing the split threshold must rebuild rows and retain patch analysis")
	}

	// Replacing the patch produces new line identities, even if the text matches.
	m.patch = diff.Parse(raw)
	m.diffLayout = diffLayoutKey{}
	m.renderDiff()
	if &m.patchMetrics.pairs[0] == pair || m.diffRows[0].Left == nil ||
		!reflect.DeepEqual(m.patchMetrics.inline[m.diffRows[0].Left], []diff.Range{{Start: 159, End: 160}}) {
		t.Fatal("replacing a patch retained emphasis keyed to the old lines")
	}
}

func TestSharedDiffSyntaxKeepsSideAndHunkStatesSeparate(t *testing.T) {
	patch := diff.Parse("--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,3 @@\n-/*\n+// opening\n var n = 1\n-*/\n+// closing\n@@ -20 +20 @@\n-var later = 1\n+var later = 2\n")
	colors := theme.Builtin("dark_modern")
	ctx := context.Background()
	got := diffSpans(ctx, patch, "file.go", colors, highlight.LanguageInfo{})
	oldSide := new(highlight.Session).Lines(ctx, highlight.Language("file.go"), "/*\nvar n = 1\n*/\n", colors)
	for i := range patch.Lines {
		line := &patch.Lines[i]
		if line.Kind != diff.Context && line.Kind != diff.Added && line.Kind != diff.Deleted {
			continue
		}
		want := new(highlight.Session).Lines(ctx, highlight.Language("file.go"), line.Text[1:]+"\n", colors)[0]
		if line.Kind == diff.Deleted && line.Text == "-*/" {
			want = oldSide[2]
		}
		if !reflect.DeepEqual(got[line], want) {
			t.Fatalf("lexical state leaked into %q: %#v, want %#v", line.Text, got[line], want)
		}
	}
}

func TestAddedAndDeletedFilesUseFullWidthDiff(t *testing.T) {
	for _, tc := range []struct {
		name, header, hunk, prefix string
	}{
		{"added", "new file mode 100644\n--- /dev/null\n+++ b/file.txt\n", "@@ -0,0 +1 @@\n", "+"},
		{"deleted", "deleted file mode 100644\n--- a/file.txt\n+++ /dev/null\n", "@@ -1 +0,0 @@\n", "-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			m.width, m.height = 180, 30
			showDiffFixture(m)
			m.split = true
			m.diffPath = "file.txt"
			text := strings.Repeat("x", 90)
			m.patch = diff.Parse("diff --git a/file.txt b/file.txt\n" + tc.header + tc.hunk + tc.prefix + text + "\n")
			for _, commit := range []string{"", "abc123"} {
				m.diffCommit = commit
				m.renderDiff()
				if m.diffLayout.split || m.diffModeButton().label != m.symbol(symbolDiffUnified) || !m.split {
					t.Fatal("single-sided file did not override the split preference")
				}
				lines := m.diffView()
				if len(lines) != 1 || !strings.Contains(plain(lines[0]), text) || ansi.StringWidth(lines[0]) != m.mainWidth() {
					t.Fatalf("expected one full-width content row: %q", lines)
				}
				m.positionDiff(m.mainX()+m.diffGutter+75, editorContentRow, false)
				if m.diffSelection.right || m.diffSelection.cursor.col != 75 {
					t.Fatalf("selection still uses split geometry: %+v", m.diffSelection)
				}
			}
			m.patch = diff.Parse("--- a/file.txt\n+++ b/file.txt\n@@ -1 +1,2 @@\n same\n+extra\n")
			m.renderDiff()
			if !m.diffLayout.split || m.diffModeButton().label != m.symbol(symbolDiffSplit) {
				t.Fatal("modified file lost the split preference")
			}
		})
	}
}

func TestDiffThemeGuttersSyntaxAndMissingLines(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.width, m.height = 180, 30
	showDiffFixture(m)
	m.split = true
	m.diffPath = "file.go"
	m.patch = diff.Parse("--- a/file.go\n+++ b/file.go\n@@ -9999,2 +9999,3 @@\n-var old = 1\n+var old = 2\n+// added\n same\n")
	m.renderDiff()
	m.diffSyntax = diffSpans(m.ctx, m.patch, m.diffPath, m.colorTheme, highlight.LanguageInfo{})
	foundSyntax, foundPlaceholder := false, false
	for _, pair := range m.diffRows {
		if pair.Right == nil || pair.Left == nil {
			foundPlaceholder = true
			if !strings.Contains(plain(m.diffRow(nil, 40, false, diffRange{})), "╱") {
				t.Fatal("missing line is not hatched")
			}
		}
		for _, line := range []*diff.Line{pair.Left, pair.Right} {
			if line == nil || line.Kind != diff.Added && line.Kind != diff.Deleted {
				continue
			}
			rendered := m.diffRow(line, 40, line.Kind == diff.Added, diffRange{})
			if ansi.StringWidth(rendered) != 40 {
				t.Fatal("diff row width changed")
			}
			cells := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0]
			bg := theme.Builtin("dark_modern").Palette().AddedBackground
			if line.Kind == diff.Deleted {
				bg = theme.Builtin("dark_modern").Palette().DeletedBackground
			}
			for x, cell := range cells {
				want := bg
				if line.Text != "+// added" && x-m.diffGutter == len("var old = ") {
					want = theme.Builtin("dark_modern").Palette().AddedText
					if line.Kind == diff.Deleted {
						want = theme.Builtin("dark_modern").Palette().DeletedText
					}
				}
				if x < m.diffGutter {
					want = theme.Builtin("dark_modern").Palette().Background
				}
				if !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
					t.Fatalf("background at %d: %v", x, cell.Style.Bg)
				}
				if x >= m.diffGutter && cell.Content == "v" && sameColor(cell.Style.Fg, lipgloss.Color("#569CD6")) {
					foundSyntax = true
				}
			}
		}
	}
	if !foundSyntax || !foundPlaceholder || m.diffGutter != 7 {
		t.Fatalf("syntax=%v placeholder=%v gutter=%d", foundSyntax, foundPlaceholder, m.diffGutter)
	}
	for _, width := range []int{1, 4, 6, 20} {
		if ansi.StringWidth(m.diffRow(&m.patch.Lines[3], width, false, diffRange{})) != width {
			t.Fatalf("narrow width %d overflowed", width)
		}
	}
}

func TestDiffSyntaxResultsCannotReplaceNewPatch(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.diffPath = "file.go"
	m.patch = diff.Parse("@@ -1 +1 @@\n-old\n+var value = 2\n")
	cmd := m.highlightDiff()
	result := cmd().(diffSyntaxMsg)
	m.diffSeq++
	m.Update(result)
	if m.diffSyntax != nil {
		t.Fatal("stale syntax applied")
	}
	result = m.highlightDiff()().(diffSyntaxMsg)
	m.Update(result)
	if len(m.diffSyntax) == 0 {
		t.Fatal("current syntax missing")
	}
	m.diffState.clearContent()
	if result.ctx.Err() == nil || m.diffSyntax != nil {
		t.Fatal("closed diff kept syntax work")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if diffSpans(ctx, m.patch, "file.go", theme.Builtin("dark_modern"), highlight.LanguageInfo{}) != nil {
		t.Fatal("cancelled work continued")
	}
}

func TestDiffSyntaxFontsKeepDiffBackground(t *testing.T) {
	p := theme.Builtin("dark_modern").Palette()
	line := diffCode(p, "bold", []highlight.Span{{Start: 0, End: 4, Color: "#569CD6", Font: theme.Bold}}, 12, 0, p.AddedBackground, p.AddedText, []diff.Range{{Start: 0, End: 4}}, diffRange{})
	cells := uv.NewStyledString(line).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range cells {
		want := p.AddedBackground
		if i < 4 {
			want = p.AddedText
		}
		if !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
			t.Fatal("TextMate font reset diff background")
		}
		if i < 4 && cell.Style.Attrs&uv.AttrBold == 0 {
			t.Fatal("diff lost bold syntax style")
		}
	}
}

func TestDiffPaintsSeparateInlineChangesInBothLayouts(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	showDiffFixture(m)
	m.diffPath = "file.go"
	m.split = true
	m.patch = diff.Parse("@@ -1 +1,2 @@\n-load(\"alpha\", keepMiddle(), 100)\n+audit(\"new event\")\n+load(\"beta\", keepMiddle(), 120)\n")
	for _, width := range []int{180, 60} {
		m.width = width
		m.renderDiff()
		right := m.diffLayout.split
		pane, paneWidth := m.diffPane(right)
		rendered := m.diffView()
		for i, pair := range m.diffRows {
			line := pair.Left
			if right {
				line = pair.Right
			}
			if line == nil || line.Kind != diff.Added {
				continue
			}
			code := line.Text[1:]
			cells := uv.NewStyledString(rendered[i]).Lines(ansi.GraphemeWidth)[0]
			for col := 0; col < paneWidth-m.diffGutter; col++ {
				want := m.colors.AddedBackground
				if strings.HasPrefix(code, "load") && (col >= 6 && col < 10 || col == 28) {
					want = m.colors.AddedText
				}
				if got := cells[pane+m.diffGutter+col].Style.Bg; !sameColor(got, lipgloss.Color(want)) {
					t.Fatalf("width %d, %q at column %d: got %v, want %s", width, code, col, got, want)
				}
			}
		}
	}
}

func TestCommitAndChangesShareDiffRenderingAndHunkNavigation(t *testing.T) {
	m := tabWorkspace(t, 0)
	showDiffFixture(m)
	m.diffPath = "file.go"
	m.patch = diff.Parse("diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,2 +1,2 @@\n-old\n+new\n same\n@@ -20,2 +20,3 @@\n before\n-old again\n+new again\n+extra\n")
	for _, split := range []bool{false, true} {
		m.split = split
		m.width = 180
		m.height = 30
		m.diffCommit = ""
		m.diffHunk = 0
		m.renderDiff()
		m.viewport.GotoTop()
		changes := strings.Join(m.diffView(), "\n")
		m.diffCommit = strings.Repeat("a", 40)
		m.renderDiff()
		m.viewport.GotoTop()
		if got := strings.Join(m.diffView(), "\n"); got != changes {
			t.Fatal("commit uses a different diff renderer")
		}
		m.nextHunk(1)
		if m.diffHunk != 1 {
			t.Fatal("commit hunk navigation differs")
		}
		m.nextHunk(-1)
		if m.diffHunk != 0 {
			t.Fatal("commit previous hunk navigation differs")
		}
	}
}
