package editor

import "testing"

func TestVimChangeSurround(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		cursor     int
		keys       []string
		want       string
	}{
		{"parentheses to brackets", "call(value)", 7, []string{"c", "s", "(", "["}, "call[value]"},
		{"closing aliases", "call(value)", 7, []string{"c", "s", ")", "]"}, "call[value]"},
		{"quotes to brackets", `"value"`, 3, []string{"c", "s", "\"", "["}, `[value]`},
		{"brackets to quotes", `[value]`, 3, []string{"c", "s", "[", "'"}, `'value'`},
		{"backticks", "`value`", 3, []string{"c", "s", "`", "\""}, `"value"`},
		{"nested", "outer(inner(value))", 13, []string{"c", "s", "(", "["}, "outer(inner[value])"},
		{"outer count", "outer(inner(value))", 13, []string{"2", "c", "s", "(", "["}, "outer[inner(value)]"},
		{"preserve whitespace", "(  value  )", 5, []string{"c", "s", "(", "["}, "[  value  ]"},
		{"unicode", "(👩‍💻é中)", 1, []string{"c", "s", "(", "["}, "[👩‍💻é中]"},
		{"empty", "()", 0, []string{"c", "s", "(", "["}, "[]"},
		{"multiline", "{\n  value\n}", 5, []string{"c", "s", "{", "["}, "[\n  value\n]"},
		{"missing source", "plain", 2, []string{"c", "s", "(", "["}, "plain"},
		{"invalid replacement", "(value)", 2, []string{"c", "s", "(", "z"}, "(value)"},
		{"escape", "(value)", 2, []string{"c", "s", "(", "esc", "["}, "(value)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Cursor: tt.cursor, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode || v.CommandPending() {
				t.Fatalf("text %q mode %s", d.Text, v.Label())
			}
			if _, ready := v.TakeClipboardCopy(); ready {
				t.Fatal("surround changed clipboard")
			}
			if tt.want != tt.text {
				if d.Cursor != tt.cursor {
					t.Fatal("surround moved cursor:", d.Cursor)
				}
				vimKeys(d, v, "u")
				if d.Text != tt.text {
					t.Fatal("surround undo:", d.Text)
				}
				vimKeys(d, v, "U")
				if d.Text != tt.want {
					t.Fatal("surround redo:", d.Text)
				}
			}
		})
	}
}
func TestVimChangeSurroundRepeat(t *testing.T) {
	d := &Document{Text: "(one) (two)", Cursor: 2, Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "c", "s", "(", "[")
	d.SetPosition(0, 8)
	vimKeys(d, v, ".")
	if d.Text != "[one] [two]" {
		t.Fatal("repeat:", d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "[one] (two)" {
		t.Fatal("repeat undo:", d.Text)
	}
}

func TestVimDeleteAndAddSurround(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		cursor     int
		keys       []string
		want       string
	}{
		{"delete quotes", `"hello" world`, 3, []string{"d", "s", "\""}, "hello world"},
		{"delete braces", "{ hello }", 3, []string{"d", "s", "{"}, " hello "},
		{"delete nested", "((hello))", 4, []string{"2", "d", "s", "("}, "(hello)"},
		{"delete empty", "[]", 0, []string{"d", "s", "]"}, ""},
		{"add word", "hello world", 2, []string{"y", "s", "i", "w", "\""}, `"hello" world`},
		{"add around word", "hello world", 2, []string{"y", "s", "a", "w", "]"}, "[hello ]world"},
		{"add motion", "hello world", 0, []string{"y", "s", "w", ")"}, "(hello )world"},
		{"add end motion", "hello world", 0, []string{"y", "s", "e", ")"}, "(hello) world"},
		{"add reverse motion", "hello world", 3, []string{"y", "s", "0", "]"}, "[hel]lo world"},
		{"add empty motion", "hello", 0, []string{"y", "s", "0", "\""}, "hello"},
		{"add line", "hello world\nnext", 3, []string{"y", "s", "s", ")"}, "(hello world)\nnext"},
		{"add empty line", "", 0, []string{"y", "s", "s", "\""}, `""`},
		{"add counted word", "one two three", 1, []string{"2", "y", "s", "i", "w", "]"}, "[one two] three"},
		{"add counted lines", "one\ntwo\nthree", 0, []string{"2", "y", "s", "s", "]"}, "[one\ntwo]\nthree"},
		{"add empty object", "()", 0, []string{"y", "s", "i", "(", "]"}, "([])"},
		{"add unicode", "👩‍💻é 中", 0, []string{"y", "s", "i", "W", "\""}, "\"👩‍💻é\" 中"},
		{"cancel range", "hello", 0, []string{"y", "s", "i", "w", "esc", "\""}, "hello"},
		{"cancel delete", "(hello)", 2, []string{"d", "s", "esc", "("}, "(hello)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Cursor: tt.cursor, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode || v.CommandPending() {
				t.Fatalf("text %q mode %s", d.Text, v.Label())
			}
			if _, ready := v.TakeClipboardCopy(); ready {
				t.Fatal("surround changed clipboard")
			}
			if tt.want != tt.text {
				vimKeys(d, v, "u")
				if d.Text != tt.text {
					t.Fatal("undo:", d.Text)
				}
				vimKeys(d, v, "U")
				if d.Text != tt.want {
					t.Fatal("redo:", d.Text)
				}
			}
		})
	}
}
func TestVimVisualSurroundAndRepeat(t *testing.T) {
	d := &Document{Text: "one two", Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "v", "l", "l", "S", "]")
	if d.Text != "[one] two" || v.VisualMode {
		t.Fatal(d.Text, v.Label())
	}
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != "[one] [two]" {
		t.Fatal("visual repeat:", d.Text)
	}
	d = &Document{Text: "one\ntwo\n", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "V", "j", "S", "]")
	if d.Text != "[one\ntwo]\n" {
		t.Fatal("Visual line surround:", d.Text)
	}
	d = &Document{Text: "one two", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "\"")
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != `"one" "two"` {
		t.Fatal("word repeat:", d.Text)
	}
	d = &Document{Text: `"one" "two"`, Cursor: 2, Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "d", "s", "\"")
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != "one two" {
		t.Fatal("delete repeat:", d.Text)
	}
}
func TestVimSurroundTags(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		cursor     int
		keys       []string
		want       string
	}{
		{"add tag", "hello world", 2, []string{"y", "s", "i", "w", "<", "e", "m", ">"}, "<em>hello</em> world"},
		{"tag prompt", "hello", 2, []string{"y", "s", "i", "w", "t", "e", "m", "enter"}, "<em>hello</em>"},
		{"change quote to tag", `"hello"`, 2, []string{"c", "s", "\"", "<", "q", ">"}, "<q>hello</q>"},
		{"tag to quote", "<p>hello</p>", 5, []string{"c", "s", "t", "\""}, `"hello"`},
		{"remove tag", "<p>hello</p>", 5, []string{"d", "s", "t"}, "hello"},
		{"nested tags", "<div><em>hello</em></div>", 10, []string{"d", "s", "t"}, "<div>hello</div>"},
		{"counted tags", "<div><em>hello</em></div>", 10, []string{"2", "d", "s", "t"}, "<em>hello</em>"},
		{"attributes", "<p title=\"a>b\">hello</p>", 18, []string{"d", "s", "t"}, "hello"},
		{"void tag", "<p>hello<br>world</p>", 5, []string{"d", "s", "t"}, "hello<br>world"},
		{"self closing", "<p><img src='x'/>hello</p>", 18, []string{"d", "s", "t"}, "<img src='x'/>hello"},
		{"comments", "<p><!-- <bad> -->hello</p>", 20, []string{"d", "s", "t"}, "<!-- <bad> -->hello"},
		{"cdata", "<p><![CDATA[<bad>]]>hello</p>", 22, []string{"d", "s", "t"}, "<![CDATA[<bad>]]>hello"},
		{"raw script", "<script>const x = '<fake>';</script>", 15, []string{"d", "s", "t"}, "const x = '<fake>';"},
		{"mismatched tags", "<p><em>hello</p>", 10, []string{"d", "s", "t"}, "<p><em>hello</p>"},
		{"unicode", "<p>👩‍💻é中</p>", 4, []string{"d", "s", "t"}, "👩‍💻é中"},
		{"namespace", "<x:item id='1'>hello</x:item>", 16, []string{"d", "s", "t"}, "hello"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Cursor: tt.cursor, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.CommandPending() {
				t.Fatalf("text %q mode %s", d.Text, v.Label())
			}
			if tt.want != tt.text {
				vimKeys(d, v, "u")
				if d.Text != tt.text {
					t.Fatal("tag undo:", d.Text)
				}
			}
		})
	}
}
func TestVimSurroundTagPasteAndSafety(t *testing.T) {
	d := &Document{Text: "one\ntwo\n", Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "V", "j", "S")
	if !v.PasteSurround(d, `<p class="important">dd`) {
		t.Fatal("paste not accepted")
	}
	if d.Text != "<p class=\"important\">\none\ntwo\n</p>\n" || v.CommandPending() {
		t.Fatal("tag paste executed trailing commands:", d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "one\ntwo\n" {
		t.Fatal("tag block undo:", d.Text)
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "<", "b", "backspace", "e", "m", ">", ".")
	if d.Text != "<em><em>hello</em></em>" {
		t.Fatal("corrected tag repeat:", d.Text)
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "<", "b", "r", ">", "esc")
	if d.Text != "hello" {
		t.Fatal("void wrapper edited text")
	}
	vimKeys(d, v, "y", "s", "i", "w", "<", "e", "m")
	d.Edit("external")
	vimKeys(d, v, ">")
	if d.Text != "external" || v.CommandPending() {
		t.Fatal("stale surround range applied")
	}
}

func TestVimSurroundMarkupAndInputBoundaries(t *testing.T) {
	d := &Document{Text: `<?xml version="1.0"?><!DOCTYPE p [<!ELEMENT p (#PCDATA)>]><p>hello</p>`, Anchor: -1}
	d.Cursor = len([]rune(`<?xml version="1.0"?><!DOCTYPE p [<!ELEMENT p (#PCDATA)>]><p>he`))
	v := &Vim{}
	vimKeys(d, v, "d", "s", "t")
	if d.Text != `<?xml version="1.0"?><!DOCTYPE p [<!ELEMENT p (#PCDATA)>]>hello` {
		t.Fatal("declaration parsing:", d.Text)
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w")
	v.PasteSurround(d, `<p title="a>b">`)
	if d.Text != `<p title="a>b">hello</p>` {
		t.Fatal("attribute input:", d.Text)
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "<", "e", "m", "esc")
	if d.Text != "hello" || v.CommandPending() {
		t.Fatal("unfinished tag changed text")
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "<", "1", ">", "esc")
	if d.Text != "hello" {
		t.Fatal("invalid tag changed text")
	}
}
func TestVimSurroundFlashRangeAndRepeat(t *testing.T) {
	d := &Document{Text: "a,b,c", Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "y", "s")
	if op, ok := v.FlashOperator(); !ok || op != "ys" {
		t.Fatal("surround range did not accept Flash")
	}
	v.ApplyFlash(d, 1, ",", "ys")
	vimKeys(d, v, "]")
	if d.Text != "[a,]b,c" {
		t.Fatal(d.Text)
	}
	d.SetPosition(0, 4)
	vimKeys(d, v, ".")
	if d.Text != "[a,][b,]c" {
		t.Fatal("Flash surround repeat:", d.Text)
	}
	if _, ready := v.TakeClipboardCopy(); ready {
		t.Fatal("Flash surround changed clipboard")
	}
}

func TestVimTagTextObjectsAndSurroundMotions(t *testing.T) {
	d := &Document{Text: "<p>hello</p>", Cursor: 5, Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "c", "i", "t", "X", "esc")
	if d.Text != "<p>X</p>" {
		t.Fatal("inner tag object:", d.Text)
	}
	d = &Document{Text: "one\ntwo\nthree", Cursor: 8, Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "g", "g", "]")
	if d.Text != "[one\ntwo\nthree]" {
		t.Fatal("first-line surround motion:", d.Text)
	}
	d = &Document{Text: "one\ntwo\nthree", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "2", "G", "]")
	if d.Text != "[one\ntwo]\nthree" {
		t.Fatal("counted G surround:", d.Text)
	}
	d = &Document{Text: "hello", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "<", "e", "m")
	if v.Handle(d, "f1") || v.CommandPending() || d.Text != "hello" {
		t.Fatal("function shortcut trapped by tag input")
	}
}
