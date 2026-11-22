package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestDiffExpandsOmittedContextWithoutChangingPatch(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 70})
	m.split = true
	ctx := t.Context()
	const path = "context.txt"
	lines := make([]string, 36)
	for i := range lines {
		lines[i] = fmt.Sprintf("original line %02d", i+1)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte(text+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(strings.Join(lines, "\n"))
	if _, err := m.repo.Run(ctx, "add", "--", path); err != nil {
		t.Fatal(err)
	}
	lines[9] = "first edit\nextra line"
	lines[24] = "second edit"
	write(strings.Join(lines, "\n"))
	change := gitservice.Change{Path: path, Worktree: 'M'}
	drain(t, m, m.requestDiff(diffTarget{path: path, change: &change}, func(ctx context.Context) (string, error) {
		return m.repo.Diff(ctx, change, false)
	}))
	m.focusEditor()
	if len(m.contextFolds) != 3 || !m.diffLayout.split {
		t.Fatalf("expected leading, middle and trailing folds in split view, got %+v", m.contextFolds)
	}
	hunkPatch := func() string {
		t.Helper()
		raw, err := m.patch.StagingPatch(path, false, func(line *diff.Line) bool { return line.Hunk == 1 })
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := hunkPatch()
	for _, pair := range m.diffRows {
		if pair.Left != nil && pair.Left.Kind == diff.HunkHeader {
			t.Fatal("Git hunk header is visible in collapsed diff")
		}
	}
	last := m.diffRows[len(m.diffRows)-1]
	foldRow := m.diffView()[len(m.diffRows)-1]
	if last.Hunk != -1 || last.Left == nil || last.Left.Kind != diff.Fold ||
		strings.Count(plain(foldRow), m.tr("%d hidden lines", 8)) != 1 {
		t.Fatal("expected a single trailing hidden line count")
	}
	if strings.Contains(plain(foldRow), "│") {
		t.Fatal("pane separator broke the fold row")
	}
	for _, cell := range uv.NewStyledString(foldRow).Lines(ansi.GraphemeWidth)[0] {
		if !sameColor(cell.Style.Bg, lipgloss.Color(m.colors.Panel)) {
			t.Fatal("fold background did not span both panes")
		}
	}
	drain(t, m, press(m, "enter"))
	if fold := m.contextFolds[0]; len(fold.lines) != 6 || !strings.Contains(m.review.Text, "original line 01") {
		t.Fatalf("Enter did not expand the leading context: %+v", fold)
	}
	drain(t, m, press(m, "enter"))
	if m.contextFolds[0].lines != nil {
		t.Fatal("Enter inside expanded context did not collapse it")
	}
	clickFold := func(index int) {
		t.Helper()
		x := m.mainX() + m.mainWidth()/2
		row := slices.IndexFunc(m.diffRows, func(pair diff.Pair) bool {
			return pair.Left != nil && pair.Left.Kind == diff.Fold && pair.Left.Hunk == index
		})
		if row < 0 {
			t.Fatalf("fold %d is missing", index)
		}
		_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: m.contentRow() + row - m.viewport.YOffset(), Button: tea.MouseLeft})
		drain(t, m, cmd)
	}
	clickFold(1)
	if fold := m.contextFolds[1]; len(fold.lines) != 8 || fold.lines[0].Old != 14 ||
		fold.lines[0].New != 15 || codeLine(&fold.lines[0]) != "original line 14" {
		t.Fatalf("unexpected middle context: %+v", fold)
	}
	collapseFromGutter := func(index int, right bool) {
		t.Helper()
		fold := &m.contextFolds[index]
		if slices.ContainsFunc(m.diffRows, func(pair diff.Pair) bool {
			return pair.Left != nil && pair.Left.Kind == diff.Fold && pair.Left.Hunk == index
		}) {
			t.Fatal("expanded fold retained its hidden line count")
		}
		row := fold.start - m.viewport.YOffset()
		lines := m.diffView()
		icon := m.symbol(symbolFold)
		expected := 1
		if m.diffLayout.split {
			expected = 2
		}
		if strings.Count(plain(lines[row]), icon) != expected || strings.Contains(plain(lines[row+1]), icon) {
			t.Fatal("expected a collapse icon in each pane on the first expanded row")
		}
		start, _ := m.diffPane(right)
		_, cmd := m.Update(tea.MouseClickMsg{X: m.mainX() + start, Y: m.contentRow() + row, Button: tea.MouseLeft})
		drain(t, m, cmd)
		if fold.lines != nil {
			t.Fatal("click on the first row's gutter did not collapse the context")
		}
	}
	collapseFromGutter(1, true)
	clickFold(2)
	collapseFromGutter(2, true)
	clickFold(1)
	collapseFromGutter(1, false)
	clickFold(1)
	m.split = false
	m.renderDiff()
	clickFold(2)
	if !strings.Contains(m.review.Text, "original line 36") || hunkPatch() != before {
		t.Fatal("tail context missing or original hunk patch was changed")
	}
	collapseFromGutter(1, false)
	if strings.Contains(m.review.Text, "original line 14") {
		t.Fatal("click did not collapse the middle fold")
	}
}
