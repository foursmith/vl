package editor

import (
	"strings"
	"testing"
)

func TestVimVisualSelection(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		keys       []string
		selection  string
		cursor     int
		label      string
	}{
		{"single", "abc", []string{"v"}, "a", 0, "VISUAL"},
		{"forward", "abc", []string{"v", "l"}, "ab", 1, "VISUAL"},
		{"backward", "abcd", []string{"l", "l", "v", "h"}, "bc", 1, "VISUAL"},
		{"cross anchor", "abcd", []string{"l", "v", "l", "h", "h"}, "ab", 0, "VISUAL"},
		{"cross line", "ab\ncd", []string{"v", "j"}, "ab\nc", 3, "VISUAL"},
		{"line", "ab\ncd", []string{"l", "V"}, "ab\n", 1, "VISUAL LINE"},
		{"line backward", "ab\ncd\nef", []string{"j", "V", "k"}, "ab\ncd\n", 0, "VISUAL LINE"},
		{"count", "abcdef", []string{"v", "3", "l"}, "abcd", 3, "VISUAL"},
		{"end", "abcdef", []string{"v", "L"}, "abcdef", 5, "VISUAL"},
		{"last", "a\nb\nc", []string{"v", "G"}, "a\nb\nc", 4, "VISUAL"},
		{"first", "a\nb\nc", []string{"G", "v", "g", "g"}, "a\nb\nc", 0, "VISUAL"},
		{"switch line", "abc\ndef", []string{"v", "V"}, "abc\n", 0, "VISUAL LINE"},
		{"switch char", "abc\ndef", []string{"V", "v"}, "a", 0, "VISUAL"},
		{"swap", "abc", []string{"v", "l", "o"}, "ab", 0, "VISUAL"},
		{"unicode", "👩‍💻é中", []string{"v", "l"}, "👩‍💻é", 3, "VISUAL"},
		{"unicode backward", "👩‍💻é中", []string{"l", "v", "h"}, "👩‍💻é", 0, "VISUAL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.SelectedText() != tt.selection || d.Cursor != tt.cursor || v.Label() != tt.label {
				t.Fatalf("selection %q cursor %d label %s", d.SelectedText(), d.Cursor, v.Label())
			}
		})
	}
}
func TestVimVisualEdits(t *testing.T) {
	for _, tt := range []struct {
		name, text     string
		keys           []string
		want, register string
		linewise       bool
	}{
		{"delete single", "abc", []string{"v", "d"}, "bc", "a", false},
		{"delete reverse", "abcd", []string{"l", "l", "v", "h", "d"}, "ad", "bc", false},
		{"copy", "abcd", []string{"v", "l", "y"}, "abcd", "ab", false},
		{"change", "abcd", []string{"v", "l", "c", "X", "Y", "esc"}, "XYcd", "ab", false},
		{"delete lines", "a\nb\nc", []string{"V", "j", "d"}, "c", "a\nb\n", true},
		{"delete last line", "a\nb", []string{"j", "V", "d"}, "a", "b", true},
		{"copy lines", "a\nb\nc", []string{"j", "V", "k", "y"}, "a\nb\nc", "a\nb\n", true},
		{"change lines", "a\nb\nc", []string{"V", "j", "c", "X", "esc"}, "X\nc", "a\nb\n", true},
		{"delete emoji", "👩‍💻é中", []string{"v", "l", "d"}, "中", "👩‍💻é", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.clipboardText != clipboardExpected(tt.register, tt.linewise) || v.VisualMode || v.InsertMode || d.Anchor != -1 {
				t.Fatalf("text %q register %+v mode %s anchor %d", d.Text, v.clipboardText, v.Label(), d.Anchor)
			}
			if tt.want != tt.text {
				vimKeys(d, v, "u")
				if d.Text != tt.text || d.Anchor != -1 {
					t.Fatalf("undo: %q anchor %d", d.Text, d.Anchor)
				}
			}
		})
	}
}
func TestVimVisualExit(t *testing.T) {
	for _, keys := range [][]string{{"v", "l", "esc"}, {"v", "l", "v"}, {"V", "j", "V"}} {
		d := &Document{Text: "abc\ndef", Anchor: -1}
		v := &Vim{}
		vimKeys(d, v, keys...)
		if v.VisualMode || d.Anchor != -1 || d.SelectedText() != "" {
			t.Fatalf("%v: visual mode or selection left behind", keys)
		}
		d.SetPosition(0, 0)
		d.Move("right", true)
		if d.SelectedText() != "a" {
			t.Fatal("Visual semantics leaked to regular selection")
		}
	}
}

func clipboardExpected(text string, linewise bool) string {
	if linewise && !strings.HasSuffix(text, "\n") {
		return text + "\n"
	}
	return text
}
