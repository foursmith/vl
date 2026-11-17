package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutRoundTripAndInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vl", "layout.json")
	got, err := LoadLayout(path)
	if err != nil || got != (Layout{}) {
		t.Fatalf("missing preferences: %+v, %v", got, err)
	}
	want := Layout{SidebarWidth: 52, SidebarRight: true, Theme: "dark_plus", DarkTheme: "dark_plus", LightTheme: "light_plus", LastCommand: "view.sidebar"}
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

func TestLayoutStoreKeepsNewestPreparedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	store := NewLayoutStore(path)
	old := store.Prepare(Layout{Theme: "dark_vs"})
	latest := store.Prepare(Layout{Theme: "light_plus"})
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	// Simulate a delayed command finishing after the most recent choice.
	if err := old(); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadLayout(path)
	if err != nil || saved.Theme != "light_plus" {
		t.Fatalf("stale write won: %+v, %v", saved, err)
	}
	// The exit-time save uses the same ordering as background writes.
	final := store.Prepare(Layout{Theme: "dark_modern"})
	if err := final(); err != nil {
		t.Fatal(err)
	}
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	saved, err = LoadLayout(path)
	if err != nil || saved.Theme != "dark_modern" {
		t.Fatalf("exit save was overwritten: %+v, %v", saved, err)
	}
}
