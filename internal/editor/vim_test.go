package editor

import (
	"testing"
)

func vimKeys(d *Document, v *Vim, keys ...string) string {
	var copied VimRegisterValue
	for _, k := range keys {
		handled, text := v.Handle(d, k, copied)
		if text.Text != "" {
			copied = text
		}
		if !handled && v.InsertMode {
			d.Insert(k)
		}
	}
	return copied.Text
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
		{"delete to last line", "one\ntwo\nthree\n", []string{"d", "G"}, ""},
		{"delete to counted line", "one\ntwo\nthree", []string{"d", "2", "G"}, "three"},
		{"delete to first line", "one\ntwo\nthree", []string{"j", "d", "g", "g"}, "three"},
		{"change to last line", "one\ntwo\nthree\n", []string{"j", "c", "G", "X", "esc"}, "one\nX\n"},
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
	vimKeys(a, av, "l", "y")
	_, copied := av.Handle(a, "y", VimRegisterValue{})
	if a.Cursor != 1 || copied != (VimRegisterValue{Text: "one\n", Linewise: true}) {
		t.Fatal("line copy moved cursor or omitted register text")
	}
	b := &Document{Buffer: Buffer{Text: "other", Anchor: -1}}
	bv := &Vim{}
	bv.Handle(b, "p", copied)
	if b.Text != "other\none" {
		t.Fatal(b.Text)
	}
}

func TestVimYankToLastLine(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "one\ntwo\nthree\n", Anchor: -1}}
	v := &Vim{}
	vimKeys(d, v, "G")
	if row, _ := d.Position(); row != 2 {
		t.Fatalf("G moved to line %d, want the last text line", row+1)
	}
	vimKeys(d, v, "k", "l", "y")
	_, copied := v.Handle(d, "G", VimRegisterValue{})
	if copied != (VimRegisterValue{Text: "two\nthree\n", Linewise: true}) || d.Cursor != 5 {
		t.Fatalf("yG copied %+v and left cursor at %d", copied, d.Cursor)
	}
}

func TestVimPutRegisterValues(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		keys       []string
		value      VimRegisterValue
	}{
		{"characterwise with newline", "oX\nne\ntwo", []string{"p"}, VimRegisterValue{Text: "X\n"}},
		{"replace a line with characters", "X\ntwo", []string{"V", "p"}, VimRegisterValue{Text: "X"}},
		{"replace the unterminated last line", "one\nX", []string{"j", "V", "p"}, VimRegisterValue{Text: "X\n", Linewise: true}},
		{"counted characterwise put", "oXXXne\ntwo", []string{"3", "p"}, VimRegisterValue{Text: "X"}},
		{"counted linewise put", "one\nX\nX\ntwo", []string{"2", "p"}, VimRegisterValue{Text: "X\n", Linewise: true}},
		{"repeat a counted put", "oXXXXXXne\ntwo", []string{"3", "p", "."}, VimRegisterValue{Text: "X"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: "one\ntwo", Anchor: -1}}
			v := &Vim{}
			for _, key := range tt.keys {
				v.Handle(d, key, tt.value)
			}
			if d.Text != tt.want {
				t.Fatalf("put = %q, want %q", d.Text, tt.want)
			}
			if tt.keys[len(tt.keys)-1] == "p" {
				vimKeys(d, v, "u")
				if d.Text != "one\ntwo" {
					t.Fatalf("undo put = %q", d.Text)
				}
			}
		})
	}
}
