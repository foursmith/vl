package app

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

func TestDiffThemeGuttersSyntaxAndMissingLines(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.width, m.height = 180, 30
	showDiffFixture(m)
	m.split = true
	m.diffPath = "file.go"
	m.patch = diff.Parse("--- a/file.go\n+++ b/file.go\n@@ -9999,2 +9999,3 @@\n-var old = 1\n+var value = 2\n+// added\n same\n")
	m.renderDiff()
	m.diffSyntax = diffSpans(m.ctx, m.patch, m.diffPath, m.light)
	foundSyntax, foundPlaceholder := false, false
	for _, pair := range m.diffRows {
		if pair.Right == nil || pair.Left == nil {
			foundPlaceholder = true
			if !strings.Contains(plain(m.diffRow(nil, 40, false)), "╱") {
				t.Fatal("missing line is not hatched")
			}
		}
		for _, line := range []*diff.Line{pair.Left, pair.Right} {
			if line == nil || line.Kind != diff.Added && line.Kind != diff.Deleted {
				continue
			}
			rendered := m.diffRow(line, 40, line.Kind == diff.Added)
			if ansi.StringWidth(rendered) != 40 {
				t.Fatal("diff row width changed")
			}
			cells := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0]
			bg := theme.Dark().AddedBackground
			if line.Kind == diff.Deleted {
				bg = theme.Dark().DeletedBackground
			}
			for x, cell := range cells {
				want := bg
				changed := m.diffEmphasis[line]
				col := x - m.diffGutter
				if col >= changed.start && (col < changed.end || changed.end > utf8.RuneCountInString(line.Text[1:])) {
					want = theme.Dark().AddedText
					if line.Kind == diff.Deleted {
						want = theme.Dark().DeletedText
					}
				}
				if x < m.diffGutter {
					want = theme.Dark().Background
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
		if ansi.StringWidth(m.diffRow(&m.patch.Lines[3], width, false)) != width {
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
	if diffSpans(ctx, m.patch, "file.go", false) != nil {
		t.Fatal("cancelled work continued")
	}
}

func TestDiffSyntaxFontsKeepDiffBackground(t *testing.T) {
	line := diffCode(theme.Dark(), "bold", []highlight.Span{{Start: 0, End: 4, Color: "#569CD6", Font: highlight.Bold}}, 12, 0, theme.Dark().AddedBackground, theme.Dark().AddedText, diffRange{0, 5})
	cells := uv.NewStyledString(line).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range cells {
		if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Dark().AddedText)) {
			t.Fatal("TextMate font reset diff background")
		}
		if i < 4 && cell.Style.Attrs&uv.AttrBold == 0 {
			t.Fatal("diff lost bold syntax style")
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
