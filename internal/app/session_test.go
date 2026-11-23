package app

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestStartupRestoresMixedTabsBeforeSnapshot(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	path := filepath.Join(m.repo.Root, "cmd", "main.go")
	readme := filepath.Join(m.repo.Root, "README.md")
	m.SetSession(config.Session{Active: 1, Tabs: []config.SessionTab{
		{Kind: "file", Path: readme, Preview: true},
		{Kind: "file", Path: path},
		{Kind: "diff", Path: "cmd/main.go"},
	}})
	commands := m.Init()().(tea.BatchMsg)
	msg, ok := commands[len(commands)-1]().(restoreSessionMsg)
	if !ok {
		t.Fatal("startup did not schedule independent file restoration")
	}
	m.Update(msg)
	if doc := m.document(); doc == nil || doc.Path != path || !m.loading || m.pendingSession != nil {
		t.Fatal("saved file was not restored while the snapshot was loading")
	}

	if len(m.tabs) != 3 || m.activeTab != 1 || m.tabs[0].path() != readme ||
		!m.tabs[0].preview || m.tabs[0].positionBaseline == nil {
		t.Fatal("restoration did not preserve the background file tab and saved selection")
	}
	if m.tabs[2].diff == nil || !m.tabs[2].diff.waitingForSnapshot || m.tabs[2].diff.read != nil {
		t.Fatal("worktree diff did not reserve its tab while waiting for Git")
	}
	snapshot, err := m.repo.Snapshot(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := m.Update(snapshotMsg{seq: m.refreshSeq, snapshot: snapshot})
	drain(t, m, cmd)
	if len(m.tabs) != 3 || m.activeTab != 1 || m.tabs[2].diff.waitingForSnapshot || m.tabs[2].diff.patch.Raw == "" {
		t.Fatal("worktree diff did not load from the snapshot and preserve tab order")
	}
}

func TestSessionRestoresSidebarTab(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	for _, tc := range []struct {
		name string
		tab  section
	}{
		{"explorer", explorer}, {"search", search}, {"changes", changes},
		{"history", history}, {"pull-requests", pullRequests},
	} {
		m.section = tc.tab
		saved := m.Session()
		if saved.SidebarTab != tc.name {
			t.Fatalf("saved sidebar tab = %q, want %q", saved.SidebarTab, tc.name)
		}
		restored := New(context.Background(), m.repo, false)
		restored.SetSession(saved)
		drain(t, restored, restored.restoreSession())
		if restored.section != tc.tab || restored.Session().SidebarTab != tc.name {
			t.Fatalf("restoring %q selected %q", tc.name, restored.Session().SidebarTab)
		}
		if tc.tab == search && !restored.search.query.Focused() {
			t.Fatal("restored Search is missing input focus")
		}
		restored.Close()
	}
	fresh := New(context.Background(), m.repo, false)
	defer fresh.Close()
	for _, saved := range []config.Session{{}, {SidebarTab: "unknown-tab"}} {
		fresh.SetSession(saved)
		drain(t, fresh, fresh.restoreSession())
		if fresh.section != explorer {
			t.Fatal("new or older workspace did not default to Explorer")
		}
	}
}

func TestSessionRestoresPRDiffTabs(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := tabWorkspace(t, 1)
	defer m.Close()
	pr := gitservice.PullRequest{Number: 19, URL: "https://github.com/example/repo/pull/19", HeadRefOID: "head", BaseRefOID: "base"}
	raw := "diff --git a/old.go b/old.go\n--- a/old.go\n+++ b/old.go\n@@ -1 +1 @@\n-old\n+new\n" +
		"diff --git a/next.go b/next.go\n--- a/next.go\n+++ b/next.go\n@@ -1 +1 @@\n-old\n+next\n"
	patches, err := diff.SplitFiles(raw)
	if err != nil {
		t.Fatal(err)
	}
	cache := config.NewPullRequestDiffCache(m.repo.Root, pullRequestCacheSize)
	cache.Save(pr.URL, pr.DiffVersion(), raw)
	m.openFile("file0.go")
	for i, patch := range patches {
		target := diffTarget{path: patch.Path, pr: pr, prVersion: pr.DiffVersion(), preview: i == 1}
		drain(t, m, m.requestDiff(target, readPullRequestPatch(patch.Raw)))
	}
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	saved := m.Session()
	if err := config.SaveSession(sessionFile, saved); err != nil {
		t.Fatal(err)
	}
	session, err := config.LoadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	restore := func(version string) {
		t.Helper()
		restored := New(context.Background(), m.repo, false)
		defer restored.Close()
		restored.SetSession(session)
		drain(t, restored, restored.restoreSession())
		expected := saved
		expected.Tabs = append([]config.SessionTab(nil), saved.Tabs...)
		for i := 1; i < len(expected.Tabs); i++ {
			pr := *expected.Tabs[i].PullRequest
			pr.Version = version
			expected.Tabs[i].PullRequest = &pr
		}
		if got := restored.Session(); !reflect.DeepEqual(got, expected) || restored.activeTab != 2 || !restored.sidebar {
			t.Fatalf("restored session = %+v, want %+v", got, expected)
		}
		for i, patch := range patches {
			d := restored.tabs[i+1].diff
			if d == nil || d.patch.Raw != patch.Raw || d.diffChange != nil || d.read != nil {
				t.Fatalf("PR tab %d was not restored as a read-only file diff", i)
			}
		}
	}
	// A cached session can restore even when gh is unavailable.
	path := t.TempDir()
	t.Setenv("PATH", path)
	restore(pr.DiffVersion())

	if runtime.GOOS == "windows" {
		return
	}
	// On a cache miss, both files share a single fetch of the complete PR.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("VL_TEST_PR_PATCH", raw)
	t.Setenv("VL_TEST_PR_CALLS", filepath.Join(path, "calls"))
	if err := os.WriteFile(filepath.Join(path, "gh"), []byte(`#!/bin/sh
[ "$1 $2" != "pr view" ] || {
  printf '%s\n' '{"baseRefOid":"base","headRefOid":"new-head"}'
  exit 0
}
[ "$1 $2" = "pr diff" ] || exit 1
printf 'fetch\n' >> "$VL_TEST_PR_CALLS"
printf '%s' "$VL_TEST_PR_PATCH"
`), 0700); err != nil {
		t.Fatal(err)
	}
	restore("base:new-head")
	calls, err := os.ReadFile(filepath.Join(path, "calls"))
	if err != nil || string(calls) != "fetch\n" {
		t.Fatalf("session cache miss ran gh %q times: %v", calls, err)
	}
}

func TestSessionRoundTripMixedTabs(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	run := func(args ...string) string {
		t.Helper()
		out, err := m.repo.Run(context.Background(), args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(out)
	}
	run("add", ".")
	run("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	commit := run("rev-parse", "HEAD")
	path := filepath.Join(m.repo.Root, "file0.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("package main\n", 80)), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "file0.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("package main // changed\n", 80)), 0600); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(m.repo.Root, "test.png")
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	m.openFile("file0.go")
	m.document().SetPosition(40, 6)
	m.tabs[0].top = 30
	m.openFile("test.png")
	c := gitservice.Change{Path: "file0.go"}
	for _, target := range []diffTarget{
		{path: c.Path, change: &c},
		{path: c.Path, change: &c, staged: true},
		{path: c.Path, change: &c, commit: commit},
	} {
		m.requestDiff(target, func(context.Context) (string, error) { return "", nil })
	}
	m.openPreviewFile("file1.go")
	m.activateTab(0)
	m.SaveFilePositions()
	saved := m.Session()
	if saved.Tabs[1].Kind != "file" {
		t.Fatalf("image tab should be stored as a file, got %q", saved.Tabs[1].Kind)
	}
	sessionPath := filepath.Join(t.TempDir(), "sessions", "workspace.json")
	if err := config.SaveSession(sessionPath, saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadSession(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	restored := New(context.Background(), m.repo, false)
	defer restored.Close()
	restored.SetFilePositionStore(store)
	restored.SetSession(loaded)
	snapshot, err := m.repo.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := restored.Update(snapshotMsg{seq: restored.refreshSeq, snapshot: snapshot})
	drain(t, restored, cmd)
	if got := restored.Session(); !reflect.DeepEqual(got, saved) {
		t.Fatalf("restored session = %#v; want %#v", got, saved)
	}
	if restored.tabs[1].image == nil || restored.tabs[2].diff.patch.Raw == "" || restored.tabs[3].diff.patch.Raw == "" || restored.tabs[4].diff.patch.Raw == "" {
		t.Fatal("mixed tabs were not loaded from their sources")
	}
	restored.activateTab(4)
	if restored.diffChange != nil {
		t.Fatal("restored commit diff enabled staging")
	}
	restored.activateTab(5)
	restored.closeTab()
	if len(restored.Session().Tabs) != 5 {
		t.Fatal("closed tab remained in session")
	}
}

func TestSessionSkipsMissingTabsAndRetainsEmptySession(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	if err := store.Save(filepath.Join(m.repo.Root, "file0.go"), config.FilePosition{Line: 999, Column: 999}); err != nil {
		t.Fatal(err)
	}
	session := config.Session{Active: 1, SidebarTab: "explorer", Tabs: []config.SessionTab{
		{Kind: "file", Path: "missing.go"},
		{Kind: "file", Path: "file0.go", Preview: true},
		{Kind: "diff", Path: "child/deleted.go", Repository: "missing-child"},
		{Kind: "file", Path: "file1.go"},
	}}
	m.SetSession(session)
	if !reflect.DeepEqual(m.Session(), session) {
		t.Fatal("interrupted startup lost pending session")
	}
	drain(t, m, m.restoreSession())
	if len(m.tabs) != 2 || m.activeTab != 0 || !m.tabs[0].preview {
		t.Fatalf("restore lost surviving tabs or active selection: %#v", m.Session())
	}
	if m.document().Dirty() {
		t.Fatal("restoration modified document contents")
	}
	m.closeAllTabs()
	empty := m.Session()
	m.SetSession(empty)
	drain(t, m, m.restoreSession())
	if len(m.tabs) != 0 {
		t.Fatal("empty session reopened closed tabs")
	}
}
