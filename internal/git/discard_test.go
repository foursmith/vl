package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscardTrackedRestoresIndexWithoutLosingStagedContent(t *testing.T) {
	r := committedRepository(t)
	ctx := context.Background()
	write(t, r, "file.txt", "staged\n")
	run(t, r, "add", "file.txt")
	write(t, r, "file.txt", "unstaged\n")
	s, err := r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Discard(ctx, s.Changes[0]); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "file.txt"))
	if string(data) != "staged\n" || run(t, r, "show", ":file.txt") != "staged\n" {
		t.Fatal("discard lost index content")
	}
}

func TestDiscardUntrackedRemovesOnlyExactFileAndRejectsChangedStatus(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	name := "new[1]\n.txt"
	other := "new1\n.txt"
	write(t, r, name, "one\n")
	write(t, r, other, "two\n")
	c := Change{Path: name, Index: '?', Worktree: '?'}
	if _, err := r.Discard(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, name)); !os.IsNotExist(err) {
		t.Fatal("exact file was not removed")
	}
	if _, err := os.Stat(filepath.Join(r.Root, other)); err != nil {
		t.Fatal("discard matched a different filename")
	}
	write(t, r, name, "new content\n")
	run(t, r, "add", "--", name)
	if _, err := r.Discard(ctx, c); err == nil {
		t.Fatal("discard accepted stale untracked status")
	}
	if _, err := os.Stat(filepath.Join(r.Root, name)); err != nil {
		t.Fatal("tracked file removed by stale untracked request")
	}
	os.Mkdir(filepath.Join(r.Root, "folder"), 0755)
	write(t, r, "folder/child", "keep\n")
	if _, err := r.Discard(ctx, Change{Path: "folder", Index: '?', Worktree: '?'}); err == nil {
		t.Fatal("discard accepted an entire directory")
	}
}

func TestDiscardUntrackedSymlinkKeepsTarget(t *testing.T) {
	r := repository(t)
	outside := filepath.Join(t.TempDir(), "target")
	os.WriteFile(outside, []byte("keep"), 0644)
	if err := os.Symlink(outside, filepath.Join(r.Root, "link")); err != nil {
		t.Skip(err)
	}
	if _, err := r.Discard(context.Background(), Change{Path: "link", Index: '?', Worktree: '?'}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep" {
		t.Fatal("discard followed symlink target")
	}
}

func TestDiscardTrackedPathReplacedByDirectoryKeepsChildren(t *testing.T) {
	r := committedRepository(t)
	path := filepath.Join(r.Root, "file.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, r, "file.txt/child", "keep\n")
	if _, err := r.Discard(context.Background(), Change{Path: "file.txt", Index: ' ', Worktree: 'D'}); err == nil {
		t.Fatal("discard accepted a directory replacing a tracked file")
	}
	if data, err := os.ReadFile(filepath.Join(path, "child")); err != nil || string(data) != "keep\n" {
		t.Fatal("discard changed directory contents")
	}
}
