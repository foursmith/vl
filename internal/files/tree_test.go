package files

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
	if len(entries) != 4 || entries[0].Path != "bin" || entries[1].Path != "cmd" || entries[2].Path != "examples/nested" {
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

func TestTreeCompactsOnlySingleDirectoryChains(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"apps/web/src/hooks", "apps/web/src/components", "mixed/child", "empty", "metadata/.git", "metadata/child"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"apps/web/src/hooks/use.go", "mixed/file.go"} {
		if err := os.WriteFile(filepath.Join(root, path), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadTree(context.Background(), root, map[string]bool{"apps/web/src": true, "apps/web/src/hooks": true})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Path: "apps/web/src", Label: "apps/web/src", Directory: true},
		{Path: "apps/web/src/components", Label: "components", Directory: true, Depth: 1},
		{Path: "apps/web/src/hooks", Label: "hooks", Directory: true, Depth: 1},
		{Path: "apps/web/src/hooks/use.go", Label: "use.go", Depth: 2},
		{Path: "empty", Label: "empty", Directory: true},
		{Path: "metadata/child", Label: "metadata/child", Directory: true},
		{Path: "mixed", Label: "mixed", Directory: true},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("tree: %+v", entries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadTree(ctx, root, nil); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestListFolderExcludesMetadataAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"hello.txt", "nested/.git/config"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	paths, err := List(context.Background(), root)
	if err != nil || len(paths) != 1 || paths[0] != "hello.txt" {
		t.Fatalf("folder list: %v, %v", paths, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := List(ctx, root); err == nil {
		t.Fatal("cancelled listing succeeded")
	}
}

func TestDeleteOnlyRemovesWorkspaceFiles(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(root, "folder"), 0755)
	os.Mkdir(filepath.Join(root, ".git"), 0755)
	os.WriteFile(filepath.Join(root, ".git/config"), nil, 0644)
	os.WriteFile(filepath.Join(outside, "keep"), nil, 0644)
	os.Symlink(outside, filepath.Join(root, "link"))
	for _, path := range []string{".", "../keep", outside, ".git/config", "folder", "link/keep"} {
		if err := Delete(root, path); err == nil {
			t.Fatalf("accepted deletion of %q", path)
		}
	}
	if err := Delete(root, "link"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "keep")); err != nil {
		t.Fatal("deleted symlink target")
	}
	os.WriteFile(filepath.Join(root, "file"), nil, 0644)
	if err := Delete(root, "file"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "file")); !os.IsNotExist(err) {
		t.Fatalf("file was not removed: %v", err)
	}
}
