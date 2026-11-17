package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const localPrefix = "local:"

// LocalDir deliberately uses ~/.config on every platform, including macOS.
func LocalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "elegit", "themes"), nil
}

func loadLocal(dir, id, uiTheme string) (*Theme, error) {
	relative := strings.TrimPrefix(id, localPrefix)
	if !filepath.IsLocal(filepath.FromSlash(relative)) {
		return nil, fmt.Errorf("invalid local theme ID %q", id)
	}
	th, err := Load(filepath.Join(dir, filepath.FromSlash(relative)), uiTheme)
	if err != nil {
		return nil, err
	}
	th.ID = localPrefix + filepath.ToSlash(filepath.Clean(relative))
	return th, nil
}

// Discover loads local theme files once, outside rendering. Subdirectories may
// contain a theme's relative includes or a whole VS Code extension directory.
// Non-theme JSON files (such as package.json) are not offered as themes.
func Discover(dir string) ([]*Theme, error) {
	var themes []*Theme
	var problems []error
	err := filepath.WalkDir(dir, func(name string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) && name == dir {
			return nil
		}
		if walkErr != nil {
			problems = append(problems, walkErr)
			return nil
		}
		if entry.IsDir() {
			if name != dir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".jsonc" && ext != ".tmtheme" {
			return nil
		}
		b, err := readThemeFile(name)
		if err == nil && ext != ".tmtheme" {
			b, err = jsonc(b)
			if err == nil {
				var keys map[string]json.RawMessage
				err = json.Unmarshal(b, &keys)
				if err == nil && keys["colors"] == nil && keys["tokenColors"] == nil && keys["settings"] == nil && keys["include"] == nil {
					return nil
				}
			}
		}
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			return nil
		}
		relative, err := filepath.Rel(dir, name)
		if err != nil {
			return err
		}
		th, err := loadLocal(dir, localPrefix+filepath.ToSlash(relative), "")
		if err != nil {
			problems = append(problems, err)
			return nil
		}
		themes = append(themes, th)
		return nil
	})
	if err != nil {
		problems = append(problems, err)
	}
	return themes, errors.Join(problems...)
}
