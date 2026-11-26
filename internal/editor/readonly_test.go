package editor

import "testing"

func TestReadOnlyVimRejectsMutationsAndSupportsYank(t *testing.T) {
	source := "one two\nthree four\n"
	for _, keys := range [][]string{
		{"i", "X"}, {"a", "X"}, {"I"}, {"A"}, {"o"}, {"O"}, {"x"}, {"d", "d"}, {"c", "c"}, {"p"}, {"P"}, {"u"}, {"U"}, {"tab"}, {"`"}, {"."},
		{"v", "l", "d"}, {"V", "j", "c"}, {"v", "S", "\""}, {"y", "s", "i", "w", "\""}, {"J"}, {"V", "j", "J"},
	} {
		d := &Document{Buffer: Buffer{ReadOnly: true, Text: source, Anchor: -1}}
		v := &Vim{}
		for _, key := range keys {
			v.Handle(d, key, VimRegisterValue{})
		}
		if d.Text != source || d.Dirty() || v.InsertMode {
			t.Fatalf("read-only mutation via %v", keys)
		}
		d.Insert("X")
		d.Delete(false)
		d.Edit("X")
		d.Undo()
		d.Redo()
		if d.Text != source || d.Save() == nil {
			t.Fatal("document boundary allowed mutation or save")
		}
	}
	d := &Document{Buffer: Buffer{ReadOnly: true, Text: source, Anchor: -1}}
	v := &Vim{}
	text := vimKeys(d, v, "w", "v", "e", "y")
	if text != "two" || d.Text != source || v.VisualMode {
		t.Fatalf("read-only Visual yank: %q", text)
	}
	text = vimKeys(d, v, "g", "g", "y", "i", "w")
	if text != "one" {
		t.Fatalf("read-only text object: %q", text)
	}
}
