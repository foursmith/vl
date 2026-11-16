package main

import (
	"os"
	"path/filepath"
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
