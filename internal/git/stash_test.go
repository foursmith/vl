package git

import (
	"context"
	"strings"
	"testing"
)

func TestStashReviewAndRestoreSelectedEntry(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	write(t, r, "tracked.txt", "base\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "base")
	write(t, r, "tracked.txt", "older\n")
	run(t, r, "add", "tracked.txt")
	write(t, r, "untracked[1].txt", "unsaved in Git\n")
	run(t, r, "stash", "push", "-u", "-m", "较早的储藏")
	write(t, r, "tracked.txt", "newer\n")
	run(t, r, "stash", "push", "-m", "newer")

	stashes, err := r.Stashes(ctx)
	if err != nil || len(stashes) != 2 || !strings.HasSuffix(stashes[1].Subject, "较早的储藏") {
		t.Fatalf("stash list: %+v, %v", stashes, err)
	}
	newer, older := stashes[0], stashes[1]
	files, err := r.StashFiles(ctx, older.ID)
	if err != nil || len(files) != 2 || files[0].Path != "tracked.txt" || files[0].Untracked ||
		files[1].Path != "untracked[1].txt" || !files[1].Untracked {
		t.Fatalf("stashed files: %+v, %v", files, err)
	}
	for i, want := range []string{"+older", "+unsaved in Git"} {
		raw, err := r.CommitFileDiff(ctx, files[i].Commit, files[i].Change)
		if err != nil || !strings.Contains(raw, want) {
			t.Fatalf("stash diff for %s: %q, %v", files[i].Path, raw, err)
		}
	}
	if _, err := r.UseStash(ctx, older, "apply", true); err != nil {
		t.Fatal(err)
	}
	if staged := run(t, r, "diff", "--cached"); !strings.Contains(staged, "+older") {
		t.Fatalf("apply --index did not restore staged changes: %s", staged)
	}
	stashes, err = r.Stashes(ctx)
	if err != nil || len(stashes) != 2 {
		t.Fatalf("apply changed stash list: %+v, %v", stashes, err)
	}
	run(t, r, "reset", "--hard", "HEAD")
	run(t, r, "clean", "-f")
	if _, err := r.UseStash(ctx, older, "pop", false); err != nil {
		t.Fatal(err)
	}
	if staged := run(t, r, "diff", "--cached"); staged != "" {
		t.Fatalf("pop without --index staged changes: %s", staged)
	}
	stashes, err = r.Stashes(ctx)
	if err != nil || len(stashes) != 1 || stashes[0].ID != newer.ID {
		t.Fatalf("pop affected the wrong stash: %+v, %v", stashes, err)
	}
	run(t, r, "stash", "push", "-u")
	if _, err := r.UseStash(ctx, newer, "drop", false); err == nil {
		t.Fatal("drop accepted a stale stash slot")
	}
	stashes, err = r.Stashes(ctx)
	if err != nil || len(stashes) != 2 || stashes[1].ID != newer.ID {
		t.Fatalf("stale drop changed stash list: %+v, %v", stashes, err)
	}
	if _, err := r.UseStash(ctx, stashes[1], "drop", false); err != nil {
		t.Fatal(err)
	}
	stashes, err = r.Stashes(ctx)
	if err != nil || len(stashes) != 1 || stashes[0].ID == newer.ID {
		t.Fatalf("drop affected the wrong stash: %+v, %v", stashes, err)
	}
}

func TestStashPopKeepsConflictingEntry(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	write(t, r, "tracked.txt", "base\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "base")
	write(t, r, "tracked.txt", "stashed\n")
	run(t, r, "stash", "push")
	stashes, err := r.Stashes(ctx)
	if err != nil || len(stashes) != 1 {
		t.Fatalf("stash list: %+v, %v", stashes, err)
	}
	stash := stashes[0]
	write(t, r, "tracked.txt", "conflicting commit\n")
	run(t, r, "commit", "-am", "conflict")
	if _, err := r.UseStash(ctx, stash, "pop", false); err == nil {
		t.Fatal("expected a conflict")
	}
	stashes, err = r.Stashes(ctx)
	if err != nil || len(stashes) != 1 || stashes[0].ID != stash.ID || run(t, r, "ls-files", "-u") == "" {
		t.Fatalf("expected the stash and unmerged index to remain: %+v, %v", stashes, err)
	}
}
