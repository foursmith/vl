package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFolder(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, file} {
		r, err := Open(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		s, err := r.Snapshot(ctx)
		if err != nil || !s.NoRepository || r.Root != real || len(s.Files) != 1 || s.Files[0] != "hello.txt" {
			t.Fatalf("folder snapshot: %+v, %v", s, err)
		}
	}
	if _, err := Open(ctx, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
	for _, directory := range []bool{false, true} {
		marker := filepath.Join(dir, ".git")
		if directory {
			if err := os.Mkdir(marker, 0755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(marker, []byte("broken metadata"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(ctx, dir); err == nil {
			t.Fatal("broken repository treated as folder")
		}
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Open(ctx, dir); err == nil {
		t.Fatal("cancelled open succeeded")
	}
}

func TestOpenPreservesRepositoryRoot(t *testing.T) {
	repo := repository(t)
	sub := filepath.Join(repo.Root, "nested")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	r, err := Open(context.Background(), sub)
	if err != nil || r.Root != repo.Root || !r.HasRepository() {
		t.Fatalf("repository discovery: %+v, %v", r, err)
	}
}

func TestOpenOrdinaryFolderWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Snapshot(t.Context())
	if err != nil || !snapshot.NoRepository {
		t.Fatalf("Gitless folder status: %+v, %v", snapshot, err)
	}
}
