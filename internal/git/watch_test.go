package git

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMetadataDirectoriesResolveLinkedWorktree(t *testing.T) {
	r := repository(t)
	write(t, r, "file", "base\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "base")
	dirs, err := r.MetadataDirectories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	realRoot, _ := filepath.EvalSymlinks(r.Root)
	if len(dirs) != 1 || dirs[0] != filepath.Join(realRoot, ".git") {
		t.Fatalf("normal repository directories: %v", dirs)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, r, "worktree", "add", "-b", "feature", linked)
	wt, err := Open(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	dirs, err = wt.MetadataDirectories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 || dirs[1] != filepath.Join(realRoot, ".git") || filepath.Dir(dirs[0]) != filepath.Join(realRoot, ".git", "worktrees") {
		t.Fatalf("linked directories: %v", dirs)
	}
}
