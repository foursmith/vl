package git

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findRef(t *testing.T, s Snapshot, name string, kind RefKind) Ref {
	t.Helper()
	for _, ref := range s.Refs {
		if ref.Name == name && ref.Kind == kind {
			return ref
		}
	}
	t.Fatalf("missing reference %s (%s): %+v", name, kind, s.Refs)
	return Ref{}
}

func committedRepository(t *testing.T) *Repository {
	t.Helper()
	r := repository(t)
	write(t, r, "file.txt", "initial\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "initial")
	return r
}

func TestReferenceCheckoutTrackingAndAnnotatedTags(t *testing.T) {
	r := committedRepository(t)
	ctx := context.Background()
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	run(t, r, "remote", "add", "origin", bare)
	if _, err := r.Publish(ctx, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	run(t, r, "push", "origin", "HEAD:refs/heads/topic")
	run(t, r, "fetch", "origin")
	run(t, r, "tag", "-a", "v1", "-m", "release")
	s, err := r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Upstream != "origin/main" || !s.TrackingKnown || s.Ahead != 0 || s.Behind != 0 || s.Detached {
		t.Fatalf("tracking state: %+v", s)
	}
	remote := findRef(t, s, "origin/topic", RemoteBranch)
	if _, err = r.Switch(ctx, remote); err != nil {
		t.Fatal(err)
	}
	s, err = r.Snapshot(ctx)
	if err != nil || s.Branch != "topic" || s.Upstream != "origin/topic" {
		t.Fatalf("remote checkout: %+v %v", s, err)
	}
	if _, err = r.Switch(ctx, remote); err == nil {
		t.Fatal("reused an existing local branch for a remote checkout")
	}
	if _, err = r.Switch(ctx, findRef(t, s, "v1", Tag)); err != nil {
		t.Fatal(err)
	}
	s, err = r.Snapshot(ctx)
	if err != nil || !s.Detached {
		t.Fatalf("tag checkout: %+v %v", s, err)
	}
	if _, err = r.CreateBranch(ctx, "from-tag", findRef(t, s, "v1", Tag).OID); err != nil {
		t.Fatal(err)
	}
	s, err = r.Snapshot(ctx)
	if err != nil || s.Branch != "from-tag" || s.Detached {
		t.Fatal("create from tag failed", err)
	}
	if _, err = r.CreateBranch(ctx, "--bad", ""); err == nil {
		t.Fatal("accepted invalid branch name")
	}
}

func TestAheadBehindSyncAndStaleReference(t *testing.T) {
	r := committedRepository(t)
	ctx := context.Background()
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	run(t, r, "remote", "add", "origin", bare)
	if _, err := r.Publish(ctx, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	s, err := r.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	old := findRef(t, s, "main", LocalBranch)
	write(t, r, "file.txt", "second\n")
	run(t, r, "commit", "-am", "second")
	s, err = r.Snapshot(ctx)
	if err != nil || s.Ahead != 1 || s.Behind != 0 {
		t.Fatalf("ahead: %+v %v", s, err)
	}
	if _, err = r.Switch(ctx, old); err == nil {
		t.Fatal("accepted stale reference")
	}
	if _, err = r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	run(t, r, "reset", "--hard", "HEAD~1")
	s, err = r.Snapshot(ctx)
	if err != nil || s.Ahead != 0 || s.Behind != 1 {
		t.Fatalf("behind: %+v %v", s, err)
	}
	if _, err = r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	s, err = r.Snapshot(ctx)
	if err != nil || s.Ahead != 0 || s.Behind != 0 {
		t.Fatalf("synced: %+v %v", s, err)
	}
	remoteHead := s.Head
	run(t, r, "reset", "--hard", "HEAD~1")
	write(t, r, "file.txt", "divergent\n")
	run(t, r, "commit", "-am", "divergent")
	localHead := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	if _, err = r.Sync(ctx); err == nil {
		t.Fatal("sync merged divergent history instead of requiring fast-forward")
	}
	if got := strings.TrimSpace(run(t, r, "rev-parse", "HEAD")); got != localHead {
		t.Fatal("failed sync changed local branch")
	}
	out, err := exec.Command("git", "--git-dir", bare, "rev-parse", "refs/heads/main").Output()
	if err != nil || strings.TrimSpace(string(out)) != remoteHead {
		t.Fatal("failed pull was followed by a push")
	}
}

func TestGitHubRemoteNormalizationAndPreference(t *testing.T) {
	for _, input := range []string{"git@github.com:owner/repo.git", "https://github.com/owner/repo.git", "ssh://git@github.com/owner/repo.git", "https://user:secret@github.com/owner/repo.git?query=secret#fragment"} {
		got, err := GitHubRepositoryURL(input)
		if err != nil || got != "https://github.com/owner/repo" {
			t.Fatalf("normalize %q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{"/local/repo", "https://gitlab.com/owner/repo", "javascript://github.com/owner/repo", "https://github.com/owner/../repo", "ssh://git@github.com/owner/repo/extra"} {
		if _, err := GitHubRepositoryURL(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	r := committedRepository(t)
	run(t, r, "remote", "add", "fork", "git@github.com:other/fork.git")
	run(t, r, "remote", "add", "origin", "https://github.com/owner/repo.git")
	s, err := r.Snapshot(context.Background())
	if err != nil || s.GitHubURL != "https://github.com/owner/repo" {
		t.Fatalf("origin preference: %+v %v", s, err)
	}
	head := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	run(t, r, "update-ref", "refs/remotes/fork/main", head)
	run(t, r, "branch", "--set-upstream-to=fork/main", "main")
	s, err = r.Snapshot(context.Background())
	if err != nil || s.GitHubURL != "https://github.com/other/fork" {
		t.Fatalf("upstream preference: %+v %v", s, err)
	}
}

func TestSyncPushesOnlySelectedBranchWithMatchingPushDefault(t *testing.T) {
	r := committedRepository(t)
	ctx := context.Background()
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	run(t, r, "remote", "add", "origin", bare)
	if _, err := r.Publish(ctx, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	initial := strings.TrimSpace(run(t, r, "rev-parse", "HEAD"))
	run(t, r, "branch", "other")
	run(t, r, "push", "origin", "other")
	run(t, r, "switch", "other")
	write(t, r, "file.txt", "private other branch\n")
	run(t, r, "commit", "-am", "other change")
	run(t, r, "switch", "main")
	run(t, r, "config", "push.default", "matching")
	if _, err := r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "--git-dir", bare, "rev-parse", "refs/heads/other").Output()
	if err != nil || strings.TrimSpace(string(out)) != initial {
		t.Fatal("sync published an unrelated branch")
	}
}
