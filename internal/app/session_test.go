package app

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ourongxing/vl/internal/config"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func TestSessionRoundTripMixedTabs(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
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
	saved := m.Session()
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
	session := config.Session{Active: 1, Tabs: []config.SessionTab{
		{Kind: "file", Path: "missing.go"},
		{Kind: "file", Path: "file0.go", Preview: true, Line: 999, Column: 999},
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
