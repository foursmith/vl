package editor

import "testing"

func TestVimDelimitedObjects(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		col        int
		keys       []string
		want       string
	}{
		{"quote change", `say("hello")`, 6, []string{"c", "i", "\"", "X", "esc"}, `say("X")`},
		{"quote around", `say("hello")`, 6, []string{"d", "a", "\""}, `say()`},
		{"single quote", `'hello' world`, 3, []string{"d", "i", "'"}, `'' world`},
		{"escaped quote", `"a\"b" tail`, 2, []string{"d", "i", "\""}, `"" tail`},
		{"backtick", "`hello` tail", 3, []string{"c", "i", "`", "X", "esc"}, "`X` tail"},
		{"next quote", `say "hello"`, 0, []string{"d", "a", "\""}, `say `},
		{"empty inner", `""`, 0, []string{"c", "i", "\"", "X", "esc"}, `"X"`},
		{"bracket", `foo(a, b)`, 5, []string{"d", "i", "("}, `foo()`},
		{"nested", `foo(a, (b), c)`, 8, []string{"d", "i", "("}, `foo(a, (), c)`},
		{"outer", `foo(a, (b), c)`, 8, []string{"d", "2", "i", "("}, `foo()`},
		{"quoted bracket", `foo("(", value)`, 11, []string{"d", "i", "("}, `foo()`},
		{"multiline", "{\n  value\n}", 5, []string{"d", "a", "{"}, ""},
		{"square", `[one, two]`, 2, []string{"d", "i", "]"}, `[]`},
		{"next pair", `call(value)`, 1, []string{"c", "i", "(", "X", "esc"}, `call(X)`},
		{"unmatched", `call(value`, 6, []string{"d", "a", "("}, `call(value`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			d.Cursor = tt.col
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode {
				t.Fatalf("got %q mode %s", d.Text, v.Label())
			}
			if tt.text != tt.want {
				vimKeys(d, v, "u")
				if d.Text != tt.text {
					t.Fatal("undo:", d.Text)
				}
			}
		})
	}
	d := &Document{Text: `foo("hello")`, Cursor: 7, Anchor: -1}
	v := &Vim{}
	vimKeys(d, v, "v", "i", "\"")
	if d.SelectedText() != "hello" {
		t.Fatal("visual quote:", d.SelectedText())
	}
}
func TestVimRepeatChanges(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		keys       []string
		want       string
	}{
		{"delete word", "one two three", []string{"d", "a", "w", "."}, "three"},
		{"counted delete", "abcdefghi", []string{"2", "x", "."}, "efghi"},
		{"repeat count", "abcdefghi", []string{"x", "3", "."}, "efghi"},
		{"insert", "abc", []string{"i", "X", "Y", "esc", "l", "."}, "XYXYabc"},
		{"append", "abc", []string{"a", "X", "esc", "l", "."}, "aXbXc"},
		{"change words", "one two three", []string{"c", "i", "w", "X", "esc", "w", "."}, "X X three"},
		{"change quotes", `"one" "two"`, []string{"c", "i", "\"", "X", "esc", "L", "."}, `"X" "X"`},
		{"open line", "a\nb", []string{"o", "X", "esc", "."}, "a\nX\nX\nb"},
		{"join lines", "a\nb\nc\nd\n", []string{"J", "j", "."}, "a b\nc d\n"},
		{"repeat visual join by line count", "a\nb\nc\na much longer line\nb\nc\n", []string{"v", "2", "j", "J", "j", "."}, "a b c\na much longer line b c\n"},
		{"visual delete", "abcdef", []string{"v", "l", "d", "."}, "ef"},
		{"visual change", "abcdef", []string{"v", "l", "c", "X", "esc", "l", "."}, "XXef"},
		{"indent", "abc", []string{"tab", "."}, "\t\tabc"},
		{"undo keeps repeat", "abc", []string{"x", "u", "."}, "bc"},
		{"unicode insert", "中", []string{"i", "👩‍💻", "esc", "l", "."}, "👩‍💻👩‍💻中"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := &Document{Text: tt.text, Anchor: -1}
			v := &Vim{}
			vimKeys(d, v, tt.keys...)
			if d.Text != tt.want || v.InsertMode || v.VisualMode {
				t.Fatalf("got %q want %q mode %s", d.Text, tt.want, v.Label())
			}
			changed := d.Text
			vimKeys(d, v, "u", "U")
			if d.Text != changed {
				t.Fatal("repeat undo/redo failed")
			}
		})
	}
}

func TestVimRepeatInsertEditingAndSafety(t *testing.T) {
	d := &Document{Text: "abc def", Cursor: 2, Anchor: -1}
	v := &Vim{}
	v.Handle(d, "i", "")
	v.Handle(d, "backspace", "")
	d.Delete(true)
	v.Handle(d, "X", "")
	d.Insert("X")
	v.Handle(d, "esc", "")
	if d.Text != "aXc def" {
		t.Fatal(d.Text)
	}
	d.SetPosition(0, 6)
	vimKeys(d, v, ".")
	if d.Text != "aXc dXf" {
		t.Fatal("backspace repeat:", d.Text)
	}
	d = &Document{Text: "abcdef", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "x", "i")
	v.Handle(d, "left", "")
	d.Move("left", false)
	v.Handle(d, "Q", "")
	d.Insert("Q")
	v.Handle(d, "esc", "")
	vimKeys(d, v, ".")
	if d.Text != "bcdef" {
		t.Fatal("unsafe insert replaced previous repeat:", d.Text)
	}
	d = &Document{Text: "abc", Anchor: -1}
	v = &Vim{}
	v.Handle(d, "p", "X")
	d.SetPosition(0, 3)
	v.Handle(d, ".", "new register text")
	if d.Text != "aXbcX" {
		t.Fatal("register repeat:", d.Text)
	}
	d = &Document{Text: `""`, Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "v", "i", "\"")
	if d.SelectedText() != "" {
		t.Fatal("empty object selected a delimiter")
	}
}
func TestVimFlashOperatorsAndRepeat(t *testing.T) {
	for _, tt := range []struct{ op, want string }{{"d", "b,c"}, {"c", "Xb,c"}, {"y", "a,b,c"}} {
		d := &Document{Text: "a,b,c", Anchor: -1}
		v := &Vim{}
		copied := v.ApplyFlash(d, 1, ",", tt.op)
		if tt.op == "c" {
			vimKeys(d, v, "X", "esc")
		}
		if d.Text != tt.want {
			t.Fatalf("%s: %q", tt.op, d.Text)
		}
		if copied != "a," {
			t.Fatal("Flash operator register:", copied)
		}
		if tt.op == "d" {
			register := vimKeys(d, v, ".")
			if d.Text != "c" || register != "b," {
				t.Fatal("Flash repeat:", d.Text)
			}
			vimKeys(d, v, "u")
			if d.Text != "b,c" {
				t.Fatal("Flash repeat undo:", d.Text)
			}
		}
	}
	d := &Document{Text: "a👩‍💻b👩‍💻c", Anchor: -1}
	v := &Vim{}
	v.ApplyFlash(d, 1, "👩‍💻", "d")
	vimKeys(d, v, ".")
	if d.Text != "c" {
		t.Fatal("Unicode Flash repeat:", d.Text)
	}
	d = &Document{Text: "a,b", Anchor: -1}
	v = &Vim{}
	vimKeys(d, v, "d")
	if op, ok := v.FlashOperator(); !ok || op != "d" {
		t.Fatal("pending operator not accepted")
	}
	v.CancelPending()
	if d.Text != "a,b" || v.CommandPending() {
		t.Fatal("cancel edited or retained operator")
	}
}

func TestVimFlashChangeRepeatAndMissingTarget(t *testing.T) {
	d := &Document{Text: "a,b,c", Anchor: -1}
	v := &Vim{}
	v.ApplyFlash(d, 1, ",", "c")
	vimKeys(d, v, "X", "esc", "l", ".")
	if d.Text != "XXc" || v.InsertMode {
		t.Fatal("Flash change repeat:", d.Text)
	}
	vimKeys(d, v, "u")
	if d.Text != "Xb,c" {
		t.Fatal("Flash change repeat undo:", d.Text)
	}
	d = &Document{Text: "a,b", Anchor: -1}
	v = &Vim{}
	v.ApplyFlash(d, 1, ",", "d")
	vimKeys(d, v, ".")
	if d.Text != "b" || v.InsertMode {
		t.Fatal("missing repeat target changed text:", d.Text)
	}
	d = &Document{Text: "é é", Anchor: -1}
	if len(flashTextPositions(d, "e")) != 0 || len(flashTextPositions(d, "é")) != 2 {
		t.Fatal("Flash split a combining cluster")
	}
}
