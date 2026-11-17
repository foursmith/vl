package theme

import (
	"encoding/json"
	"path/filepath"
)

type themeContribution struct {
	Label   string `json:"label"`
	UITheme string `json:"uiTheme"`
	Path    string `json:"path"`
}

// VS Code stores uiTheme on the extension contribution, often outside the
// theme JSON itself. Inspect ancestor manifests only; do not scan installations.
func extensionTheme(filename string) themeContribution {
	for dir, depth := filepath.Dir(filename), 0; depth < 6; dir, depth = filepath.Dir(dir), depth+1 {
		b, err := readThemeFile(filepath.Join(dir, "package.json"))
		if err != nil {
			if filepath.Dir(dir) == dir {
				break
			}
			continue
		}
		b, err = jsonc(b)
		if err != nil {
			continue
		}
		var manifest struct {
			Contributes struct {
				Themes []themeContribution `json:"themes"`
			} `json:"contributes"`
		}
		if json.Unmarshal(b, &manifest) != nil {
			continue
		}
		for _, th := range manifest.Contributes.Themes {
			if filepath.Clean(filepath.Join(dir, th.Path)) == filename {
				return th
			}
		}
	}
	return themeContribution{}
}
