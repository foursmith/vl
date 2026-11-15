package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutRoundTripAndInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "elegit", "layout.json")
	got, err := LoadLayout(path)
	if err != nil || got != (Layout{}) {
		t.Fatalf("missing preferences: %+v, %v", got, err)
	}
	want := Layout{SidebarWidth: 52, SidebarRight: true, Theme: "auto"}
	if err := SaveLayout(path, want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadLayout(path)
	if err != nil || got != want {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	if err := SaveLayout(path, Layout{}); err != nil {
		t.Fatal(err)
	}
	got, err = LoadLayout(path)
	if err != nil || got != (Layout{}) {
		t.Fatal("reset did not persist")
	}
	if err := os.WriteFile(path, []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLayout(path); err == nil {
		t.Fatal("invalid preferences were silently accepted")
	}
}
