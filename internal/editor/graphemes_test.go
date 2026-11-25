package editor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDeleteToLineBoundaryPreservesGraphemesAndNewlines(t *testing.T) {
	const text = "a\n👩‍💻e\u0301\nb"
	for _, test := range []struct {
		name     string
		backward bool
		cursor   int
		want     string
	}{
		{"to start", true, 5, "a\ne\u0301\nb"},
		{"to end", false, 5, "a\n👩‍💻\nb"},
		{"at start", true, 2, text},
		{"at end", false, 7, text},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := Buffer{Text: text, Cursor: test.cursor, Anchor: -1}
			if changed := buffer.DeleteToLineBoundary(test.backward); changed != (test.want != text) || buffer.Text != test.want {
				t.Fatalf("delete returned %v and left %q, want %q", changed, buffer.Text, test.want)
			}
			buffer.Undo()
			if buffer.Text != text || buffer.Cursor != test.cursor {
				t.Fatalf("undo restored %q at %d, want %q at %d", buffer.Text, buffer.Cursor, text, test.cursor)
			}
		})
	}
}

func TestEditingNeverSplitsGraphemeClusters(t *testing.T) {
	for _, cluster := range []string{"中", "e\u0301", "👍🏽", "👨‍👩‍👧‍👦", "🇨🇳", "🏳️‍🌈", "1️⃣", "가", "क्\u200dष"} {
		t.Run(cluster, func(t *testing.T) {
			length := utf8.RuneCountInString(cluster)
			original := "a" + cluster + "b"
			d := &Document{Buffer: Buffer{Text: original, Anchor: -1}}
			d.Move("right", false)
			d.Move("right", false)
			if d.Cursor != 1+length {
				t.Fatalf("right stopped at %d", d.Cursor)
			}
			d.Move("left", false)
			if d.Cursor != 1 {
				t.Fatal("left split cluster")
			}
			for col := 2; col < 1+length; col++ {
				d.SetPosition(0, col)
				if d.Cursor != 1 {
					t.Fatal("position split cluster")
				}
			}
			d.SetPosition(0, 1)
			d.Move("right", true)
			if d.SelectedText() != cluster {
				t.Fatalf("selected %q", d.SelectedText())
			}
			d.Move("left", false)
			if d.Cursor != 1 || d.Anchor != -1 {
				t.Fatal("selection did not collapse")
			}
			d.Delete(false)
			if d.Text != "ab" || d.Cursor != 1 {
				t.Fatalf("forward delete: %q", d.Text)
			}
			d.Undo()
			if d.Text != original || d.Cursor != 1 {
				t.Fatal("undo lost cluster")
			}
			d.SetPosition(0, 1+length)
			d.Delete(true)
			if d.Text != "ab" || d.Cursor != 1 {
				t.Fatalf("backspace: %q", d.Text)
			}
			d.Undo()
			d.Redo()
			if d.Text != "ab" {
				t.Fatal("redo lost deletion")
			}
		})
	}
}
func TestInsertAndDeleteCanMergeAdjacentClusters(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "eX", Cursor: 1, Anchor: -1}}
	d.Insert("\u0301")
	if d.Text != "e\u0301X" || d.Cursor != 2 {
		t.Fatal("combining insertion did not join base")
	}
	d.Delete(true)
	if d.Text != "X" {
		t.Fatal("backspace left partial accent")
	}
	d = &Document{Buffer: Buffer{Text: "👩👩", Cursor: 1, Anchor: -1}}
	d.Insert("\u200d")
	if d.Cursor != 3 {
		t.Fatal("cursor stayed inside joined emoji")
	}
	d.Delete(true)
	if d.Text != "" {
		t.Fatal("joined emoji not deleted together")
	}
	d = &Document{Buffer: Buffer{Text: "🇨x🇳", Cursor: 1, Anchor: -1}}
	d.Delete(false)
	if d.Text != "🇨🇳" || d.Cursor != 0 {
		t.Fatal("delete left cursor inside merged flag")
	}
}
func TestSelectionVerticalMotionAndNewlinesUseBoundaries(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: "xx\ne\u0301z\n🇨🇳", Anchor: -1}}
	d.SetPosition(0, 1)
	d.Move("down", false)
	if d.Cursor != 3 {
		t.Fatal("vertical motion split accent")
	}
	d.SelectRange(4, 5)
	if d.SelectedText() != "e\u0301" {
		t.Fatal("partial selection did not expand")
	}
	d.Insert("q")
	if d.Text != "xx\nqz\n🇨🇳" {
		t.Fatal("replacement left combining mark")
	}
	d.SetPosition(2, 0)
	d.Move("left", false)
	if line, col := d.Position(); line != 1 || col != 2 {
		t.Fatalf("newline traversal: %d:%d", line, col)
	}
	d.Move("right", false)
	d.Delete(true)
	if d.Text != "xx\nqz🇨🇳" {
		t.Fatal("newline delete corrupted adjacent emoji")
	}
}
func TestBoundaryCacheIsBoundedAndInvalidatesOnUndo(t *testing.T) {
	d := &Document{Buffer: Buffer{Text: strings.Repeat("e\u0301\n", 200), Anchor: -1}}
	for row := 0; row < 200; row++ {
		d.SetPosition(row, 1)
	}
	if len(d.graphemes) > 128 {
		t.Fatal("boundary cache grew without bound")
	}
	d.SetPosition(0, 0)
	d.Delete(false)
	d.Undo()
	d.SetPosition(0, 1)
	if d.Cursor != 0 {
		t.Fatal("undo reused stale boundaries")
	}
}
