package editor

import "testing"

func TestReadOnlyVimRejectsMutationsAndSupportsYank(t *testing.T) {
	source := "one two\nthree four\n"
	for _, keys := range [][]string{
		{"i", "X"}, {"a", "X"}, {"I"}, {"A"}, {"o"}, {"O"}, {"x"}, {"d", "d"}, {"c", "c"}, {"p"}, {"P"}, {"u"}, {"U"}, {"tab"}, {"`"}, {"."},
		{"v", "l", "d"}, {"V", "j", "c"}, {"v", "S", "\""}, {"y", "s", "i", "w", "\""},
	} {
		d := &Document{ReadOnly: true, Text: source, Anchor: -1}
		v := &Vim{}
		for _, key := range keys {
			v.Handle(d, key)
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
	d := &Document{ReadOnly: true, Text: source, Anchor: -1}
	v := &Vim{}
	for _, key := range []string{"w", "v", "e", "y"} {
		v.Handle(d, key)
	}
	text, ok := v.TakeClipboardCopy()
	if !ok || text != "two" || d.Text != source || v.VisualMode {
		t.Fatalf("read-only Visual yank: %q", text)
	}
	v.Handle(d, "g")
	v.Handle(d, "g")
	v.Handle(d, "y")
	v.Handle(d, "i")
	v.Handle(d, "w")
	text, ok = v.TakeClipboardCopy()
	if !ok || text != "one" {
		t.Fatalf("read-only text object: %q", text)
	}
}
