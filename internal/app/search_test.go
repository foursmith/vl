package app

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
)

func TestSearchRanges(t *testing.T) {
	for _, tt := range []struct {
		text, query string
		want        [][2]int
	}{
		{"one one", "one", [][2]int{{0, 3}, {4, 7}}},
		{"中🙂中🙂", "🙂", [][2]int{{1, 2}, {3, 4}}},
		{"aaaa", "aa", [][2]int{{0, 2}, {2, 4}}},
		{"word", "", nil}, {"word", "Word", nil},
	} {
		if got := searchRanges(tt.text, tt.query); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%q / %q: %v", tt.text, tt.query, got)
		}
	}
}

func TestSearchHighlightsAllAndClearsUntilNext(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "one one\none one", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false
	m.vimEnabled = true
	m.find = "one"
	m.findNext()
	check := func(want int) {
		t.Helper()
		count := 0
		for _, row := range m.editorLines()[:2] {
			for _, cell := range uv.NewStyledString(row).Lines(ansi.GraphemeWidth)[0] {
				if sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatch)) || sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatchHighlight)) {
					count++
				}
			}
		}
		if count != want {
			t.Fatalf("highlighted cells = %d, want %d", count, want)
		}
	}
	check(12)
	press(m, " ")
	press(m, "enter")
	check(0)
	press(m, "n")
	check(12)
	// Query and text changes must invalidate the cached rows too.
	m.find = "two"
	check(0)
	d.Text = strings.ReplaceAll(d.Text, "one", "two")
	check(12)
}
