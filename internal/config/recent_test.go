package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecentFoldersPersistAndMoveToFront(t *testing.T) {
	path := filepath.Join(t.TempDir(), "elegit", "recent.json")
	if folders, err := loadRecentFolders(path); err != nil || len(folders) != 0 {
		t.Fatalf("missing history: %v %v", folders, err)
	}
	first, second := filepath.Join(t.TempDir(), "中文 project"), t.TempDir()
	for _, folder := range []string{first, second, first} {
		if err := rememberFolder(path, folder); err != nil {
			t.Fatal(err)
		}
	}
	folders, err := loadRecentFolders(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{first, second}; !reflect.DeepEqual(folders, want) {
		t.Fatalf("folders=%v, want %v", folders, want)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files were left behind: %v %v", entries, err)
	}
}

func TestRecentFoldersAreBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent.json")
	for i := 0; i < maxRecentFolders+5; i++ {
		if err := rememberFolder(path, fmt.Sprintf("/project/%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	folders, err := loadRecentFolders(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != maxRecentFolders || folders[0] != "/project/24" || folders[len(folders)-1] != "/project/5" {
		t.Fatalf("unexpected history order or limit: %v", folders)
	}
}

func TestMalformedRecentFoldersAreNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent.json")
	original := []byte("invalid history")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := rememberFolder(path, "/project/new"); err == nil {
		t.Fatal("malformed history was accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatal("malformed history was overwritten")
	}
}

func TestForgetFolderPreservesDirectoriesAndOtherEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent.json")
	folder, other := t.TempDir(), t.TempDir()
	if err := saveJSON(path, []string{folder, other, folder}); err != nil {
		t.Fatal(err)
	}
	folders, err := forgetFolder(path, folder)
	if err != nil || !reflect.DeepEqual(folders, []string{other}) {
		t.Fatalf("remove: %v %v", folders, err)
	}
	persisted, err := loadRecentFolders(path)
	if err != nil || !reflect.DeepEqual(persisted, folders) {
		t.Fatalf("persisted: %v %v", persisted, err)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Fatal("workspace directory was removed")
	}
	if err := os.WriteFile(path, []byte("invalid history"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := forgetFolder(path, other); err == nil {
		t.Fatal("malformed history was accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "invalid history" {
		t.Fatal("malformed history was overwritten")
	}
}
