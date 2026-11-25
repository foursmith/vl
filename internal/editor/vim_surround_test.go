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
			d := &Document{Buffer: Buffer{Text: tt.text, Cursor: tt.cursor, Anchor: -1}}
			v := &Vim{}
			copied := vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode || v.CommandPending() {
				t.Fatalf("text %q mode %s", d.Text, v.Label())
			}
			if copied != "" {
				t.Fatal("surround changed the register")
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
	d := &Document{Buffer: Buffer{Text: "(one) (two)", Cursor: 2, Anchor: -1}}
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
		{"add opening angle", "hello world", 2, []string{"y", "s", "i", "w", "<"}, "<hello> world"},
		{"add closing angle", "hello world", 2, []string{"y", "s", "i", "w", ">"}, "<hello> world"},
		{"delete angle", "<hello>", 2, []string{"d", "s", "<"}, "hello"},
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
			d := &Document{Buffer: Buffer{Text: tt.text, Cursor: tt.cursor, Anchor: -1}}
			v := &Vim{}
			copied := vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode || v.CommandPending() {
				t.Fatalf("text %q mode %s", d.Text, v.Label())
			}
			if copied != "" {
				t.Fatal("surround changed the register")
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
	d := &Document{Buffer: Buffer{Text: "one two", Anchor: -1}}
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
	d = &Document{Buffer: Buffer{Text: "one\ntwo\n", Anchor: -1}}
	v = &Vim{}
	vimKeys(d, v, "V", "j", "S", "]")
	if d.Text != "[one\ntwo]\n" {
		t.Fatal("Visual line surround:", d.Text)
	}
	d = &Document{Buffer: Buffer{Text: "one two", Anchor: -1}}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w", "\"")
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != `"one" "two"` {
		t.Fatal("word repeat:", d.Text)
	}
	d = &Document{Buffer: Buffer{Text: `"one" "two"`, Cursor: 2, Anchor: -1}}
	v = &Vim{}
	vimKeys(d, v, "d", "s", "\"")
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != "one two" {
		t.Fatal("delete repeat:", d.Text)
	}
}
func TestVimSurroundFlashRangeAndRepeat(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "a,b,c", Anchor: -1}}
	v := &Vim{}
	vimKeys(d, v, "y", "s")
	if op, ok := v.FlashOperator(); !ok || op != "ys" {
		t.Fatal("surround range did not accept Flash")
	}
	copied := v.ApplyFlash(d, 1, ",", "ys")
	vimKeys(d, v, "]")
	if d.Text != "[a,]b,c" {
		t.Fatal(d.Text)
	}
	d.SetPosition(0, 4)
	replayed := vimKeys(d, v, ".")
	if d.Text != "[a,][b,]c" {
		t.Fatal("Flash surround repeat:", d.Text)
	}
	if copied != "" || replayed != "" {
		t.Fatal("Flash surround changed the register")
	}
}

func TestVimSurroundMotions(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "one\ntwo\nthree", Cursor: 8, Anchor: -1}}
	v := &Vim{}
	vimKeys(d, v, "y", "s", "g", "g", "]")
	if d.Text != "[one\ntwo\nthree]" {
		t.Fatal("first-line surround motion:", d.Text)
	}
	d = &Document{Buffer: Buffer{Text: "one\ntwo\nthree", Anchor: -1}}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "2", "G", "]")
	if d.Text != "[one\ntwo]\nthree" {
		t.Fatal("counted G surround:", d.Text)
	}
	d = &Document{Buffer: Buffer{Text: "hello", Anchor: -1}}
	v = &Vim{}
	vimKeys(d, v, "y", "s", "i", "w")
	if handled, _ := v.Handle(d, "f1", ""); handled || v.CommandPending() || d.Text != "hello" {
		t.Fatal("function shortcut trapped by pending surround")
	}
}

func TestVimSurroundRejectsStaleRange(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "hello", Anchor: -1}}
	v := &Vim{}
	vimKeys(d, v, "y", "s", "i", "w")
	d.Edit("external")
	vimKeys(d, v, ">")
	if d.Text != "external" || v.CommandPending() {
		t.Fatal("stale surround range applied")
	}
}

func TestVimTagCommandsAreUnmapped(t *testing.T) {
	for _, keys := range [][]string{
		{"d", "s", "t"}, {"c", "s", "t"}, {"c", "i", "t"}, {"d", "a", "t"}, {"y", "s", "i", "w", "t"},
	} {
		d := &Document{Buffer: Buffer{Text: "<p>hello</p>", Cursor: 5, Anchor: -1}}
		v := &Vim{}
		vimKeys(d, v, keys...)
		if d.Text != "<p>hello</p>" || v.InsertMode || v.CommandPending() {
			t.Fatalf("%v changed text or mode: %q, %s", keys, d.Text, v.Label())
		}
	}
}
