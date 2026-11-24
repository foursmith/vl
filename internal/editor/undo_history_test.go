package editor

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foursmith/vl/internal/config"
)

func TestUndoHistoryStoresChangedSpan(t *testing.T) {
	prefix := strings.Repeat("prefix\n", 8<<10)
	suffix := strings.Repeat("suffix\n", 8<<10)
	original := prefix + "éé" + suffix
	path, open := undoHistoryFixture(t, original)
	d := open()
	d.Edit(prefix + "èĩ" + suffix)
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	record, err := config.UndoHistoryPath(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	header := undoFileHeader(d.saved)
	if !bytes.HasPrefix(data, header[:]) || !strings.HasSuffix(record, ".undo") {
		t.Fatalf("expected a binary undo file, got %s", record)
	}
	if len(data) != len(header)+2+binary.Size(undoRecord{})+len("éé")+4 {
		t.Fatalf("unexpected history size: %d bytes", len(data))
	}
	steps := data[len(header):]
	if steps[0] != 1 || steps[1] != 0 {
		t.Fatalf("expected one undo step, got counts %v", steps[:2])
	}
	var change undoRecord
	n, err := binary.Decode(steps[2:], binary.LittleEndian, &change)
	if err != nil || change.Start != uint32(len(prefix)) ||
		change.Remove != uint32(len("èĩ")) || change.InsertBytes != uint32(len("éé")) ||
		string(steps[2+n:len(steps)-4]) != "éé" {
		t.Fatalf("expected one replacement of whole runes, got %+v (%v)", change, err)
	}
	reopened := open()
	reopened.Undo()
	if reopened.Text != original {
		t.Fatal("undo did not reconstruct the original file")
	}
}

func TestUndoHistoryPersistsBothStacks(t *testing.T) {
	path, open := undoHistoryFixture(t, "\xef\xbb\xbf中文\r\n")
	first := open()
	first.SetPosition(0, 1)
	first.BeginEditGroup()
	first.Insert("🙂")
	first.Insert("語")
	first.EndEditGroup()
	first.Edit("first\n")
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}
	second := open()
	second.Undo()
	if second.Text != "中🙂語文\n" || !second.Dirty() || second.Cursor != 3 {
		t.Fatalf("undo after reopening yielded %q at %d", second.Text, second.Cursor)
	}
	if err := second.Save(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "\xef\xbb\xbf中🙂語文\r\n" {
		t.Fatalf("saved undo changed file encoding: %q, %v", data, err)
	}

	third := open()
	third.Redo()
	if third.Text != "first\n" || !third.Dirty() {
		t.Fatalf("redo after reopening yielded %q", third.Text)
	}
	third.Undo()
	if third.Dirty() {
		t.Fatal("undo did not return to the saved version")
	}
	third.Undo()
	if third.Text != "中文\n" || third.Cursor != 1 {
		t.Fatalf("grouped undo after reopening yielded %q at %d", third.Text, third.Cursor)
	}
	third.Redo()
	if third.Dirty() {
		t.Fatal("redo did not return to the saved version")
	}

	fourth := open()
	fourth.Insert("new")
	fourth.Redo()
	if fourth.Text != "new中🙂語文\n" {
		t.Fatalf("new edit did not clear redo: %q", fourth.Text)
	}
	fourth.Undo()
	fourth.Undo()
	if fourth.Text != "中文\n" {
		t.Fatalf("new edit did not preserve the loaded undo stack: %q", fourth.Text)
	}
}

func TestUndoHistoryUsesOnlyMatchingSuccessfulSaves(t *testing.T) {
	path, open := undoHistoryFixture(t, "old\r\n")
	d := open()
	d.Edit("saved\n")
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	// A change in line endings alone invalidates the old history.
	writeUndoTestFile(t, path, "saved\n")
	external := open()
	external.Undo()
	if external.Text != "saved\n" || external.Dirty() {
		t.Fatal("restored history for externally modified disk bytes")
	}
	d.Insert("unsaved")
	if err := d.Save(); err == nil {
		t.Fatal("save unexpectedly overwrote an external edit")
	}
	writeUndoTestFile(t, path, "saved\r\n")
	reopened := open()
	reopened.Undo()
	if reopened.Text != "old\n" {
		t.Fatal("failed save replaced the last successful undo history")
	}
	record, err := config.UndoHistoryPath(path)
	if err != nil {
		t.Fatal(err)
	}
	history, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	history[len(history)-5] ^= 1
	writeUndoTestFile(t, record, string(history))
	reopened = open()
	reopened.Undo()
	if reopened.Text != "saved\n" || reopened.Dirty() {
		t.Fatal("corrupted history changed the document")
	}
	reopened.Edit("recovered\n")
	if err := reopened.Save(); err != nil {
		t.Fatal(err)
	}
	reopened = open()
	reopened.Undo()
	if reopened.Text != "saved\n" {
		t.Fatal("new save did not replace corrupted undo history")
	}
}

func TestWorkspaceEditStartsNewUndoHistory(t *testing.T) {
	path, open := undoHistoryFixture(t, "original\n")
	doc := open()
	doc.Edit("manual\n")
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	record, err := config.UndoHistoryPath(path)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	doc.Edit("unsaved\n")
	doc.Undo()
	if err := doc.SaveWorkspaceEdit("batch\n"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(record); err != nil || !bytes.Equal(data, previous) {
		t.Fatalf("workspace save rewrote persisted history: %v", err)
	}
	doc.Undo()
	doc.Redo()
	if doc.Text != "batch\n" || doc.Dirty() {
		t.Fatalf("workspace edit did not reset undo and redo: %q", doc.Text)
	}
	doc.Edit("later\n")
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	reopened := open()
	reopened.Undo()
	reopened.Undo()
	if reopened.Text != "batch\n" {
		t.Fatalf("manual save serialized history across the workspace edit: %q", reopened.Text)
	}
	reopened.Redo()
	if reopened.Text != "later\n" || reopened.Dirty() {
		t.Fatalf("manual redo should restore the saved edit: %q", reopened.Text)
	}
}

func undoHistoryFixture(t *testing.T, text string) (string, func() *Document) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "file.txt")
	writeUndoTestFile(t, path, text)
	return path, func() *Document {
		t.Helper()
		d, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
}

func writeUndoTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
