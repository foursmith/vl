package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
)

func TestNewFileDiffUsesCompactContextAndContent(t *testing.T) {
	m := tabWorkspace(t, 0)
	showDiffFixture(m)
	m.diffPath = "go.mod"
	m.patch = diff.Parse("diff --git a/go.mod b/go.mod\nnew file mode 100644\nindex 000..111\n--- /dev/null\n+++ b/go.mod\n@@ -0,0 +1,2 @@\n+module example\n+go 1.26.7\n")
	m.renderDiff()
	text := plain(strings.Join(m.diffView(), "\n"))
	if strings.Contains(text, "new file mode") || strings.Contains(text, "@@") || strings.Contains(text, "diff --git") || !strings.Contains(text, "module example") {
		t.Fatalf("raw headers leaked into new-file content: %s", text)
	}
	toolbar := plain(m.contentToolbar())
	if !strings.Contains(toolbar, "New file") || !strings.Contains(toolbar, "+2") || strings.Contains(toolbar, "−0") || !strings.Contains(toolbar, "Working Tree") {
		t.Fatalf("toolbar: %s", toolbar)
	}
	if m.tabLabel(m.activeTab) != "go.mod · Diff" {
		t.Fatal("diff tab does not distinguish itself from the file tab")
	}
}

func TestCompactHunkRangesRetainNavigationAndModeMetadata(t *testing.T) {
	m := tabWorkspace(t, 0)
	showDiffFixture(m)
	m.diffPath = "file.go"
	m.patch = diff.Parse("diff --git a/file.go b/file.go\nold mode 100644\nnew mode 100755\n--- a/file.go\n+++ b/file.go\n@@ -1,2 +1,2 @@\n-old\n+new\n same\n")
	m.renderDiff()
	text := plain(strings.Join(m.diffView(), "\n"))
	if !strings.Contains(text, "1–2 → 1–2") || strings.Contains(text, "old mode") || strings.Contains(text, "@@") {
		t.Fatalf("hunk presentation: %s", text)
	}
	if !strings.Contains(plain(m.contentToolbar()), "100644 → 100755") {
		t.Fatal("mode change information was lost")
	}
	m.nextHunk(1)
	if m.diffHunk != 0 || len(m.diffRowHunks) == 0 {
		t.Fatal("hunk navigation lost row identity")
	}
}

func TestDiffToolbarControlAndWidth(t *testing.T) {
	m := tabWorkspace(t, 0)
	showDiffFixture(m)
	m.diffPath = "nested/path/file.go"
	for _, width := range []int{30, 70, 120, 180} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		if ansi.StringWidth(m.contentToolbar()) != m.mainWidth() {
			t.Fatalf("toolbar width at %d", width)
		}
	}
	button := m.diffModeButton()
	before := m.split
	m.click(tea.MouseClickMsg{X: m.mainX() + button.x + 1, Y: editorContentRow - 1, Button: tea.MouseLeft})
	if m.split == before {
		t.Fatal("diff layout button did not toggle mode")
	}
}

func TestLargeDiffPaintsOnlyViewportAndKeepsHunkSelection(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.patch = benchmarkPatch()
	m.diffPath = "file.go"
	showDiffFixture(m)
	m.renderDiff()
	if m.viewport.TotalLineCount() != len(m.diffRows) {
		t.Fatal("viewport geometry does not cover the complete diff")
	}
	if len(m.diffView()) > m.contentHeight() {
		t.Fatal("diff rendered beyond its viewport")
	}
	m.nextHunk(0) // The read-only cursor initially selects the first hunk.
	if m.diffHunk != 0 || !strings.Contains(plain(strings.Join(m.diffView(), "\n")), m.symbol(symbolRight)) {
		t.Fatal("current hunk marker missing after navigation")
	}
	m.nextHunk(1)
	if m.diffHunk != 1 {
		t.Fatal("hunk navigation lost index")
	}
	setTestFind(m, "new value 19")
	m.findNext()
	if !strings.Contains(plain(strings.Join(m.diffView(), "\n")), "new value 19") {
		t.Fatal("source search no longer reveals the matching Diff row")
	}
	m.closeTab()
	if len(m.diffRows) != 0 || m.diffLayout.raw != "" {
		t.Fatal("closed diff retained a large layout")
	}
}
