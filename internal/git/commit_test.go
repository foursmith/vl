package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCommitReviewRootRenameAndMerge(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	write(t, r, "old\tname.go", "package main\n// original\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "root")
	root := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	files, err := r.CommitFiles(ctx, root)
	if err != nil || len(files) != 1 || files[0].Index != 'A' {
		t.Fatalf("root files: %+v %v", files, err)
	}
	raw, err := r.CommitFileDiff(ctx, root, files[0])
	if err != nil || !strings.Contains(raw, "+package main") {
		t.Fatalf("root patch: %s %v", raw, err)
	}
	run(t, r, "mv", "old\tname.go", "new\nname.go")
	run(t, r, "commit", "-m", "rename")
	id := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	files, err = r.CommitFiles(ctx, id)
	if err != nil || len(files) != 1 || files[0].Index != 'R' || files[0].Original != "old\tname.go" || files[0].Path != "new\nname.go" {
		t.Fatalf("rename: %+v %v", files, err)
	}
	raw, err = r.CommitFileDiff(ctx, id, files[0])
	if err != nil || !strings.Contains(raw, "rename from") {
		t.Fatalf("rename patch: %s %v", raw, err)
	}
	run(t, r, "checkout", "-b", "topic")
	write(t, r, "topic.txt", "topic\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "topic")
	run(t, r, "checkout", "main")
	write(t, r, "main.txt", "main\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "main")
	run(t, r, "merge", "--no-ff", "topic", "-m", "merge")
	id = strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	files, err = r.CommitFiles(ctx, id)
	if err != nil || len(files) != 1 || files[0].Path != "topic.txt" {
		t.Fatalf("first-parent merge: %+v %v", files, err)
	}
	raw, err = r.CommitFileDiff(ctx, id, files[0])
	if err != nil || !strings.Contains(raw, "+topic") || strings.Contains(raw, "+main") {
		t.Fatalf("merge patch: %s %v", raw, err)
	}
}

func TestCommitFileDiffMatchesChangesContext(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	var source strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&source, "line %d\n", i)
	}
	write(t, r, "file.txt", source.String())
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "base")
	modified := strings.Replace(source.String(), "line 10\n", "changed 10\n", 1)
	modified = strings.Replace(modified, "line 60\n", "changed 60\n", 1)
	write(t, r, "file.txt", modified)
	c := Change{Path: "file.txt", Index: ' ', Worktree: 'M'}
	worktree, err := r.Diff(ctx, c, false)
	if err != nil {
		t.Fatal(err)
	}
	run(t, r, "add", ".")
	staged, err := r.Diff(ctx, c, true)
	if err != nil {
		t.Fatal(err)
	}
	run(t, r, "commit", "-m", "changes")
	id := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	committed, err := r.CommitFileDiff(ctx, id, c)
	if err != nil {
		t.Fatal(err)
	}
	if worktree != staged || staged != committed {
		t.Fatal("worktree, index and commit use different patch context")
	}
	if strings.Count(committed, "\n@@ ") != 2 || strings.Contains(committed, " line 30\n") {
		t.Fatal("expected two hunks with bounded context")
	}
}
