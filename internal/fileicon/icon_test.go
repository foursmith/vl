package fileicon

import (
	"github.com/charmbracelet/x/ansi"
	"testing"
)

func TestAllMaterialAssociationsFitIconCells(t *testing.T) {
	paths := []string{"unknown", "eslint.config.mjs", "Dockerfile", "go.mod", "README.md", "Makefile", ".gitignore", "LICENSE"}
	for ext := range extensions {
		paths = append(paths, "file"+ext)
	}
	for _, nerd := range []bool{false, true} {
		check := func(icon Icon) {
			t.Helper()
			if ansi.StringWidth(icon.Glyph) != 1 {
				t.Fatalf("%s icon has width %d", icon.Glyph, ansi.StringWidth(icon.Glyph))
			}
			if !nerd {
				for _, r := range icon.Glyph {
					if r < 32 || r > 126 {
						t.Fatalf("%s fallback requires a symbol font", icon.Glyph)
					}
				}
			}
		}
		for _, path := range paths {
			check(For(path, nerd))
		}
		for _, path := range []string{"src", "cmd", "docs", "bin", "examples", "tests"} {
			for _, open := range []bool{false, true} {
				check(Folder(path, open, nerd))
			}
		}
	}
}
