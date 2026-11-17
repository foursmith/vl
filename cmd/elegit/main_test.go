package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/ourongxing/elegit/internal/config"
)

func TestStartupPathUsesExplicitPathOrMostRecentWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := startupPath(nil); err == nil {
		t.Fatal("startup without history must require a path")
	}
	for _, folder := range []string{"/workspace/older", "/workspace/latest"} {
		if err := config.RememberFolder(folder); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "/workspace/latest"},
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
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "elegit", "recent.json"), []byte("invalid"), 0600); err != nil {
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
