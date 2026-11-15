package fileicon

import (
	"path/filepath"
	"strings"
)

type Icon struct{ Glyph, Color string }

// Terminal adaptations of Material Icon Theme's file associations and colors.
// Glyphs come from Nerd Fonts; the upstream SVG artwork is not rendered as text.
var extensions = map[string]Icon{
	".go": {"\ue627", "#00ACC1"},
	".ts": {"\ue628", "#0288D1"}, ".tsx": {"\ue7ba", "#00BCD4"},
	".js": {"\ue74e", "#FFCA28"}, ".jsx": {"\ue7ba", "#00BCD4"},
	".mjs": {"\ue74e", "#FFCA28"}, ".cjs": {"\ue74e", "#FFCA28"},
	".json": {"\ue60b", "#F9A825"}, ".jsonc": {"\ue60b", "#F9A825"},
	".md": {"\ue73e", "#42A5F5"}, ".mdx": {"\ue73e", "#42A5F5"},
	".rs": {"\ue7a8", "#FF7043"}, ".py": {"\ue73c", "#0288D1"},
	".html": {"\ue736", "#E65100"}, ".css": {"\ue749", "#7E57C2"},
	".scss": {"\ue74b", "#F06292"}, ".vue": {"\ue6a0", "#41B883"},
	".svelte": {"\ue697", "#FF3E00"}, ".sh": {"\ue795", "#4CAF50"},
	".yaml": {"\ue6a8", "#F44336"}, ".yml": {"\ue6a8", "#F44336"},
	".toml": {"\ue615", "#9E9E9E"}, ".xml": {"\ue619", "#8BC34A"},
	".sql": {"\ue706", "#FFCA28"}, ".swift": {"\ue755", "#F05138"},
	".c": {"\ue61e", "#0288D1"}, ".h": {"\ue61e", "#A682E6"},
	".cpp": {"\ue61d", "#0288D1"}, ".java": {"\ue738", "#F44336"},
	".png": {"\uf03e", "#26A69A"}, ".jpg": {"\uf03e", "#26A69A"}, ".svg": {"\uf03e", "#FFB300"},
}

func For(path string, nerd bool) Icon {
	name := strings.ToLower(filepath.Base(path))
	icon := Icon{"\uf15b", "#90A4AE"}
	switch {
	case strings.HasPrefix(name, "eslint.config.") || strings.HasPrefix(name, ".eslintrc") || name == ".eslintignore":
		icon = Icon{"\ue655", "#4B32C3"}
	case name == "dockerfile" || strings.HasPrefix(name, "docker-compose"):
		icon = Icon{"\ue7b0", "#0288D1"}
	case name == "go.mod" || name == "go.sum":
		icon = Icon{"\U000f07d3", "#FF4081"}
	case name == "readme.md" || name == "readme":
		icon = Icon{"\uf05a", "#42A5F5"}
	case name == "makefile" || name == "justfile":
		icon = Icon{"\ue615", "#EF5350"}
	case strings.HasPrefix(name, ".git"):
		icon = Icon{"\ue702", "#E64A19"}
	case name == "license" || strings.HasPrefix(name, "license."):
		icon = Icon{"\uf48a", "#FFCA28"}
	default:
		if match, ok := extensions[strings.ToLower(filepath.Ext(name))]; ok {
			icon = match
		}
	}
	if !nerd {
		icon.Glyph = "-"
	}
	return icon
}

func Folder(path string, open, nerd bool) Icon {
	icon := Icon{"\uf07b", "#90A4AE"}
	if open {
		icon.Glyph = "\uf07c"
	}
	switch strings.ToLower(filepath.Base(path)) {
	case "bin", "build", "dist":
		icon.Color = "#EF9A9A"
	case "cmd", "command", "commands":
		icon.Color = "#00BCD4"
	case "docs", "doc", "documentation":
		icon.Color = "#42A5F5"
	case "examples", "example", "demo":
		icon.Color = "#26A69A"
	case "test", "tests":
		icon.Color = "#8BC34A"
	}
	if !nerd {
		icon.Glyph = "/"
	}
	return icon
}
