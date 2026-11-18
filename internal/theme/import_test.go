package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportSkipsBundledThemesAndResolvesSavedIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dest, err := ImportDir()
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	extension := filepath.Join(source, defaultExtensionID)
	if err := os.Mkdir(extension, 0700); err != nil {
		t.Fatal(err)
	}
	writeTheme(t, extension, "package.json", `{"publisher":"vscode","name":"theme-defaults","contributes":{"themes":[
		{"id":"Default High Contrast","label":"Dark High Contrast","path":"missing.json","uiTheme":"hc-black"},
		{"id":"exact/id","label":"Display Name","path":"theme.json","uiTheme":"vs"}
	]}}`)
	writeTheme(t, extension, "theme.json", `{"colors":{"editor.background":"#EEE"}}`)
	if count, err := ImportVSCode(dest, []string{source}); count != 1 || err != nil {
		t.Fatalf("imported %d themes: %v", count, err)
	}
	id := "vscode:vscode.theme-defaults/exact/id"
	loaded, err := Load(id)
	if err != nil || loaded.ID != id || loaded.VSCodeID != "exact/id" || loaded.Name != "Display Name" {
		t.Fatalf("theme ID did not survive importing: %v", err)
	}
	standalone := writeTheme(t, filepath.Dir(dest), "standalone.json", `{"colors":{"editor.background":"#FFF"}}`)
	if _, err := Load(standalone); err == nil {
		t.Fatal("loaded a standalone theme outside the import catalog")
	}
	if _, err := Load("vscode:vscode.theme-defaults/Default High Contrast"); err == nil {
		t.Fatal("loaded a bundled theme through an import ID")
	}
	if err := os.Remove(filepath.Join(extension, "theme.json")); err != nil {
		t.Fatal(err)
	}
	if themes, err := DiscoverImports(dest); err != nil || len(themes) != 1 || themes[0].ID != id {
		t.Fatalf("imported catalog depends on the source extension: %d themes, %v", len(themes), err)
	}
}

func TestImportVSCode(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	extension := filepath.Join(source, "example.colors-1.0.0")
	if err := os.MkdirAll(filepath.Join(extension, "themes"), 0700); err != nil {
		t.Fatal(err)
	}
	writeTheme(t, extension, "package.json", `{
		"name": "colors", "publisher": "Example",
		"contributes": {"themes": [
			{"id":"day", "label":"%theme.day%", "uiTheme":"hc-light", "path":"./themes/day.jsonc"},
			{"label":"Broken", "uiTheme":"vs-dark", "path":"./missing.json"}
		]}
	}`)
	writeTheme(t, extension, "package.nls.json", `{"theme.day":"Bright Day"}`)
	writeTheme(t, extension, "base.json", `{"colors":{"editor.background":"#eee","editor.foreground":"#121212","sideBar.background":"#123"},"tokenColors":[{"scope":"comment","settings":{"foreground":"#654","fontStyle":"italic"}}]}`)
	writeTheme(t, extension, "tokens.tmTheme", `<plist version="1.0"><dict><key>settings</key><array>
		<dict><key>scope</key><string>comment</string><key>settings</key><dict>
		<key>foreground</key><string>#abc</string><key>fontStyle</key><string></string>
		</dict></dict></array></dict></plist>`)
	writeDay := func(background string) {
		writeTheme(t, filepath.Join(extension, "themes"), "day.jsonc", `{
			// The contribution supplies the name and the type.
			"name":"File Name","type":"dark","include":"../base.json",
			"colors":{"editor.background":"`+background+`","sideBar.background":"default",},
			"tokenColors":"../tokens.tmTheme",
		}`)
	}
	writeDay("#fff")
	count, err := ImportVSCode(dest, []string{source})
	if count != 1 || err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Fatalf("import = %d, %v", count, err)
	}
	original, err := DiscoverImports(dest)
	if err != nil || len(original) != 1 {
		t.Fatalf("discovered %d themes: %v", len(original), err)
	}
	th := original[0]
	if th.VSCodeID != "day" || th.ID != "vscode:example.colors/day" || th.Name != "Bright Day" || th.Type != "hc-light" || th.Color("editor.foreground") != "#121212" ||
		th.Color("editor.background") != "#FFFFFF" || th.Color("sideBar.background") == "#112233" {
		t.Fatalf("imported metadata or inherited colors differ: %s, %s, %+v", th.Name, th.Type, th.colors)
	}
	if got := th.Resolve([]string{"comment.line"}); got.Color != "#AABBCC" || got.Font != 0 {
		t.Fatalf("imported TextMate rule differs: %+v", got)
	}

	writeDay("#fefefe")
	count, _ = ImportVSCode(dest, []string{source})
	if count != 0 {
		t.Fatalf("reimport added %d existing themes", count)
	}
	updated, err := DiscoverImports(dest)
	if err != nil || len(updated) != 1 ||
		updated[0].ID != th.ID || updated[0].Color("editor.background") != "#FFFFFF" {
		t.Fatalf("reimport changed the existing theme: %v", err)
	}
}
