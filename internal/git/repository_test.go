package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	patchdiff "github.com/foursmith/vl/internal/diff"
)

func repository(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", out, err)
	}
	r, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config", "user.name", "vl Test"}, {"config", "user.email", "vl@example.test"}, {"config", "commit.gpgsign", "false"}, {"config", "core.autocrlf", "false"}} {
		run(t, r, args...)
	}
	return r
}

func run(t *testing.T, r *Repository, args ...string) string {
	t.Helper()
	out, err := r.Run(context.Background(), args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func write(t *testing.T, r *Repository, path, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Root, path), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeDiffDoesNotWriteIndex(t *testing.T) {
	r := repository(t)
	run(t, r, "config", "diff.autoRefreshIndex", "true")
	const path = "code.go"
	write(t, r, path, "package main\n")
	run(t, r, "add", "--", path)
	run(t, r, "commit", "-m", "initial")
	index := filepath.Join(r.Root, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	changed := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(r.Root, path), changed, changed); err != nil {
		t.Fatal(err)
	}
	if raw, err := r.Diff(context.Background(), Change{Path: path, Index: ' ', Worktree: 'M'}, false); err != nil || raw != "" {
		t.Fatalf("timestamp-only change produced a diff: %q, %v", raw, err)
	}
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("reading the worktree diff rewrote the Git index")
	}
}

func TestWorkflowAndHunkRoundTrip(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	var original strings.Builder
	for i := 0; i < 24; i++ {
		original.WriteString(string(rune('a'+i)) + " original\n")
	}
	write(t, r, "code.go", original.String())
	run(t, r, "add", "--", "code.go")
	run(t, r, "commit", "-m", "initial")
	modified := strings.Replace(original.String(), "b original", "b changed", 1)
	modified = strings.Replace(modified, "v original", "v changed", 1)
	write(t, r, "code.go", modified)
	s, err := r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Changes) != 1 || s.Branch != "main" || len(s.Commits) != 1 || s.Commits[0].Subject != "initial" {
		t.Fatalf("snapshot: %+v", s)
	}
	c := s.Changes[0]
	raw, err := r.Diff(ctx, c, false)
	if err != nil {
		t.Fatal(err)
	}
	patch := patchdiff.Parse(raw)
	if len(patch.Hunks) != 2 {
		t.Fatalf("expected 2 hunks: %s", raw)
	}
	hunk, err := patch.HunkPatch(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ApplyHunk(ctx, hunk, false); err != nil {
		t.Fatal(err)
	}
	index := run(t, r, "show", ":code.go")
	if !strings.Contains(index, "b changed") || strings.Contains(index, "v changed") {
		t.Fatal("staging a hunk changed unrelated lines")
	}
	s, err = r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Changes[0].Staged() || !s.Changes[0].Unstaged() {
		t.Fatal("expected both index and worktree changes")
	}
	raw, err = r.Diff(ctx, s.Changes[0], true)
	if err != nil {
		t.Fatal(err)
	}
	hunk, err = patchdiff.Parse(raw).HunkPatch(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ApplyHunk(ctx, hunk, true); err != nil {
		t.Fatal(err)
	}
	if got := run(t, r, "show", ":code.go"); got != original.String() {
		t.Fatal("unstaging did not restore index")
	}
	if _, err = r.Stage(ctx, c); err != nil {
		t.Fatal(err)
	}
	run(t, r, "commit", "-m", "changed")
	s, err = r.Snapshot(ctx)
	if err != nil || len(s.Changes) != 0 || len(s.Commits) != 2 {
		t.Fatalf("post-commit snapshot: %+v %v", s, err)
	}
	if _, err = r.CommitFileDiff(ctx, s.Commits[0].ID, c); err != nil {
		t.Fatal(err)
	}
	run(t, r, "switch", "-c", "topic")
	run(t, r, "switch", "--", "main")
}

func TestPartialStagingUpdatesOnlySelectedIndexLines(t *testing.T) {
	const path = "文件 [a].txt"
	for _, test := range []struct {
		name, before, after, indexed string
		selectLine                   func(*patchdiff.Line) bool
	}{
		{"adjacent replacements", "a\nb\nc\nd\n", "A\nB\nC\nd\n", "A\nb\nC\nd\n", func(line *patchdiff.Line) bool {
			return line.Old == 1 || line.Old == 3 || line.New == 1 || line.New == 3
		}},
		{"separate hunks", "a\n" + strings.Repeat("context\n", 10) + "z\n", "first\na\n" + strings.Repeat("context\n", 10) + "Z\n", "first\na\n" + strings.Repeat("context\n", 10) + "Z\n", func(*patchdiff.Line) bool {
			return true
		}},
		{"end of file", "context\nold", "context\nnew", "context\nnew", func(*patchdiff.Line) bool { return true }},
		{"added side at end of file", "context\nold", "context\nnew", "context\nold\nnew", func(line *patchdiff.Line) bool { return line.Kind == patchdiff.Added }},
		{"new file", "", "first\nsecond\nthird", "second\nthird", func(line *patchdiff.Line) bool { return line.New > 1 }},
		{"deleted file", "first\nsecond\nthird", "", "first\nthird", func(line *patchdiff.Line) bool { return line.Old == 2 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := repository(t)
			ctx := context.Background()
			if test.before != "" {
				write(t, r, path, test.before)
				run(t, r, "add", "--", path)
			}
			if test.before != "" {
				run(t, r, "commit", "-m", "initial")
			}
			if test.after != "" {
				write(t, r, path, test.after)
			} else if err := os.Remove(filepath.Join(r.Root, path)); err != nil {
				t.Fatal(err)
			}
			apply := func(reverse bool, selected func(*patchdiff.Line) bool) {
				t.Helper()
				snapshot, err := r.Snapshot(ctx)
				if err != nil || len(snapshot.Changes) != 1 {
					t.Fatalf("status: %+v, %v", snapshot.Changes, err)
				}
				raw, err := r.Diff(ctx, snapshot.Changes[0], reverse)
				if err != nil {
					t.Fatal(err)
				}
				patch, err := patchdiff.Parse(raw).StagingPatch(path, reverse, selected)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := r.ApplyHunk(ctx, patch, false); err != nil {
					t.Fatalf("apply: %v\n%s", err, patch)
				}
			}
			apply(false, test.selectLine)
			if got := run(t, r, "show", ":"+path); got != test.indexed {
				t.Fatalf("index = %q, want %q", got, test.indexed)
			}
			apply(true, func(*patchdiff.Line) bool { return true })
			if test.before == "" {
				if got := run(t, r, "ls-files", "--", path); got != "" {
					t.Fatalf("new file still in index: %q", got)
				}
				apply(false, test.selectLine)
				if _, err := r.Unstage(ctx, Change{Path: path}); err != nil {
					t.Fatal(err)
				}
				if got := run(t, r, "ls-files", "--", path); got != "" {
					t.Fatalf("whole-file unstage left an index entry: %q", got)
				}
			} else if got := run(t, r, "show", ":"+path); got != test.before {
				t.Fatalf("unstaged index = %q, want %q", got, test.before)
			}
			contents, err := os.ReadFile(filepath.Join(r.Root, path))
			if test.after == "" && !os.IsNotExist(err) || test.after != "" && (err != nil || string(contents) != test.after) {
				t.Fatalf("working file changed: %q, %v", contents, err)
			}
		})
	}
}

func TestLiteralPathsRenameAndUnbornRepository(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	name := "odd\n文件[1].txt"
	other := "odd\n文件1.txt"
	write(t, r, name, "one\n")
	write(t, r, other, "two\n")
	s, err := r.Snapshot(ctx)
	if err != nil || len(s.Changes) != 2 {
		t.Fatalf("unborn status: %+v %v", s, err)
	}
	var c Change
	for _, change := range s.Changes {
		if change.Path == name {
			c = change
		}
	}
	raw, err := r.Diff(ctx, c, false)
	if err != nil || !strings.Contains(raw, "+one") {
		t.Fatalf("untracked diff: %s %v", raw, err)
	}
	if _, err = r.Stage(ctx, c); err != nil {
		t.Fatal(err)
	}
	staged := run(t, r, "diff", "--cached", "--name-only", "-z")
	if staged != name+"\x00" {
		t.Fatalf("pathspec was not literal: %q", staged)
	}
	if _, err = r.Unstage(ctx, Change{Path: name, Index: 'A', Worktree: ' '}); err != nil {
		t.Fatal(err)
	}
	if got := run(t, r, "diff", "--cached", "--name-only", "-z"); got != "" {
		t.Fatalf("unborn unstage: %q", got)
	}
	run(t, r, "add", "--all")
	run(t, r, "commit", "-m", "initial")
	run(t, r, "mv", "--", name, "renamed.txt")
	s, err = r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Changes) != 1 || s.Changes[0].Original != name || s.Changes[0].Path != "renamed.txt" {
		t.Fatalf("rename parse: %+v", s.Changes)
	}
	if _, err = r.Unstage(ctx, s.Changes[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStageRenameThenDelete(t *testing.T) {
	r := committedRepository(t)
	ctx := context.Background()
	oldPath := "file.txt"
	newPath := "renamed.txt"
	if err := os.Rename(filepath.Join(r.Root, oldPath), filepath.Join(r.Root, newPath)); err != nil {
		t.Fatal(err)
	}
	// Intent to add lets Git detect the rename in the worktree.
	run(t, r, "add", "-N", "--", newPath)
	stage := func(index, worktree byte) {
		t.Helper()
		raw := run(t, r, "status", "--porcelain=v1", "-z", "--renames")
		changes, err := ParseStatus(raw)
		want := Change{Path: newPath, Original: oldPath, Index: index, Worktree: worktree}
		if err != nil || len(changes) != 1 || changes[0] != want {
			t.Fatalf("status = %+v, %v, want %+v", changes, err, want)
		}
		if _, err := r.Stage(ctx, changes[0]); err != nil {
			t.Fatal(err)
		}
	}
	stage(' ', 'R')
	if err := os.Remove(filepath.Join(r.Root, newPath)); err != nil {
		t.Fatal(err)
	}
	stage('R', 'D')
	if got := run(t, r, "status", "--porcelain=v1", "-z"); got != "D  "+oldPath+"\x00" {
		t.Fatalf("status after staging deletion = %q", got)
	}
}

func TestStatusParserAndObjectValidation(t *testing.T) {
	for _, raw := range []string{"x\x00", "R  file\x00"} {
		if _, err := ParseStatus(raw); err == nil {
			t.Fatalf("accepted malformed status %q", raw)
		}
	}
	if validObjectID("--all") || validObjectID(strings.Repeat("z", 40)) {
		t.Fatal("accepted invalid revision")
	}
	if !validObjectID(strings.Repeat("a", 40)) {
		t.Fatal("rejected object ID")
	}
}

func TestIgnoredPathsAreLiteralAndNULDelimited(t *testing.T) {
	r := repository(t)
	for _, dir := range []string{"bin", "examples"} {
		if err := os.Mkdir(filepath.Join(r.Root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, r, ".gitignore", "bin/\nexamples/\nodd?name\n")
	paths := []string{"bin", "examples", "cmd", "odd\nname"}
	ignored, err := r.Ignored(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if !ignored["bin"] || !ignored["examples"] || ignored["cmd"] {
		t.Fatalf("ignored paths: %+v", ignored)
	}
	ignored, err = r.Ignored(context.Background(), []string{"visible.go"})
	if err != nil || len(ignored) != 0 {
		t.Fatalf("unignored paths: %+v %v", ignored, err)
	}
}
