package editor

import "testing"

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
		{"visual indent", "a\nb", []string{"V", "j", "tab"}, "\ta\n\tb", 4, true},
		{"visual repeat indent", "a\nb", []string{"V", "j", "tab", "tab", "shift+tab"}, "\ta\n\tb", 4, true},
		{"visual H L", "  abc", []string{"v", "L", "H"}, "  abc", 2, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || d.Cursor != tt.cursor || v.VisualMode != tt.visual {
				t.Fatalf("text %q cursor %d visual %v", d.Text, d.Cursor, v.VisualMode)
			}
		})
	}
}
func TestVimFocusAction(t *testing.T) {
	d := &Document{Text: "one two", Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "c", "w")
	if v.TakeAction() != "focus" || v.InsertMode || d.Text != "one two" {
		t.Fatal("cw did not become focus mapping")
	}
}
func TestVimClipboardYank(t *testing.T) {
	for _, tt := range []struct {
		keys []string
		want string
	}{{[]string{"y", "y"}, "one two\n"}, {[]string{"y", "w"}, "one "}, {[]string{"v", "l", "y"}, "on"}} {
		d := &Document{Text: "one two\nthree", Anchor: -1}
		v := &Vim{}
		vimKeys(d, v, tt.keys...)
		text, ready := v.TakeClipboardCopy()
		if !ready || text != tt.want || v.VisualMode || d.Text != "one two\nthree" {
			t.Fatalf("%v: %q ready %v", tt.keys, text, ready)
		}
		if _, ready = v.TakeClipboardCopy(); ready {
			t.Fatal("clipboard yank delivered twice")
		}
	}
}
func TestVimClipboardPaste(t *testing.T) {
	d := &Document{Text: "abc", Anchor: -1}
	v := &Vim{}
	v.PasteClipboard(d, "👩‍💻", true)
	if d.Text != "a👩‍💻bc" {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "abc" {
		t.Fatal(d.Text)
	}
	d.SetPosition(0, 0)
	vimKeys(d, v, "v", "l")
	v.PasteClipboard(d, "X", true)
	if d.Text != "Xc" || v.VisualMode {
		t.Fatal(d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "abc" {
		t.Fatal(d.Text)
	}
}

func TestVimUnifiedClipboard(t *testing.T) {
	for _, keys := range [][]string{{"d", "w"}, {"c", "e"}, {"x"}} {
		d := &Document{Text: "one two", Anchor: -1}
		v := &Vim{}
		vimKeys(d, v, keys...)
		if text, ready := v.TakeClipboardCopy(); !ready || text == "" {
			t.Fatalf("%v did not write clipboard", keys)
		}
	}
	d := &Document{Text: "abc", Anchor: -1}
	v := &Vim{}
	v.PasteClipboard(d, "X", false)
	if d.Text != "Xabc" {
		t.Fatal(d.Text)
	}
}

func TestVimLineBoundaryAliases(t *testing.T) {
	for _, pair := range [][2]string{{"L", "$"}, {"H", "^"}} {
		for _, prefix := range [][]string{nil, {"d"}, {"c"}, {"y"}, {"v"}, {"V"}} {
			docs := [2]*Document{}
			modes := [2]*Vim{}
			for i, key := range pair {
				d := &Document{Text: "  abc\nnext", Anchor: -1}
				d.SetPosition(0, 3)
				v := &Vim{}
				keys := append(append([]string(nil), prefix...), key)
				vimKeys(d, v, keys...)
				docs[i] = d
				modes[i] = v
			}
			a, b := docs[0], docs[1]
			av, bv := modes[0], modes[1]
			if a.Text != b.Text || a.Cursor != b.Cursor || a.SelectedText() != b.SelectedText() || av.Label() != bv.Label() || av.clipboardText != bv.clipboardText || av.clipboardReady != bv.clipboardReady {
				t.Fatalf("%v with %v differ", pair, prefix)
			}
		}
	}
	for _, tt := range []struct {
		key string
		col int
	}{{"0", 0}, {"H", 2}, {"^", 2}, {"L", 4}, {"$", 4}} {
		d := &Document{Text: "  abc", Anchor: -1}
		d.SetPosition(0, 3)
		v := &Vim{}
		vimKeys(d, v, tt.key)
		if d.Cursor != tt.col {
			t.Fatalf("%s moved to %d, want %d", tt.key, d.Cursor, tt.col)
		}
	}
}
