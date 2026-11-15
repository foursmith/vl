package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTreeIsLazyIncludesIgnoredCandidatesAndSkipsGitMetadata(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", "bin", "cmd", "examples/nested"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(root, "README.md"), nil, 0644)
	entries, err := ReadTree(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 || entries[0].Path != "bin" || entries[1].Path != "cmd" || entries[2].Path != "examples" {
		t.Fatalf("unexpected root tree: %+v", entries)
	}
	entries, err = ReadTree(context.Background(), root, map[string]bool{"cmd": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 || entries[2].Path != "cmd/main.go" || entries[2].Depth != 1 {
		t.Fatalf("expanded tree: %+v", entries)
	}
}

func TestCreateDoesNotOverwriteOrEscapeWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := Create(root, "folder", true); err != nil {
		t.Fatal(err)
	}
	if err := Create(root, "folder/new.go", false); err != nil {
		t.Fatal(err)
	}
	if err := Create(root, "folder/new.go", false); err == nil {
		t.Fatal("overwrote existing file")
	}
	for _, path := range []string{"../escape", ".git/config", filepath.Join(outside, "outside")} {
		if err := Create(root, path, false); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err == nil {
		if err = Create(root, "link/outside", false); err == nil {
			t.Fatal("followed symlink outside workspace")
		}
	}
}
