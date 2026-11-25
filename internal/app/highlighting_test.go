package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/highlight"
)

func TestPendingHighlightPreservesEditedRowColors(t *testing.T) {
	const header = "package sample\n"
	for _, tc := range []struct{ name, before, after string }{
		{"insert", "var 名 = 42\n", "var 名字 = 42\n"},
		{"split", "var 名 = 42\n", "var 名 =\n  42\n"},
		{"join", "var 名 =\n  42\n", "var 名 = 42\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			m.openFile("file0.go")
			d := m.document()
			d.Edit(header + tc.before)
			drain(t, m, m.scheduleHighlight())
			colorAt := func(rows []string, token string) string {
				t.Helper()
				for _, row := range rows {
					text := ansi.Strip(row)
					if start := strings.Index(text, token); start >= 0 {
						column := ansi.StringWidth(text[:start])
						cell := uv.NewStyledString(ansi.Cut(row, column, column+1)).Lines(ansi.GraphemeWidth)[0][0]
						if cell.Style.Fg != nil {
							r, g, b, _ := cell.Style.Fg.RGBA()
							return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
						}
					}
				}
				t.Fatalf("no color found for %q", token)
				return ""
			}
			before := m.editorLines()
			d.Edit(header + tc.after)
			pending := m.editorLines()
			for _, token := range []string{"var", "42"} {
				want := colorAt(before, token)
				if want == m.colors.Foreground {
					t.Fatalf("fixture token %q did not start highlighted", token)
				}
				if got := colorAt(pending, token); got != want {
					t.Fatalf("pending color for %q = %s, want %s", token, got, want)
				}
			}
			drain(t, m, m.scheduleHighlight())
			if m.tabs[0].highlightText != d.Text {
				t.Fatal("fresh highlight did not replace the fallback")
			}
		})
	}
}

func TestHighlightSessionSurvivesEditsAndUndo(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	complete := func() {
		t.Helper()
		request := m.scheduleHighlight()().(highlightStartMsg).request
		m.finishHighlight(m.startHighlight(request)().(highlightResultMsg))
		want := new(highlight.Session).Lines(m.ctx, highlight.Language(m.document().Path), request.text, request.theme)
		for _, row := range request.rows {
			if !reflect.DeepEqual(m.tabs[0].spans[row], want[row]) {
				t.Fatalf("incremental app result differs from fresh highlight at row %d", row)
			}
		}
	}
	complete()
	session := m.tabs[0].highlighter
	m.document().Insert("/*\n")
	complete()
	m.document().Undo()
	complete()
	if m.tabs[0].highlighter != session {
		t.Fatal("editing replaced the incremental session")
	}
}

func TestEditorHighlightFollowsViewportAndKeepsIncomingState(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	_, resize := m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	drain(t, m, resize)
	d := m.document()
	d.Edit("package sample\n/*\n" + strings.Repeat("var n = 1\n", 100) + "*/\nvar n = 2\n")
	drain(t, m, m.scheduleHighlight())
	tab := &m.tabs[0]
	if len(tab.spans) > 2*m.contentHeight() || len(tab.spans[2]) == 0 {
		t.Fatalf("initial highlight should cover only the area around the viewport, got %d rows", len(tab.spans))
	}
	const target = 75
	tab.top = target
	drain(t, m, m.scheduleHighlight())
	want := new(highlight.Session).Lines(m.ctx, tab.documentLanguage(), d.Text, m.colorTheme)
	if got := tab.spans[target]; len(got) == 0 || !reflect.DeepEqual(got, want[target]) {
		t.Fatalf("highlight for a scrolled-in line inside a comment = %#v, want %#v", got, want[target])
	}
	if _, cached := tab.spans[2]; cached {
		t.Fatal("highlight cache retained the old viewport")
	}
}

func TestEditorHighlightSizeLimit(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	d := m.document()
	const prefix, padding = "package sample\n", "// padding in the rest of the file\n"
	const limit = 2048 << 10
	count := (limit - len(prefix) - 2) / len(padding)
	text := prefix + strings.Repeat(padding, count) + "//" +
		strings.Repeat("x", limit-len(prefix)-len(padding)*count-2)
	for _, size := range []int{limit, limit + 1, limit} {
		d.Edit((text + "x")[:size])
		drain(t, m, m.scheduleHighlight())
		if colored := len(m.tabs[0].spans[0]) != 0; colored != (size == limit) {
			t.Fatalf("first line highlight at %d bytes = %t", size, colored)
		}
	}
}

func TestHighlightingRejectsCancelledAndStaleText(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	first := m.scheduleHighlight()
	if first == nil || m.tabs[0].highlightJob.ctx == nil {
		t.Fatal("highlight work was not deferred")
	}
	old := first().(highlightStartMsg).request
	m.document().Insert("// new\n")
	next := m.scheduleHighlight()
	if old.ctx.Err() == nil {
		t.Fatal("new text did not cancel old tokenization")
	}
	stale := map[int][]highlight.Span{0: {{Start: 0, End: 1, Color: "#FF0000"}}}
	m.finishHighlight(highlightResultMsg{old, stale})
	if m.tabs[0].highlightText == old.text {
		t.Fatal("stale highlight replaced current syntax")
	}
	request := next().(highlightStartMsg).request
	work := m.startHighlight(request)
	if work == nil {
		t.Fatal("latest highlight request was rejected")
	}
	m.tabs[0].top = 1
	result := work().(highlightResultMsg)
	m.finishHighlight(result)
	if m.tabs[0].highlightText != m.document().Text || len(m.tabs[0].spans) == 0 {
		t.Fatal("latest syntax did not reach the document")
	}
	if m.tabs[0].top != 1 {
		t.Fatal("syntax completion moved scroll position")
	}
}

func TestClosingTabCancelsHighlightJob(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	request := m.scheduleHighlight()().(highlightStartMsg).request
	m.removeTab(request.doc)
	if request.ctx.Err() == nil || m.startHighlight(request) != nil {
		t.Fatal("closed tab retained background tokenization")
	}
	m.finishHighlight(highlightResultMsg{request, nil})
	if len(m.tabs) != 0 {
		t.Fatal("background result reopened a closed tab")
	}
}

func TestInsertRefreshesHighlightBeforeReturningToNormal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	source := "# Heading\n\n```go\nvar x = 1\n```\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	d := m.document()
	d.SetPosition(1, 0)
	m.ensureEditor()
	drain(t, m, m.scheduleHighlight())
	// This comparison covers token styling, independent of the moving line border.
	m.colors.CurrentLineBorder = ""
	editorText := func(line int) string {
		return ansi.Cut(m.editorLines()[line], 7, m.mainWidth())
	}
	heading, code := editorText(0), editorText(3)
	if len(m.tabs[0].spans[0]) == 0 || len(m.tabs[0].spans[3]) == 0 {
		t.Fatal("fixture did not start with highlighting")
	}

	drain(t, m, press(m, "i"))
	_, cmd := m.Update(tea.PasteMsg{Content: "## New heading\n"})
	if tab := &m.tabs[0]; !tab.vim.InsertMode || tab.highlightJob.ctx == nil || tab.highlightText != source {
		t.Fatal("inserting text did not schedule a background highlight pass")
	}
	if got := editorText(0); got != heading {
		t.Fatalf("highlight above edit changed: %q, want %q", got, heading)
	}
	if got := editorText(4); got != code {
		t.Fatalf("highlight below inserted line did not follow it: %q, want %q", got, code)
	}

	drain(t, m, cmd)
	if tab := &m.tabs[0]; !tab.vim.InsertMode || tab.highlightText != d.Text || len(tab.spans[1]) == 0 {
		t.Fatal("the new heading was not highlighted in Insert mode")
	}
}
