package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRoundTripBOMCRLFTabsUnicodeAndUndo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.go")
	original := "\xef\xbb\xbfpackage main\r\n\t// 中文 👋\r\n"
	if err := os.WriteFile(path, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.Dirty() {
		t.Fatal("opening dirtied document")
	}
	if string(d.encode(d.Text)) != original {
		t.Fatal("open changed document bytes")
	}
	d.SetPosition(1, 1)
	d.Insert("测试")
	d.Undo()
	if d.Dirty() {
		t.Fatal("undo did not return to saved state")
	}
	d.Redo()
	if err = d.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "\xef\xbb\xbfpackage main\r\n\t测试// 中文 👋\r\n" {
		t.Fatalf("save corrupted bytes: %q", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0755 {
		t.Fatal("save lost file mode")
	}
}

func TestSaveDetectsExternalEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	os.WriteFile(path, []byte("before\n"), 0644)
	d, _ := Open(path)
	d.Insert("mine")
	os.WriteFile(path, []byte("external\n"), 0644)
	if err := d.Save(); err == nil {
		t.Fatal("overwrote external edit")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "external\n" {
		t.Fatal("external edit lost")
	}
}

func TestSelectionMultilineAndCursor(t *testing.T) {
	d := &Document{Text: "ab\n中文\nxyz", Anchor: -1}
	d.SetPosition(1, 1)
	d.Move("right", true)
	if d.SelectedText() != "文" {
		t.Fatalf("selection: %q", d.SelectedText())
	}
	d.Insert("🙂\n")
	if d.Text != "ab\n中🙂\n\nxyz" {
		t.Fatalf("insert: %q", d.Text)
	}
	d.Delete(true)
	if d.Text != "ab\n中🙂\nxyz" {
		t.Fatalf("backspace: %q", d.Text)
	}
	d.Undo()
	d.Undo()
	if d.Text != "ab\n中文\nxyz" {
		t.Fatal("undo sequence")
	}
	d.SelectAll()
	d.Insert("")
	if d.Text != "" || d.Cursor != 0 {
		t.Fatal("select all delete")
	}
	d.Move("left", false)
	d.Delete(true)
	d.Move("down", false)
}

func TestRejectBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binary")
	os.WriteFile(path, []byte{0, 1, 2}, 0644)
	if _, err := Open(path); err == nil {
		t.Fatal("accepted binary")
	}
}

func TestRejectMixedLineEndingsWithoutRewriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed")
	original := []byte("one\r\ntwo\n")
	os.WriteFile(path, original, 0644)
	if _, err := Open(path); err == nil {
		t.Fatal("accepted mixed line endings")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("opening changed bytes")
	}
}

func TestLineIndexTracksUnicodeEditsUndoRedoAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.txt")
	if err := os.WriteFile(path, []byte("ab\n中文🙂\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func(text string) {
		t.Helper()
		runes := []rune(text)
		for cursor := 0; cursor <= len(runes); cursor++ {
			d.Cursor = cursor
			line, col := 0, 0
			for _, r := range runes[:cursor] {
				if r == '\n' {
					line++
					col = 0
				} else {
					col++
				}
			}
			gotLine, gotCol := d.Position()
			if gotLine != line || gotCol != col {
				t.Fatalf("cursor %d: got %d:%d, want %d:%d", cursor, gotLine, gotCol, line, col)
			}
			d.SetPosition(line, col)
			if d.Cursor != cursor {
				t.Fatal("line offset did not round-trip cursor")
			}
		}
	}
	check(d.Text)
	d.SetPosition(1, 1)
	d.Insert("test\n")
	check(d.Text)
	if !d.Dirty() {
		t.Fatal("edit did not mark dirty")
	}
	d.Undo()
	check(d.Text)
	if d.Dirty() {
		t.Fatal("undo did not restore saved state")
	}
	d.Redo()
	check(d.Text)
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if d.Dirty() {
		t.Fatal("save did not update clean state")
	}
	d.BOM = true
	if !d.Dirty() {
		t.Fatal("encoding change was ignored")
	}
	d.BOM = false
	d.CRLF = true
	if !d.Dirty() {
		t.Fatal("line ending change was ignored")
	}
}

func TestByteOffsetsAndWordMovementMatchRuneCoordinates(t *testing.T) {
	d := &Document{Text: "ab 中🙂文\n  next\tword", Anchor: -1}
	for cursor := 0; cursor <= utf8.RuneCountInString(d.Text); cursor++ {
		want := len(string([]rune(d.Text)[:cursor]))
		if got := d.ByteOffset(cursor); got != want {
			t.Fatalf("cursor %d byte offset %d, want %d", cursor, got, want)
		}
	}
	d.Move("ctrl+end", false)
	d.Move("ctrl+left", false)
	if d.Cursor != 14 {
		t.Fatalf("word-left stopped at %d", d.Cursor)
	}
	d.Move("ctrl+left", false)
	d.Move("ctrl+left", false)
	if d.Cursor != 3 {
		t.Fatalf("unicode word-left stopped at %d", d.Cursor)
	}
	d.Move("ctrl+right", false)
	if d.Cursor != 9 {
		t.Fatalf("unicode word-right stopped at %d", d.Cursor)
	}
	d.Delete(false)
	d.Undo()
	if d.Text != "ab 中🙂文\n  next\tword" {
		t.Fatal("byte-based deletion did not round trip")
	}
}

func TestUndoHistoryStaysBoundedAndRedoSurvivesTrimming(t *testing.T) {
	d := &Document{Anchor: -1}
	for i := 0; i < 200; i++ {
		d.Edit(fmt.Sprintf("edit %d", i))
	}
	latest := d.Text
	steps := 0
	for len(d.undo) > 0 {
		d.Undo()
		steps++
	}
	if steps != 128 {
		t.Fatalf("undo retained %d steps, want 128", steps)
	}
	for len(d.redo) > 0 {
		d.Redo()
	}
	if d.Text != latest {
		t.Fatal("redo lost latest text after history trimming")
	}
	large := strings.Repeat("x", 1<<20)
	for i := 0; i < 20; i++ {
		d.Edit(fmt.Sprintf("%d%s", i, large))
	}
	bytes := 0
	for _, state := range d.undo {
		bytes += len(state.text)
	}
	if bytes > 8<<20 {
		t.Fatalf("undo history exceeded byte budget: %d", bytes)
	}
}
