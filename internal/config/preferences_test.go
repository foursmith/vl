package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferencesPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path, err := PreferencesPath()
	if err != nil || path != filepath.Join(home, ".config", "vl", "vl.config.json") {
		t.Fatalf("preferences path = %q, %v", path, err)
	}
}

func TestPreferencesRoundTripAndInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vl", "vl.config.json")
	got, err := LoadPreferences(path)
	if err != nil || !got.Equal(Preferences{}) || !got.Features.VimEnabled() {
		t.Fatalf("missing preferences: %+v, %v", got, err)
	}
	want := Preferences{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_plus", Light: "light_plus"}, Sidebar: SidebarPreferences{Side: "right", Width: 52}},
		Features: Features{
			VimMode: new(false), ChineseWordSegmentation: true,
			AutoSwitchInputMethod: InputMethodSwitch{Enable: true, Cmd: []string{"im-select", "com.apple.keylayout.ABC"}},
		},
	}
	store := NewPreferencesStore(path)
	if err := store.Prepare(want)(); err != nil {
		t.Fatal(err)
	}
	got, err = LoadPreferences(path)
	if err != nil || !got.Equal(want) || got.Features.VimEnabled() {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	if err := store.Prepare(Preferences{})(); err != nil {
		t.Fatal(err)
	}
	got, err = LoadPreferences(path)
	if err != nil || !got.Equal(Preferences{}) {
		t.Fatal("reset did not persist")
	}
	if err := os.WriteFile(path, []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPreferences(path); err == nil {
		t.Fatal("invalid preferences were silently accepted")
	}
}

func TestPreferencesStoreKeepsNewestPreparedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vl.config.json")
	store := NewPreferencesStore(path)
	old := store.Prepare(Preferences{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_vs"}}})
	latest := store.Prepare(Preferences{Appearance: Appearance{Theme: ThemePreferences{Dark: "light_plus"}}})
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	// Simulate a delayed command finishing after the most recent choice.
	if err := old(); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadPreferences(path)
	if err != nil || saved.Appearance.Theme.Dark != "light_plus" {
		t.Fatalf("stale write won: %+v, %v", saved, err)
	}
	// The exit-time save uses the same ordering as background writes.
	final := store.Prepare(Preferences{Appearance: Appearance{Theme: ThemePreferences{Dark: "dark_modern"}}})
	if err := final(); err != nil {
		t.Fatal(err)
	}
	if err := latest(); err != nil {
		t.Fatal(err)
	}
	saved, err = LoadPreferences(path)
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

func TestPreferencesJSONCUpdatesKeepCommentsAndUnrelatedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vl.config.json")
	const original = "\ufeff// VL preferences\r\n" +
		"{\r\n" +
		"  /* Workbench */\r\n" +
		"  \"appearance\": {\r\n" +
		"    \"sidebar\": {\"width\": 40, /* automatic width */},\r\n" +
		"    \"theme\": null,\r\n" +
		"    \"example\": \"https://example.com/a/*path*/\",\r\n" +
		"  },\r\n" +
		"  \"unknown\": [1, 2,], // keep this field\r\n" +
		"}\r\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	preferences, err := LoadPreferences(path)
	if err != nil || preferences.Appearance.Sidebar.Width != 40 {
		t.Fatalf("LoadPreferences = %+v, %v", preferences, err)
	}
	store := NewPreferencesStore(path)
	want, err := store.Load()
	if err != nil || !want.Equal(preferences) {
		t.Fatalf("store.Load = %+v, %v", want, err)
	}
	want.Appearance.Sidebar.Width = 0
	want.Appearance.Theme.Dark = "dark_plus"
	want.Language = "zh-CN"
	if err := store.Prepare(want)(); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(path)
	if err != nil || !got.Equal(want) {
		t.Fatalf("updated preferences = %+v, %v, want %+v", got, err, want)
	}
	expected := strings.NewReplacer(
		"\"width\": 40", "\"width\": 0",
		"\"theme\": null", "\"theme\": {\"dark\": \"dark_plus\"}",
	).Replace(original)
	expected = strings.TrimSuffix(expected, "}\r\n") + "  \"language\": \"zh-CN\"\r\n}\r\n"
	data, err := os.ReadFile(path)
	if err != nil || string(data) != expected {
		t.Fatalf("updated source = %q, %v, want %q", data, err, expected)
	}
}

func TestPreferencesRejectMalformedJSONC(t *testing.T) {
	for _, source := range []string{`{ /* unclosed`, `{ , }`, `{"features": {"vimMode": false,, }}`, `{features: 'false'}`} {
		if _, err := decodePreferences([]byte(source)); err == nil {
			t.Errorf("accepted malformed JSONC %q", source)
		}
	}
}
