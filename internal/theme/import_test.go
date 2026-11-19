package theme

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsVSCodeExtensionDirs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows installation discovery")
	}
	for _, location := range []string{"user", "system", "system-x86", "path"} {
		for _, version := range []string{"", "645f29cc31"} {
			t.Run(location+"/"+version, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("USERPROFILE", filepath.Join(base, "home"))
				t.Setenv("LOCALAPPDATA", filepath.Join(base, "local"))
				t.Setenv("ProgramFiles", filepath.Join(base, "Program Files"))
				t.Setenv("ProgramFiles(x86)", filepath.Join(base, "Program Files (x86)"))
				t.Setenv("PATH", filepath.Join(base, "empty-path"))
				var install string
				switch location {
				case "user":
					install = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Microsoft VS Code")
				case "system":
					install = filepath.Join(os.Getenv("ProgramFiles"), "Microsoft VS Code")
				case "system-x86":
					install = filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft VS Code")
				case "path":
					install = filepath.Join(base, "自定义 [VS Code]")
					bin := filepath.Join(install, "bin")
					if err := os.MkdirAll(bin, 0700); err != nil {
						t.Fatal(err)
					}
					writeTheme(t, bin, "code.cmd", "@echo off\r\n")
					t.Setenv("PATH", bin)
				}
				extensions := filepath.Join(install, version, "resources", "app", "extensions")
				extension := filepath.Join(extensions, "theme-fixture")
				if err := os.MkdirAll(extension, 0700); err != nil {
					t.Fatal(err)
				}
				writeTheme(t, extension, "package.json", `{"publisher":"example","name":"fixture","contributes":{"themes":[
					{"id":"night","label":"Fixture Night","uiTheme":"vs-dark","path":"theme.json"}
				]}}`)
				writeTheme(t, extension, "theme.json", `{"colors":{"editor.background":"#123456"}}`)
				dirs, err := VSCodeExtensionDirs()
				if err != nil {
					t.Fatal(err)
				}
				want, err := filepath.EvalSymlinks(extensions)
				if err != nil {
					t.Fatal(err)
				}
				if len(dirs) != 1 || dirs[0] != want {
					t.Fatalf("extension directories = %v, want [%s]", dirs, want)
				}
				dest := t.TempDir()
				if count, err := ImportVSCode(dest, dirs); count != 1 || err != nil {
					t.Fatalf("imported %d themes: %v", count, err)
				}
				themes, err := DiscoverImports(dest)
				if err != nil || len(themes) != 1 || themes[0].Name != "Fixture Night" || themes[0].Color("editor.background") != "#123456" {
					t.Fatalf("imported themes = %v, %v", themes, err)
				}
			})
		}
	}
}

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
