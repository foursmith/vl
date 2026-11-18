package theme

import (
	"embed"
	"maps"
	"slices"
	"sync"
)

//go:embed themes/*.json
var files embed.FS

const defaultExtensionID = "vscode.theme-defaults"

// The theme files and contribution metadata come from the same pinned VS Code
// build as defaults.json.
var builtinThemes = map[string]struct{ vscodeID, label, kind, file string }{
	"dark_2026":    {"Dark 2026", "Dark 2026", "vs-dark", "2026-dark.json"},
	"dark_modern":  {"Dark Modern", "Dark Modern", "vs-dark", "dark_modern.json"},
	"dark_plus":    {"Dark+", "Dark+", "vs-dark", "dark_plus.json"},
	"dark_vs":      {"Visual Studio Dark", "Dark (Visual Studio)", "vs-dark", "dark_vs.json"},
	"hc_black":     {"Default High Contrast", "Dark High Contrast", "hc-black", "hc_black.json"},
	"hc_light":     {"Default High Contrast Light", "Light High Contrast", "hc-light", "hc_light.json"},
	"light_2026":   {"Light 2026", "Light 2026", "vs", "2026-light.json"},
	"light_modern": {"Light Modern", "Light Modern", "vs", "light_modern.json"},
	"light_plus":   {"Light+", "Light+", "vs", "light_plus.json"},
	"light_vs":     {"Visual Studio Light", "Light (Visual Studio)", "vs", "light_vs.json"},
}

var BuiltinIDs = slices.Sorted(maps.Keys(builtinThemes))
var builtins sync.Map

func isBundledTheme(extensionID, id string) bool {
	if extensionID != defaultExtensionID {
		return false
	}
	for _, spec := range builtinThemes {
		if spec.vscodeID == id {
			return true
		}
	}
	return false
}

func Builtin(id string) *Theme {
	if t, ok := builtins.Load(id); ok {
		return t.(*Theme)
	}
	spec := builtinThemes[id]
	data, err := loadSource("themes/"+spec.file, spec.kind, files.ReadFile)
	if err != nil {
		panic(err)
	}
	data.ID, data.Name = spec.vscodeID, spec.label
	actual, _ := builtins.LoadOrStore(id, data.theme(id))
	return actual.(*Theme)
}
