package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFilePositionStoreSharesAliasesAndKeepsFilesIndependent(t *testing.T) {
	dir := t.TempDir()
	store := &FilePositionStore{Dir: filepath.Join(dir, "files")}
	first, second := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	if err := os.WriteFile(first, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.go")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	want := FilePosition{Line: 20, Column: 3, Top: 10, Left: 2}
	if err := store.Save(first, want); err != nil {
		t.Fatal(err)
	}
	record, err := fileStatePath(store.Dir, first)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]int
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("position record contains non-position fields: %s: %v", data, err)
	}
	if len(fields) != 4 || fields["line"] != want.Line || fields["column"] != want.Column || fields["top"] != want.Top || fields["left"] != want.Left {
		t.Fatalf("unexpected position record: %s", data)
	}
	other := &FilePositionStore{Dir: store.Dir}
	if err := other.Save(second, FilePosition{Line: 90}); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := other.Load(alias); err != nil || !ok || !got.Equal(want) {
		t.Fatalf("alias position = %#v, %v, %v", got, ok, err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("per-file records = %d, %v", len(entries), err)
	}
	if _, ok, err := store.Load(filepath.Join(dir, "missing.go")); err != nil || ok {
		t.Fatalf("missing position = %v, %v", ok, err)
	}
}
