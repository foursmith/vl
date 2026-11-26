package editor

import "testing"

func TestVimJoinLines(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		keys               []string
		row, col           int
	}{
		{"next line", "one\n \ttwo\nthree\n", "one two\nthree\n", []string{"J"}, 0, 3},
		{"count and Unicode", "猫\n  犬\n\t鳥\n", "猫 犬 鳥\n", []string{"3", "J"}, 0, 3},
		{"space and closing parenthesis", "text\t\n  word\n  )\n", "text\tword)\n", []string{"9", "J"}, 0, 9},
		{"blank line", "one\n  \n  two", "one two", []string{"3", "J"}, 0, 3},
		{"final newline", "one\ntwo\n", "one\ntwo\n", []string{"j", "J"}, 1, 0},
		{"visual single row", "one\n  two\nthree", "one two\nthree", []string{"v", "J"}, 0, 3},
		{"visual backwards", "one\n  two\n  three\nfour", "one two three\nfour", []string{"j", "j", "V", "2", "k", "J"}, 0, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tc.source, Anchor: -1}}
			v := &Vim{}
			if register := vimKeys(d, v, tc.keys...); register != "" {
				t.Fatalf("join returned register text %q", register)
			}
			row, col := d.Position()
			if d.Text != tc.want || row != tc.row || col != tc.col || v.VisualMode {
				t.Fatalf("join produced %q at %d:%d in %s", d.Text, row, col, v.Label())
			}
			vimKeys(d, v, "u")
			if d.Text != tc.source {
				t.Fatalf("undo produced %q", d.Text)
			}
		})
	}
}

func TestVimUserMappings(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		keys       []string
		want       string
		cursor     int
		visual     bool
	}{
		{"home", "  one two", []string{"L", "H"}, "  one two", 2, false},
		{"end", "  one two", []string{"L"}, "  one two", 8, false},
		{"redo", "abc", []string{"x", "u", "U"}, "bc", 0, false},
		{"word case", "one two", []string{"w", "`"}, "One two", 1, false},
		{"indent", "abc", []string{"tab"}, "\tabc", 1, false},
		{"unindent tab", "\tabc", []string{"shift+tab"}, "abc", 0, false},
		{"unindent spaces", "    abc", []string{"shift+tab"}, "abc", 0, false},
		{"two spaces", "a\n  b\n    c\n  d", []string{"j", "j", "shift+tab"}, "a\n  b\n  c\n  d", 6, false},
		{"four spaces", "a\n    b\n        c", []string{"tab"}, "    a\n    b\n        c", 4, false},
		{"retain style after outdent", "  abc", []string{"shift+tab", "tab"}, "  abc", 2, false},
		{"tabs with alignment", "\ta\n\t  b", []string{"j", "tab", "shift+tab"}, "\ta\n\t  b", 3, false},
		{"visual indent", "a\nb", []string{"V", "j", "tab"}, "\ta\n\tb", 4, true},
		{"visual two spaces", "a\n  b\n    c", []string{"V", "j", "tab", "tab", "shift+tab"}, "  a\n    b\n    c", 6, true},
		{"visual repeat indent", "a\nb", []string{"V", "j", "tab", "tab", "shift+tab"}, "\ta\n\tb", 4, true},
		{"visual H L", "  abc", []string{"v", "L", "H"}, "  abc", 2, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || d.Cursor != tt.cursor || v.VisualMode != tt.visual {
				t.Fatalf("text %q cursor %d visual %v", d.Text, d.Cursor, v.VisualMode)
			}
		})
	}
}
func TestVimChangeWord(t *testing.T) {
	for _, tt := range []struct {
		name, text, want, register string
		column                     int
		keys                       []string
	}{
		{"word", "one two", " two", "one", 0, []string{"c", "w"}},
		{"last character", "one two", "on two", "e", 2, []string{"c", "w"}},
		{"whitespace", "one  two", "onetwo", "  ", 3, []string{"c", "w"}},
		{"count", "a two three", " three", "a two", 0, []string{"c", "2", "w"}},
		{"grapheme", "e\u0301 two", " two", "e\u0301", 0, []string{"c", "w"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
			d.SetPosition(0, tt.column)
			v := &Vim{}
			if copied := vimKeys(d, v, tt.keys...); copied != tt.register || !v.InsertMode || d.Text != tt.want || d.Cursor != tt.column {
				t.Fatalf("text %q cursor %d insert %v register %q", d.Text, d.Cursor, v.InsertMode, copied)
			}
		})
	}
}

func TestVimReplaceCharacter(t *testing.T) {
	for _, tt := range []struct {
		name, text, want string
		keys             []string
		cursor           int
	}{
		{"digit", "abc", "2bc", []string{"r", "2"}, 0},
		{"counted graphemes", "e\u0301👩‍💻c", "中中c", []string{"2", "r", "中"}, 1},
		{"count exceeds line", "ab\nc", "ab\nc", []string{"3", "r", "X"}, 0},
		{"line break", "abc", "\nc", []string{"2", "r", "enter"}, 1},
		{"escape", "abc", "abc", []string{"r", "esc"}, 0},
		{"repeat", "abc", "XXc", []string{"r", "X", "l", "."}, 1},
		{"undo", "abc", "abc", []string{"2", "r", "X", "u"}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Buffer: Buffer{Text: tt.text, Anchor: -1}}
			v := &Vim{}
			if copied := vimKeys(d, v, tt.keys...); d.Text != tt.want || d.Cursor != tt.cursor || v.InsertMode || v.CommandPending() || copied != "" {
				t.Fatalf("text %q cursor %d mode %s register %q", d.Text, d.Cursor, v.Label(), copied)
			}
		})
	}
}

func TestVimSubstituteCharacters(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "e\u0301👩‍💻cd", Anchor: -1}}
	v := &Vim{}
	if copied := vimKeys(d, v, "2", "s"); copied != "e\u0301👩‍💻" || d.Text != "cd" || !v.InsertMode {
		t.Fatalf("text %q mode %s register %q", d.Text, v.Label(), copied)
	}
	vimKeys(d, v, "X", "esc", "l", ".")
	if d.Text != "XX" || v.InsertMode {
		t.Fatalf("repeat: text %q mode %s", d.Text, v.Label())
	}
	vimKeys(d, v, "2", "u")
	if d.Text != "e\u0301👩‍💻cd" {
		t.Fatalf("undo: text %q", d.Text)
	}
}

func TestVimRegisterYank(t *testing.T) {
	for _, tt := range []struct {
		keys []string
		want string
	}{{[]string{"y", "y"}, "one two\n"}, {[]string{"y", "w"}, "one "}, {[]string{"v", "l", "y"}, "on"}} {
		d := &Document{Buffer: Buffer{Text: "one two\nthree", Anchor: -1}}
		v := &Vim{}
		text := vimKeys(d, v, tt.keys...)
		if text != tt.want || v.VisualMode || d.Text != "one two\nthree" {
			t.Fatalf("%v: %q", tt.keys, text)
		}
		if copied := vimKeys(d, v, "esc"); copied != "" {
			t.Fatal("later command returned the previous yank")
		}
	}
}
func TestVimRegisterPut(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "abc", Anchor: -1}}
	v := &Vim{}
	v.Handle(d, "p", VimRegisterValue{Text: "👩‍💻"})
	if d.Text != "a👩‍💻bc" {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "abc" {
		t.Fatal(d.Text)
	}
	d.SetPosition(0, 0)
	vimKeys(d, v, "v", "l")
	v.Handle(d, "p", VimRegisterValue{Text: "X"})
	if d.Text != "Xc" || v.VisualMode {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "abc" {
		t.Fatal(d.Text)
	}
}

func TestVimEditsUpdateRegister(t *testing.T) {
	for _, tt := range []struct {
		keys []string
		want string
	}{
		{[]string{"d", "w"}, "one "},
		{[]string{"c", "e"}, "one"},
		{[]string{"x"}, "o"},
		{[]string{"x", "."}, "n"},
	} {
		d := &Document{Buffer: Buffer{Text: "one two", Anchor: -1}}
		v := &Vim{}
		if copied := vimKeys(d, v, tt.keys...); copied != tt.want {
			t.Fatalf("%v register = %q, want %q", tt.keys, copied, tt.want)
		}
	}
}

func TestVimLineBoundaryAliases(t *testing.T) {
	for _, pair := range [][2]string{{"L", "$"}, {"H", "^"}} {
		for _, prefix := range [][]string{nil, {"d"}, {"c"}, {"y"}, {"v"}, {"V"}} {
			docs := [2]*Document{}
			modes := [2]*Vim{}
			copied := [2]string{}
			for i, key := range pair {
				d := &Document{Buffer: Buffer{Text: "  abc\nnext", Anchor: -1}}
				d.SetPosition(0, 3)
				v := &Vim{}
				keys := append(append([]string(nil), prefix...), key)
				copied[i] = vimKeys(d, v, keys...)
				docs[i] = d
				modes[i] = v
			}
			a, b := docs[0], docs[1]
			av, bv := modes[0], modes[1]
			if a.Text != b.Text || a.Cursor != b.Cursor || a.SelectedText() != b.SelectedText() || av.Label() != bv.Label() || copied[0] != copied[1] {
				t.Fatalf("%v with %v differ", pair, prefix)
			}
		}
	}
	for _, tt := range []struct {
		key string
		col int
	}{{"0", 0}, {"H", 2}, {"^", 2}, {"L", 4}, {"$", 4}} {
		d := &Document{Buffer: Buffer{Text: "  abc", Anchor: -1}}
		d.SetPosition(0, 3)
		v := &Vim{}
		vimKeys(d, v, tt.key)
		if d.Cursor != tt.col {
			t.Fatalf("%s moved to %d, want %d", tt.key, d.Cursor, tt.col)
		}
	}
}
