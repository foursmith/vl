package editor

import (
	"testing"
)

func vimKeys(d *Document, v *Vim, keys ...string) string {
	var copied string
	for _, k := range keys {
		handled, text := v.Handle(d, k, copied)
		if text != "" {
			copied = text
		}
		if !handled && v.InsertMode {
			d.Insert(k)
		}
	}
	return copied
}
func TestVimBasicEdits(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		keys       []string
		want       string
	}{
		{"delete words", "one two three four", []string{"2", "d", "w"}, "three four"},
		{"multiply counts", "a b c d e f g", []string{"2", "d", "3", "w"}, "g"},
		{"delete punctuation", "one.two", []string{"d", "w"}, ".two"},
		{"delete mixed-script word", "foo_中文bar", []string{"w", "d", "w"}, "foo_bar"},
		{"word end preserves final newline", "中文  \n", []string{"l", "d", "e"}, "中\n"},
		{"delete line end", "one\ntwo", []string{"d", "L"}, "\ntwo"},
		{"word at line end", "one\n  two", []string{"d", "w"}, "\n  two"},
		{"change word", "one two", []string{"c", "e", "X", "esc"}, "X two"},
		{"change single letter", "a two", []string{"c", "l", "X", "esc"}, "X two"},
		{"change counted words", "one two c", []string{"c", "2", "e", "X", "esc"}, "X c"},
		{"change empty final line", "one\n", []string{"j", "c", "c", "X", "esc"}, "one\nX"},
		{"change line", "one\ntwo", []string{"c", "c", "X", "esc"}, "X\ntwo"},
		{"delete final line", "one\ntwo", []string{"j", "d", "d"}, "one"},
		{"delete terminated line", "one\ntwo\n", []string{"j", "d", "d"}, "one\n"},
		{"delete final blank", "one\n", []string{"j", "d", "d"}, "one"},
		{"line paste below", "one\ntwo", []string{"y", "y", "p"}, "one\none\ntwo"},
		{"line paste at eof", "one", []string{"y", "y", "p"}, "one\none"},
		{"line paste above", "one", []string{"y", "y", "P"}, "one\none"},
		{"move line with ddp", "one\ntwo", []string{"d", "d", "p"}, "two\none"},
		{"character paste", "one two", []string{"y", "w", "p"}, "oone ne two"},
		{"open below", "one\ntwo", []string{"o", "X", "esc"}, "one\nX\ntwo"},
		{"open above", "one", []string{"O", "X", "esc"}, "X\none"},
		{"append", "one", []string{"A", "!", "esc"}, "one!"},
		{"normal ignores typing", "one", []string{"z", "enter", "delete"}, "one"},
		{"emoji delete", "👩‍💻é中", []string{"x"}, "é中"},
		{"combining delete", "é中", []string{"x"}, "中"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want {
				t.Fatalf("got %q want %q", d.Text, tt.want)
			}
			if v.InsertMode {
				t.Fatal("still inserting")
			}
		})
	}
}
func TestVimMotions(t *testing.T) {
	for _, tt := range []struct {
		keys     []string
		row, col int
	}{
		{[]string{"l"}, 0, 1}, {[]string{"L"}, 0, 10}, {[]string{"^"}, 0, 2},
		{[]string{"w"}, 0, 2}, {[]string{"w", "e"}, 0, 4}, {[]string{"w", "w", "b"}, 0, 2},
		{[]string{"G"}, 2, 0}, {[]string{"2", "G"}, 1, 0}, {[]string{"G", "g", "g"}, 0, 0},
		{[]string{"9", "l"}, 0, 9},
	} {
		d := &Document{Buffer: Buffer{Text: "  one two!!\nshort\nlast", Anchor: -1}}
		v := &Vim{}
		vimKeys(d, v, tt.keys...)
		row, col := d.Position()
		if row != tt.row || col != tt.col {
			t.Errorf("%v: got %d,%d want %d,%d", tt.keys, row, col, tt.row, tt.col)
		}
	}
}

func TestVimWordMotionMatchesDefaultVim(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		keys       []string
		want       []int
	}{
		{
			"Han next to identifiers", "foo_中文bar，done",
			[]string{"w", "b", "e", "w", "e", "w", "b", "w", "w", "w"},
			[]int{4, 0, 3, 4, 5, 6, 4, 6, 9, 10},
		},
		{
			"mixed scripts and punctuation", "foo中文かなカナ한글。🙂！ＡＢ_吧〇々𠀀文𰀀foo",
			[]string{"w", "w", "w", "w", "w", "w", "w", "w", "w", "w", "w", "w"},
			[]int{3, 5, 7, 9, 11, 12, 13, 14, 17, 18, 20, 22},
		},
		{
			"empty versus whitespace-only lines", "中文\n\n   \n  测试\n",
			[]string{"w", "w", "w", "b", "b", "e"},
			[]int{3, 10, 11, 10, 3, 11},
		},
		{
			"terminating newline", "中文\n",
			[]string{"w", "w", "e", "b"},
			[]int{1, 1, 1, 0},
		},
		{
			"empty last line", "中文\n\n",
			[]string{"2", "w", "b"},
			[]int{0, 3, 0},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
			v := &Vim{}
			for i, key := range tt.keys {
				vimKeys(d, v, key)
				if d.Cursor != tt.want[i] {
					t.Fatalf("after %v: cursor %d, want %d", tt.keys[:i+1], d.Cursor, tt.want[i])
				}
			}
		})
	}
}

func TestVimUndoGroups(t *testing.T) {
	for _, keys := range [][]string{{"i", "a", "b", "esc"}, {"c", "e", "a", "b", "esc"}, {"o", "a", "b", "esc"}, {"d", "d"}, {"x"}} {
		d := &Document{Buffer: Buffer{Text: "one two\nthree", Anchor: -1}}
		v := &Vim{}
		vimKeys(d, v, keys...)
		changed := d.Text
		vimKeys(d, v, "u")
		if d.Text != "one two\nthree" || d.Cursor != 0 || d.Anchor != -1 {
			t.Fatalf("%v undo: %q cursor %d anchor %d", keys, d.Text, d.Cursor, d.Anchor)
		}
		vimKeys(d, v, "U")
		if d.Text != changed {
			t.Fatalf("%v redo: %q != %q", keys, d.Text, changed)
		}
	}
}
func TestVimResetAndIndependentSessions(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "one", Anchor: -1}}
	v := &Vim{}
	vimKeys(d, v, "i", "A", "esc", "i", "B", "esc", "u")
	if d.Text != "Aone" {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "i", "C")
	v.Reset(d)
	d.Insert("D")
	d.Undo()
	if d.Text != "CAone" {
		t.Fatal("reset leaked edit group:", d.Text)
	}
}

func TestVimRegisterAcrossEditors(t *testing.T) {
	a := &Document{Buffer: Buffer{Text: "one\ntwo", Anchor: -1}}
	av := &Vim{}
	text := vimKeys(a, av, "l", "y", "y")
	if a.Cursor != 1 || text != "one\n" {
		t.Fatal("line copy moved cursor or omitted register text")
	}
	b := &Document{Buffer: Buffer{Text: "other", Anchor: -1}}
	bv := &Vim{}
	bv.Handle(b, "p", text)
	if b.Text != "other\none" {
		t.Fatal(b.Text)
	}
}
