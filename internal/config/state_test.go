package config

import (
	"path/filepath"
	"testing"
)

func TestStateHomeOverrideAndFallback(t *testing.T) {
	for _, value := range []string{"", "relative/state", filepath.Join(t.TempDir(), "custom-state")} {
		t.Run(value, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", value)
			base := filepath.Join(home, ".local", "state", "vl")
			if filepath.IsAbs(value) {
				base = filepath.Join(value, "vl")
			}
			store, err := NewFilePositionStore()
			if err != nil {
				t.Fatal(err)
			}
			if store.Dir != filepath.Join(base, "files") {
				t.Fatalf("file state directory = %q, want %q", store.Dir, base)
			}
			path := filepath.Join(home, "example.go")
			if err := store.Save(path, FilePosition{Line: 12}); err != nil {
				t.Fatal(err)
			}
			sessionPath, err := SessionPath(home)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Dir(sessionPath) != filepath.Join(base, "workspaces") {
				t.Fatalf("session directory = %q", sessionPath)
			}
			if err := SaveSession(sessionPath, Session{Active: -1}); err != nil {
				t.Fatal(err)
			}
			if err := RememberFolder(home); err != nil {
				t.Fatal(err)
			}
			if folders, err := loadRecentFolders(filepath.Join(base, "recent.json")); err != nil || len(folders) != 1 || folders[0] != home {
				t.Fatalf("recent folders = %v, %v", folders, err)
			}
		})
	}
}
