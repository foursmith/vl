package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path, err := LayoutPath()
	if err != nil || path != filepath.Join(home, ".config", "vl", "vl.config.json") {
		t.Fatalf("layout path = %q, %v", path, err)
	}
}

func TestLayoutRoundTripAndInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vl", "vl.config.json")
	got, err := LoadLayout(path)
	if err != nil || !got.Equal(Layout{}) {
		t.Fatalf("missing preferences: %+v, %v", got, err)
	}
	want := Layout{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_plus", Light: "light_plus"}, Sidebar: SidebarPreferences{Side: "right", Width: 52}}, Vim: VimPreferences{EnableJieba: true},
		AutoSwitchInputMethod: InputMethodSwitch{Enable: true, Cmd: []string{"im-select", "com.apple.keylayout.ABC"}}}
	if err := SaveLayout(path, want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadLayout(path)
	if err != nil || !got.Equal(want) {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	if err := SaveLayout(path, Layout{}); err != nil {
		t.Fatal(err)
	}
	got, err = LoadLayout(path)
	if err != nil || !got.Equal(Layout{}) {
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
	path := filepath.Join(t.TempDir(), "vl.config.json")
	store := NewLayoutStore(path)
	old := store.Prepare(Layout{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_vs"}}})
	latest := store.Prepare(Layout{Appearance: Appearance{Theme: ThemePreferences{Dark: "light_plus"}}})
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	// Simulate a delayed command finishing after the most recent choice.
	if err := old(); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadLayout(path)
	if err != nil || saved.Appearance.Theme.Dark != "light_plus" {
		t.Fatalf("stale write won: %+v, %v", saved, err)
	}
	// The exit-time save uses the same ordering as background writes.
	final := store.Prepare(Layout{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_modern"}}})
	if err := final(); err != nil {
		t.Fatal(err)
	}
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	saved, err = LoadLayout(path)
	if err != nil || saved.Appearance.Theme.Dark != "dark_modern" {
		t.Fatalf("exit save was overwritten: %+v, %v", saved, err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Prepare(saved)(); err != nil {
		t.Fatal(err)
	}
	if after, err := os.Stat(path); err != nil || !os.SameFile(before, after) {
		t.Fatalf("identical preferences replaced the saved file: %v", err)
	}
}
