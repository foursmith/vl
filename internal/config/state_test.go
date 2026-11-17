package config

import (
	"os"
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

func TestDeletedStateDoesNotImportOldRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "preferences"))
	file := filepath.Join(home, "project", "file.go")
	oldStore := &FilePositionStore{Dir: filepath.Join(home, ".config", "vl", "files")}
	if err := oldStore.Save(file, FilePosition{Line: 25}); err != nil {
		t.Fatal(err)
	}
	legacySession := filepath.Join(home, ".config", "vl", "workspaces", "old.json")
	if err := SaveSession(legacySession, Session{Tabs: []SessionTab{{Kind: "file", Path: file}}}); err != nil {
		t.Fatal(err)
	}
	oldRecent, err := configPath("recent.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(oldRecent, []string{"/old/workspace"}); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		store, err := NewFilePositionStore()
		if err != nil {
			t.Fatal(err)
		}
		if got, exists, err := store.Load(file); err != nil || exists {
			t.Fatalf("old position returned on pass %d: %#v, %v", pass, got, err)
		}
		if folders, err := RecentFolders(); err != nil || len(folders) != 0 {
			t.Fatalf("old recent folders returned: %v, %v", folders, err)
		}
		sessionPath, err := SessionPath(home)
		if err != nil {
			t.Fatal(err)
		}
		if session, err := LoadSession(sessionPath); err != nil || len(session.Tabs) != 0 {
			t.Fatalf("old session returned: %#v, %v", session, err)
		}
		if entries, err := os.ReadDir(filepath.Dir(sessionPath)); !os.IsNotExist(err) {
			t.Fatalf("workspace records were recreated: %v, %v", entries, err)
		}
		if err := store.Save(file, FilePosition{Line: 50}); err != nil {
			t.Fatal(err)
		}
		if err := RememberFolder(home); err != nil {
			t.Fatal(err)
		}
		if err := SaveSession(sessionPath, Session{Tabs: []SessionTab{{Kind: "file", Path: file}}}); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(home, ".local", "state", "vl")); err != nil {
			t.Fatal(err)
		}
	}
}
