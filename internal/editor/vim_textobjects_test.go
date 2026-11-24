package editor

import "testing"

func TestVimWordObjects(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		col        int
		keys       []string
		want, copy string
	}{
		{"around middle", "alpha beta gamma", 8, []string{"d", "a", "w"}, "alpha gamma", "beta "},
		{"inside middle", "alpha beta gamma", 8, []string{"d", "i", "w"}, "alpha  gamma", "beta"},
		{"final word", "alpha beta", 8, []string{"d", "a", "w"}, "alpha", " beta"},
		{"line end", "alpha\nbeta", 2, []string{"d", "a", "w"}, "\nbeta", "alpha"},
		{"punctuation", "foo.bar", 1, []string{"d", "a", "w"}, ".bar", "foo"},
		{"big word", "foo.bar next", 1, []string{"d", "a", "W"}, "next", "foo.bar "},
		{"whitespace", "one   two", 4, []string{"d", "a", "w"}, "one", "   two"},
		{"inner whitespace", "one   two", 4, []string{"d", "i", "w"}, "onetwo", "   "},
		{"count", "one two three four", 1, []string{"2", "d", "a", "w"}, "three four", "one two "},
		{"motion count", "one two three", 1, []string{"d", "2", "a", "w"}, "three", "one two "},
		{"change inside", "alpha beta", 2, []string{"c", "i", "w", "X", "esc"}, "X beta", "alpha"},
		{"change around", "alpha beta", 2, []string{"c", "a", "w", "X", "esc"}, "Xbeta", "alpha "},
		{"copy inside", "alpha beta", 2, []string{"y", "i", "w"}, "alpha beta", "alpha"},
		{"copy around", "alpha beta", 2, []string{"y", "a", "w"}, "alpha beta", "alpha "},
		{"emoji", "👩‍💻 中", 0, []string{"d", "a", "w"}, "中", "👩‍💻 "},
		{"combining", "é 中", 0, []string{"d", "i", "w"}, " 中", "é"},
		{"mixed scripts", "foo_中文bar", 5, []string{"d", "i", "w"}, "foo_bar", "中文"},
		{"empty", "", 0, []string{"d", "a", "w"}, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			d.SetPosition(0, tt.col)
			v := &Vim{}
			copied := vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || copied != tt.copy || v.InsertMode || v.VisualMode {
				t.Fatalf("text %q register %q mode %s", d.Text, copied, v.Label())
			}
			if tt.want != tt.text {
				vimKeys(d, v, "u")
				if d.Text != tt.text {
					t.Fatal("undo did not restore text:", d.Text)
				}
			}
		})
	}
}
func TestVimVisualWordObjects(t *testing.T) {
	for _, tt := range []struct{ key, want string }{{"i", "beta"}, {"a", "beta "}} {
		d := &Document{Text: "alpha beta gamma", Anchor: -1}
		d.SetPosition(0, 8)
		v := &Vim{}
		vimKeys(d, v, "v", tt.key, "w")
		if d.SelectedText() != tt.want || !v.VisualMode || v.VisualLine {
			t.Fatal(d.SelectedText(), v.Label())
		}
		vimKeys(d, v, "d")
		if v.VisualMode {
			t.Fatal("delete retained visual mode")
		}
	}
	d := &Document{Text: "alpha beta", Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "d", "a", "esc", "w")
	if d.Text != "alpha beta" || v.InsertMode {
		t.Fatal("Escape left text object armed")
	}
}
