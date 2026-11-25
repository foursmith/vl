package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func clickText(t *testing.T, m *Model, x, y int) {
	t.Helper()
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func TestEditorMouseSelectsWordThenLine(t *testing.T) {
	const line = "hi 👩‍💻 éclair.word"
	for _, disabled := range []bool{false, true} {
		m := flashModel(t, line+"\nnext line")
		m.setVimDisabled(disabled)
		d := m.document()
		x, y := m.mainX()+editorGutterWidth+ansi.StringWidth("hi 👩‍💻 ")+1, m.contentRow()
		clickText(t, m, x, y)
		if d.SelectedText() != "" {
			t.Fatal("single click selected text")
		}
		clickText(t, m, x, y)
		if got := d.SelectedText(); got != "éclair" {
			t.Fatalf("double click selected %q", got)
		}
		clickText(t, m, x, y)
		if got := d.SelectedText(); got != line+"\n" {
			t.Fatalf("triple click selected %q", got)
		}
		m.textClick.at = time.Now().Add(-time.Second)
		clickText(t, m, x, y)
		if d.SelectedText() != "" {
			t.Fatal("click after timeout extended the selection")
		}

		start := m.mainX() + editorGutterWidth
		m.Update(tea.MouseClickMsg{X: start, Y: y, Button: tea.MouseLeft})
		m.Update(tea.MouseMotionMsg{X: start + 2, Y: y, Button: tea.MouseLeft})
		m.Update(tea.MouseReleaseMsg{X: start + 2, Y: y, Button: tea.MouseLeft})
		if d.SelectedText() != "hi" {
			t.Fatal("single-click drag stopped selecting text")
		}
	}
}

func TestDiffMouseSelectsWordThenLine(t *testing.T) {
	m := reviewWorkspace(t, true)
	for _, right := range []bool{false, true} {
		pane, _ := m.diffPane(right)
		line := "old value"
		if right {
			line = "new value"
		}
		for row := range m.diffRows {
			if codeLine(m.selectionLine(row, right)) != line {
				continue
			}
			x, y := m.mainX()+pane+m.diffGutter+1, m.contentRow()+row
			clickText(t, m, x, y)
			clickText(t, m, x, y)
			if got := m.diffSelectedText(); got != line[:3] {
				t.Fatalf("double click selected %q on right=%v", got, right)
			}
			clickText(t, m, x, y)
			if got := m.diffSelectedText(); got != line {
				t.Fatalf("triple click selected %q on right=%v", got, right)
			}
			break
		}
	}
}

func TestInputMouseSelection(t *testing.T) {
	const value = "猫 éclair.word"
	for _, name := range []string{"picker", "dialog", "commit", "query", "replacement", "include", "exclude", "find", "file replacement"} {
		t.Run(name, func(t *testing.T) {
			m := flashModel(t, "example")
			input := &m.input
			x, y := 0, 0
			switch name {
			case "picker":
				m.prompt("quick", "Go to File", "")
				l := m.pickerLayout(m.filteredActions())
				x, y = l.x+2, l.y+1
			case "dialog", "commit":
				kind := "newFile"
				if name == "commit" {
					kind = name
				}
				m.prompt(kind, "Prompt", "")
				l := m.dialogLayout()
				x, y = l.x+2, l.y+2
				if name == "commit" {
					x, y = l.x+4, l.y+4
				}
			case "find", "file replacement":
				m.openFileReplace()
				focus, row := 0, fileFindQueryRow
				if name == "file replacement" {
					focus, row = 1, fileReplaceRow
				}
				input = m.searchField(focus)
				x, y = m.fileFindX()+searchFieldInset, m.contentRow()+row
			default:
				m.sidebarVisible = true
				m.SetSidebarLayout(45, false)
				m.openSearch()
				m.search.replace, m.search.details = true, true
				l := m.searchPanelLayout()
				focus, row := 0, l.query
				switch name {
				case "replacement":
					focus, row = 1, l.replacement
				case "include":
					focus, row = 3, l.include
				case "exclude":
					focus, row = 4, l.exclude
				}
				input = m.searchField(focus)
				x, y = m.sideX()+searchFieldInset, row
			}
			input.SetValue(value)
			x += ansi.StringWidth(input.Prompt + "猫 ")
			clickText(t, m, x, y)
			if input.Position() != 2 {
				t.Fatalf("single click positioned caret at %d", input.Position())
			}
			clickText(t, m, x, y)
			if start, end := input.buffer.Selection(); start != 2 || end != 9 {
				t.Fatalf("double click selected [%d, %d)", start, end)
			}
			clickText(t, m, x, y)
			if start, end := input.buffer.Selection(); start != 0 || end != 14 {
				t.Fatalf("triple click selected [%d, %d)", start, end)
			}
			m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			m.Update(tea.MouseMotionMsg{X: x - 3, Y: y, Button: tea.MouseLeft})
			if got := input.buffer.SelectedText(); got != "猫 " || m.hoverOnly {
				t.Fatalf("drag left selected %q with hover-only rendering %v", got, m.hoverOnly)
			}
			m.Update(tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
			if got := input.buffer.SelectedText(); got != "écl" || m.inputDrag != nil {
				t.Fatalf("release selected %q with drag still active %v", got, m.inputDrag != nil)
			}
			press(m, "x")
			if got := input.Value(); got != "猫 xair.word" {
				t.Fatalf("typing over the mouse selection produced %q", got)
			}
		})
	}
}

func TestMouseSelectionUsesHorizontallyScrolledInput(t *testing.T) {
	m := flashModel(t, "")
	m.prompt("newFile", "Prompt", "")
	area := m.dialogLayout().input
	prefix := strings.Repeat("x", area.width) + " "
	m.input.SetValue(prefix + "second")
	x := area.x + ansi.StringWidth(m.input.Prompt) + area.width - 3
	clickText(t, m, x, area.y)
	clickText(t, m, x, area.y)
	if start, end := m.input.buffer.Selection(); start != len(prefix) || end != len(prefix)+len("second") {
		t.Fatalf("double-click in scrolled input selected [%d, %d)", start, end)
	}
	left, _ := m.input.viewport(area.width)
	x = area.x + ansi.StringWidth(m.input.Prompt)
	m.Update(tea.MouseClickMsg{X: x, Y: area.y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x - 2, Y: area.y, Button: tea.MouseLeft})
	if start, end := m.input.buffer.Selection(); start != left-2 || end != left {
		t.Fatalf("drag beyond the left edge selected [%d, %d) from viewport offset %d", start, end, left)
	}
}
