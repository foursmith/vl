package editor

import (
	"os"
	"path/filepath"
	"testing"
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
	if string(d.Bytes()) != original {
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
