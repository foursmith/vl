package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	patchdiff "github.com/ourongxing/elegit/internal/diff"
)

func browsingWorkspace(t *testing.T) (*Repository, *Repository, *Repository) {
	t.Helper()
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	leaf, source, parent := repository(t), repository(t), repository(t)
	for _, repo := range []*Repository{leaf, source, parent} {
		write(t, repo, "file.txt", "original\n")
		run(t, repo, "add", ".")
		run(t, repo, "commit", "-m", "initial")
	}
	run(t, source, "submodule", "add", leaf.Root, "nested")
	run(t, source, "commit", "-am", "nested")
	run(t, parent, "submodule", "add", source.Root, "child space")
	run(t, parent, "submodule", "update", "--init", "--recursive")
	run(t, parent, "commit", "-am", "child")
	child, err := parent.RepositoryAt(context.Background(), "child space")
	if err != nil {
		t.Fatal(err)
	}
	nested, err := parent.RepositoryAt(context.Background(), "child space/nested")
	if err != nil {
		t.Fatal(err)
	}
	for _, repo := range []*Repository{child, nested} {
		run(t, repo, "config", "user.name", "Test")
		run(t, repo, "config", "user.email", "test@example.test")
		run(t, repo, "config", "commit.gpgsign", "false")
	}
	return parent, child, nested
}

func snapshot(t *testing.T, r *Repository) Snapshot {
	t.Helper()
	s, err := r.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func changeAt(t *testing.T, s Snapshot, path string) Change {
	t.Helper()
	for _, c := range s.Changes {
		if c.Path == path {
			return c
		}
	}
	t.Fatalf("missing change %q: %+v", path, s.Changes)
	return Change{}
}

func TestWorkspaceNestedFilesAndOwningRepositories(t *testing.T) {
	root, child, nested := browsingWorkspace(t)
	write(t, child, "file.txt", "child edit\n")
	write(t, child, "new file.txt", "new child file\n")
	write(t, nested, "file.txt", "nested edit\n")
	s := snapshot(t, root)
	summary := changeAt(t, s, "child space")
	if !summary.Gitlink || !summary.GitlinkDirty || summary.Repository != "" {
		t.Fatalf("parent summary: %+v", summary)
	}
	if state := s.RepositoryStates["child space"]; state.Branch != "main" || state.Detached || state.Head == "" || len(state.Remotes) == 0 {
		t.Fatalf("child status metadata: %+v", state)
	}
	if state := s.RepositoryStates["child space/nested"]; !state.Detached || state.Head == "" {
		t.Fatalf("nested status metadata: %+v", state)
	}
	c := changeAt(t, s, "child space/file.txt")
	if c.Repository != "child space" || c.Gitlink {
		t.Fatalf("owner: %+v", c)
	}
	n := changeAt(t, s, "child space/nested/file.txt")
	if n.Repository != "child space/nested" {
		t.Fatalf("nested owner: %+v", n)
	}
	ctx := context.Background()
	raw, err := root.Diff(ctx, c, false)
	if err != nil || !strings.Contains(raw, "+child edit") {
		t.Fatalf("child diff: %q %v", raw, err)
	}
	indexBefore := run(t, root, "ls-files", "--stage", "--", "child space")
	hunk, err := patchdiff.Parse(raw).HunkPatch(0)
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := root.ForChange(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyHunk(ctx, hunk, false); err != nil {
		t.Fatal(err)
	}
	if got := run(t, child, "show", ":file.txt"); got != "child edit\n" {
		t.Fatal(got)
	}
	if _, err := root.Unstage(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stage(ctx, n); err != nil {
		t.Fatal(err)
	}
	if got := run(t, nested, "show", ":file.txt"); got != "nested edit\n" {
		t.Fatal(got)
	}
	if got := run(t, root, "ls-files", "--stage", "--", "child space"); got != indexBefore {
		t.Fatal("child staging changed parent gitlink")
	}
}

func TestWorkspaceIgnoreRulesBelongToEachRepository(t *testing.T) {
	root, child, _ := browsingWorkspace(t)
	write(t, child, ".gitignore", "*.cache\nfile.txt\n")
	write(t, child, "output.cache", "ignored")
	metadata, err := child.MetadataDirectories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata[0], "info", "exclude"), []byte("local.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	write(t, child, "local.log", "ignored locally")
	paths := []string{"file.txt", "child space", "child space/file.txt", "child space/output.cache", "child space/local.log", "child space/nested/file.txt"}
	ignored, err := root.Ignored(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if !ignored["child space/output.cache"] || !ignored["child space/local.log"] {
		t.Fatalf("child ignore rules: %v", ignored)
	}
	for _, path := range []string{"file.txt", "child space", "child space/file.txt", "child space/nested/file.txt"} {
		if ignored[path] {
			t.Fatalf("tracked file marked ignored: %q", path)
		}
	}
}

func TestWorkspaceDiscardAndStaleOwnership(t *testing.T) {
	root, child, _ := browsingWorkspace(t)
	ctx := context.Background()
	write(t, root, "file.txt", "parent edit\n")
	write(t, child, "file.txt", "child edit\n")
	c := changeAt(t, snapshot(t, root), "child space/file.txt")
	if _, err := root.Discard(ctx, c); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(filepath.Join(child.Root, "file.txt")); string(text) != "original\n" {
		t.Fatal("child was not restored")
	}
	if text, _ := os.ReadFile(filepath.Join(root.Root, "file.txt")); string(text) != "parent edit\n" {
		t.Fatal("child discard affected parent")
	}
	if err := os.Remove(filepath.Join(child.Root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stage(ctx, c); err == nil {
		t.Fatal("stale child ownership fell back to parent")
	}
	if _, _, err := root.ForChange(ctx, Change{Repository: "../outside", Path: "file.txt"}); err == nil {
		t.Fatal("outside repository accepted")
	}
}

func TestWorkspaceKeepsParentCommitPointerSeparate(t *testing.T) {
	root, child, _ := browsingWorkspace(t)
	ctx := context.Background()
	write(t, child, "file.txt", "committed in child\n")
	run(t, child, "commit", "-am", "child change")
	s := snapshot(t, root)
	c := changeAt(t, s, "child space")
	if c.Repository != "" || !c.Gitlink {
		t.Fatalf("pointer owner: %+v", c)
	}
	raw, err := root.Diff(ctx, c, false)
	if err != nil || !strings.Contains(raw, "Subproject commit") {
		t.Fatalf("%q %v", raw, err)
	}
	if _, err := patchdiff.Parse(raw).HunkPatch(0); err == nil {
		t.Fatal("pointer allowed partial staging")
	}
	if _, err := root.Discard(ctx, c); err == nil {
		t.Fatal("directory discarded as a file")
	}
	if _, err := root.Stage(ctx, c); err != nil {
		t.Fatal(err)
	}
	if !changeAt(t, snapshot(t, root), "child space").Staged() {
		t.Fatal("parent pointer not staged")
	}
	if _, err := root.Unstage(ctx, c); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceUninitializedChildIsNotAFileOrParent(t *testing.T) {
	root, _, _ := browsingWorkspace(t)
	run(t, root, "submodule", "deinit", "--", "child space")
	s := snapshot(t, root)
	if len(s.Changes) != 0 || len(s.Repositories) != 0 {
		t.Fatalf("uninitialized child resolved to parent: %+v", s)
	}
	if _, err := root.RepositoryAt(context.Background(), "child space"); err == nil {
		t.Fatal("uninitialized child opened parent")
	}
}

func TestWorkspaceChildRenamePathsAndUntrackedDiscard(t *testing.T) {
	root, child, _ := browsingWorkspace(t)
	ctx := context.Background()
	run(t, child, "mv", "file.txt", "renamed\tfile.txt")
	c := changeAt(t, snapshot(t, root), "child space/renamed\tfile.txt")
	if c.Original != "child space/file.txt" || c.Repository != "child space" {
		t.Fatalf("rename: %+v", c)
	}
	if _, err := root.Unstage(ctx, c); err != nil {
		t.Fatal(err)
	}
	write(t, child, "untracked.txt", "remove me")
	c = changeAt(t, snapshot(t, root), "child space/untracked.txt")
	if _, err := root.Discard(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(child.Root, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked child file remains")
	}
}

func TestWorkspaceIncludesIndependentCommitHistories(t *testing.T) {
	root, child, nested := browsingWorkspace(t)
	write(t, child, "file.txt", "child history\n")
	run(t, child, "commit", "-am", "child history change")
	run(t, child, "tag", "child-tag")
	run(t, root, "tag", "root-tag")
	s := snapshot(t, root)
	history := s.RepositoryStates["child space"]
	if len(history.Commits) == 0 || history.Commits[0].Subject != "child history change" {
		t.Fatalf("child history: %+v", history)
	}
	if s.Commits[0].Subject != "child" {
		t.Fatal("child commit changed root log")
	}
	seenTag := false
	for _, ref := range history.Refs {
		if ref.Name == "root-tag" {
			t.Fatal("root ref leaked into child")
		}
		seenTag = seenTag || ref.Name == "child-tag"
	}
	if !seenTag {
		t.Fatal("child tag missing")
	}
	nestedHistory := s.RepositoryStates["child space/nested"]
	if len(nestedHistory.Commits) == 0 || nestedHistory.Commits[0].ID != strings.TrimSpace(run(t, nested, "rev-parse", "HEAD")) {
		t.Fatal("nested history missing")
	}
	id := history.Commits[0].ID
	if _, err := root.Run(context.Background(), "cat-file", "-e", id); err == nil {
		t.Fatal("fixture child commit exists in root")
	}
	raw, err := root.CommitFileDiff(context.Background(), id, Change{Repository: "child space", Path: "child space/file.txt"})
	if err != nil || !strings.Contains(raw, "+child history") {
		t.Fatalf("child history diff routed to root: %q %v", raw, err)
	}
}

func TestWorkspaceHistoryRenameDiffUsesChildPaths(t *testing.T) {
	root, child, _ := browsingWorkspace(t)
	run(t, child, "mv", "file.txt", "renamed\tfile.txt")
	run(t, child, "commit", "-m", "rename")
	id := strings.TrimSpace(run(t, child, "rev-parse", "HEAD"))
	raw, err := root.CommitFileDiff(context.Background(), id, Change{Repository: "child space", Path: "child space/renamed\tfile.txt", Original: "child space/file.txt", Index: 'R', Worktree: ' '})
	if err != nil || !strings.Contains(raw, "rename from file.txt") {
		t.Fatalf("child rename diff: %q %v", raw, err)
	}
}
