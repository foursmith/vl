package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/foursmith/vl/internal/config"
)

func TestLoadWorkspacePreservesSavedGitDiffsWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m, repo, editorOnly, err := startupModel(t.Context(), t.TempDir(), false)
	if err != nil || editorOnly {
		t.Fatalf("open folder without Git: %v", err)
	}
	defer m.Close()
	path, err := config.SessionPath(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveSession(path, config.Session{Tabs: []config.SessionTab{{Kind: "diff", Path: "file.txt"}}}); err != nil {
		t.Fatal(err)
	}
	if writable := loadWorkspace(m, repo); writable != "" {
		t.Fatalf("saved Git diff would be overwritten at %q", writable)
	}
}

func TestStartupPathUsesExplicitPathOrMostRecentWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	if _, err := startupPath(nil); err == nil {
		t.Fatal("startup without history must require a path")
	}
	if err := config.RememberFolder(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
	if _, err := startupPath(nil); err == nil {
		t.Fatal("startup with only missing workspaces must require a path")
	}
	latest := t.TempDir()
	for _, folder := range []string{t.TempDir(), latest, filepath.Join(t.TempDir(), "missing")} {
		if err := config.RememberFolder(folder); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, latest},
		{[]string{"/workspace/explicit"}, "/workspace/explicit"},
		{[]string{"."}, "."},
	} {
		got, err := startupPath(tc.args)
		if err != nil || got != tc.want {
			t.Fatalf("startupPath(%v) = %q, %v; want %q", tc.args, got, err, tc.want)
		}
	}
	if _, err := startupPath([]string{""}); err == nil {
		t.Fatal("empty path must not open the current directory")
	}
	// Explicit paths must remain usable when recent-history storage is invalid.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".local", "state", "vl", "recent.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := startupPath(nil); err == nil {
		t.Fatal("unreadable history must not fall back to the current directory")
	}
	if got, err := startupPath([]string{"."}); err != nil || got != "." {
		t.Fatalf("explicit path depends on recent history: %q, %v", got, err)
	}
}

func TestStartupFileBypassesGit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	m, repo, only, err := startupModel(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if !only || repo != nil {
		t.Fatal("file opened as workspace")
	}
	if _, _, _, err := startupModel(context.Background(), filepath.Join(dir, "missing"), false); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestEditorMemoryPolicyPreservesOverridesAndWorkbench(t *testing.T) {
	previous := debug.SetGCPercent(100)
	defer debug.SetGCPercent(previous)
	t.Setenv("GOGC", "")
	os.Unsetenv("GOGC")
	configureMemory(false)
	if got := debug.SetGCPercent(100); got != 100 {
		t.Fatalf("workbench GC = %d", got)
	}
	configureMemory(true)
	if got := debug.SetGCPercent(100); got != 50 {
		t.Fatalf("editor GC = %d", got)
	}
	for _, value := range []string{"off", "200", ""} {
		os.Setenv("GOGC", value)
		configureMemory(true)
		if got := debug.SetGCPercent(100); got != 100 {
			t.Fatalf("overrode GOGC=%q: %d", value, got)
		}
	}
}
