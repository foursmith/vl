package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	patchdiff "github.com/ourongxing/vl/internal/diff"
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
