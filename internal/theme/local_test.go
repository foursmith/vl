package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverLocalThemes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pack"), 0700); err != nil {
		t.Fatal(err)
	}
	writeTheme(t, dir, "dark_modern.json", `{"name":"Personal Dark","type":"dark","colors":{"editor.background":"#112233"}}`)
	writeTheme(t, dir, "package.json", `{"name":"not a theme"}`)
	writeTheme(t, dir, "notes.json", `{"note":"not a theme"}`)
	writeTheme(t, filepath.Join(dir, "pack"), "base.json", `{"colors":{"editor.foreground":"#123456"}}`)
	writeTheme(t, filepath.Join(dir, "pack"), "day.jsonc", `{/* local */ "name":"Day", "type":"light", "include":"base.json", "colors":{"editor.background":"#fff"},}`)
	themes, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 3 {
		t.Fatalf("discovered %d themes", len(themes))
	}
	byID := map[string]*Theme{}
	for _, th := range themes {
		byID[th.ID] = th
	}
	dark := byID["local:dark_modern.json"]
	day := byID["local:pack/day.jsonc"]
	if dark == nil || dark.IsLight() || dark.Palette().Background != "#112233" {
		t.Fatal("local name collided with builtin")
	}
	if day == nil || !day.IsLight() || day.Palette().Foreground != "#123456" {
		t.Fatal("local inheritance/type lost")
	}
	restored, err := loadLocal(dir, day.ID, day.Type)
	if err != nil || restored.ID != day.ID || restored.Palette() != day.Palette() {
		t.Fatalf("local identity restore: %v", err)
	}
	writeTheme(t, dir, "broken.json", `{/*`)
	valid, err := Discover(dir)
	if err == nil || len(valid) != 3 {
		t.Fatal("bad theme either disappeared silently or blocked valid themes")
	}
}

func TestLocalThemeDirectoryAndIDs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := LocalDir()
	if err != nil || dir != filepath.Join(home, ".config", "vl", "themes") {
		t.Fatalf("local directory: %q %v", dir, err)
	}
	themes, err := Discover(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(themes) != 0 {
		t.Fatalf("missing directory: %v", err)
	}
	for _, id := range []string{"local:../outside.json", "local:/tmp/outside.json", "local:"} {
		if _, err := loadLocal(t.TempDir(), id, ""); err == nil {
			t.Errorf("accepted invalid local ID %q", id)
		}
	}
}
