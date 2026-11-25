package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
)

func flashModel(t *testing.T, text string) *Model {
	t.Helper()
	m := newHoverRenderModel(t)
	m.tabs = []tab{{doc: &editor.Document{Buffer: editor.Buffer{Text: text, Anchor: -1}}}}
	m.activeTab = 0
	m.sidebar = false
	m.sidebarVisible = false

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	return m
}
func flashQuery(m *Model, text string) {
	press(m, "f")
	m.Update(tea.PasteMsg{Content: text})
}
func TestFlashIncrementalSearchAndJump(t *testing.T) {
	m := flashModel(t, "far foo fop\nfoo end")
	original := m.document().Text
	press(m, "f")
	press(m, "f")
	if m.flash == nil || m.flash.query != "f" {
		t.Fatal("Flash did not start")
	}
	for _, label := range m.flash.labels {
		if label == 'a' || label == 'o' {
			t.Fatal("label intercepted query continuation")
		}
	}
	press(m, "o")
	press(m, "o")
	if m.flash.query != "foo" || len(m.flash.targets) != 2 {
		t.Fatal("query was mistaken for a label")
	}
	target := m.flash.targets[1]
	if target.label == "" {
		t.Fatal("missing target label")
	}
	before := m.View().Content
	press(m, target.label)
	if m.flash != nil || m.document().Cursor != target.offset || m.document().Text != original || m.sidebar {
		t.Fatal("label jump changed text or focus")
	}
	if before == m.View().Content || strings.Contains(m.statusLine(), "FLASH") {
		t.Fatal("jump retained overlay")
	}
}
func TestFlashCancelVisualAndInsert(t *testing.T) {
	m := flashModel(t, "one two three")
	press(m, "v")
	press(m, "l")
	anchor, cursor := m.document().Anchor, m.document().Cursor
	flashQuery(m, "three")
	press(m, "ctrl+[")
	if m.flash != nil || m.document().Anchor != anchor || m.document().Cursor != cursor || !m.tabs[0].vim.VisualMode {
		t.Fatal("cancel changed Visual selection")
	}
	flashQuery(m, "three")
	target := m.flash.targets[0]
	press(m, target.label)
	if !m.tabs[0].vim.VisualMode || m.document().SelectedText() != "one two t" {
		t.Fatal("Visual jump did not extend selection:", m.document().SelectedText())
	}
	press(m, "esc")
	press(m, "i")
	press(m, "s")
	if m.flash != nil || !strings.Contains(m.document().Text, "s") {
		t.Fatal("Insert s opened Flash")
	}
}
func TestFlashUnicodeAndRendering(t *testing.T) {
	m := flashModel(t, "中 é 👩‍💻 中 é 👩‍💻")
	for _, query := range []string{"中", "é", "👩‍💻"} {
		flashQuery(m, query)
		if len(m.flash.targets) != 2 {
			t.Fatalf("%q: %d matches", query, len(m.flash.targets))
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) != m.width {
				t.Fatal("label changed terminal width")
			}
		}
		press(m, "esc")
	}
	flashQuery(m, "e")
	if len(m.flash.targets) != 0 {
		t.Fatal("match split a combining cluster")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.flash.query != "" {
		t.Fatal("backspace did not clear query")
	}
	m.Update(tea.PasteMsg{Content: "👩‍💻"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.flash.query != "" {
		t.Fatal("backspace split emoji")
	}
}
func TestFlashPagesAndExplicitLabels(t *testing.T) {
	m := flashModel(t, strings.Repeat("x ", 80))
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 18})
	flashQuery(m, "x")
	if len(m.flash.targets) <= len(m.flash.labels) {
		t.Fatal("fixture did not require paging")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.flash.page != 1 {
		t.Fatal("Tab did not page labels")
	}
	target := m.flash.targets[len(m.flash.labels)]
	press(m, target.label)
	if m.document().Cursor != target.offset {
		t.Fatal("paged label jumped incorrectly")
	}
	var lines []string
	for _, label := range flashLabels {
		lines = append(lines, "x"+string(label)+" ")
	}
	m = flashModel(t, strings.Join(lines, ""))
	m.Update(tea.WindowSizeMsg{Width: 250, Height: 18})
	flashQuery(m, "x")
	if m.flash.labels != "" {
		t.Fatal("unsafe labels were not excluded")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.flash.explicit || m.flash.labels == "" {
		t.Fatal("explicit labels unavailable")
	}
	target = m.flash.targets[0]
	press(m, target.label)
	if m.flash != nil || m.document().Cursor != target.offset {
		t.Fatal("explicit label failed")
	}
}
func TestFlashOnlyVisibleContentAndInvalidation(t *testing.T) {
	m := flashModel(t, "👩‍💻 x target\n"+strings.Repeat("hidden\n", 40))
	m.tabs[0].left = 1
	flashQuery(m, "👩‍💻")
	if len(m.flash.targets) != 0 {
		t.Fatal("partially clipped target was labeled")
	}
	press(m, "esc")
	m.tabs[0].left = 0
	flashQuery(m, "hidden")
	for _, target := range m.flash.targets {
		row, _ := positionAtDoc(m.document(), target.offset)
		if row >= m.contentHeight() {
			t.Fatal("offscreen target labeled")
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 18})
	if m.flash != nil {
		t.Fatal("resize retained stale labels")
	}
	flashQuery(m, "target")
	m.document().Insert("changed")
	press(m, "a")
	if m.flash != nil {
		t.Fatal("document mutation retained labels")
	}
	flashQuery(m, "target")
	press(m, "ctrl+g")
	if m.flash != nil || m.modal != "quick" {
		t.Fatal("shortcut did not leave Flash")
	}
	press(m, "esc")
	flashQuery(m, "target")
	m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelDown})
	if m.flash != nil {
		t.Fatal("scroll retained stale labels")
	}
}
func positionAtDoc(d *editor.Document, offset int) (int, int) {
	old := d.Cursor
	d.Cursor = offset
	row, col := d.Position()
	d.Cursor = old
	return row, col
}

func TestFlashSingleEntryAndOperators(t *testing.T) {
	for _, operator := range []string{"d", "c", "y"} {
		m := flashModel(t, "a,b,c")
		press(m, operator)
		press(m, "f")
		if m.flash == nil || m.flash.operator != operator {
			t.Fatal("pending operator did not open Flash")
		}
		press(m, ",")
		cmd := m.flashKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil || m.vimRegister != "a," {
			t.Fatalf("%s produced a command or wrong register text: %q", operator, m.vimRegister)
		}
		switch operator {
		case "d":
			if m.document().Text != "b,c" {
				t.Fatal(m.document().Text)
			}
		case "c":
			press(m, "X")
			press(m, "esc")
			if m.document().Text != "Xb,c" {
				t.Fatal(m.document().Text)
			}
		case "y":
			if m.document().Text != "a,b,c" {
				t.Fatal("copy edited text")
			}
		}
	}
	m := flashModel(t, "a,b,c")
	press(m, "d")
	press(m, "f")
	press(m, "esc")
	press(m, "w")
	if m.document().Text != "a,b,c" || m.flash != nil {
		t.Fatal("cancelled Flash operator edited text")
	}
}
func TestFlashRemovedAliases(t *testing.T) {
	for _, key := range []string{"F", "t", "T", "s"} {
		for _, prefix := range []string{"", "v", "d", "c", "y"} {
			t.Run(prefix+key, func(t *testing.T) {
				m := flashModel(t, "alpha beta")
				if prefix != "" {
					press(m, prefix)
				}
				press(m, key)
				want := "alpha beta"
				if prefix == "" && key == "s" {
					want = "lpha beta"
				}
				if m.flash != nil || m.document().Text != want {
					t.Fatal("removed alias is active")
				}
				// Leave Insert or a pending Surround command before opening Flash.
				if key == "s" && prefix != "v" {
					press(m, "esc")
				}
				press(m, "f")
				if m.flash == nil || m.flash.operator != "" {
					t.Fatal("f failed or removed alias retained operator")
				}
			})
		}
	}
}
