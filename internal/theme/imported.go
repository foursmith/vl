package theme

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/foursmith/vl/internal/config"
)

const importedPrefix = "vscode:"

var extensionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.[a-z0-9][a-z0-9-]*$`)
var storedThemeFilename = regexp.MustCompile(`^[a-f0-9]{16}\.json$`)

func themeFilename(id string) string {
	sum := sha256.Sum256([]byte("id:" + id))
	return fmt.Sprintf("%x.json", sum[:8])
}

func ImportDir() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes", "vscode"), nil
}

func loadImportedID(id string) (*Theme, error) {
	ref, ok := strings.CutPrefix(id, importedPrefix)
	if !ok {
		return nil, fmt.Errorf("unknown color theme %q", id)
	}
	extension, name, _ := strings.Cut(ref, "/")
	if !extensionIDPattern.MatchString(extension) || name == "" {
		return nil, fmt.Errorf("invalid imported theme ID %q", id)
	}
	if isBundledTheme(extension, name) {
		return nil, fmt.Errorf("unknown color theme %q", id)
	}

	dir, err := ImportDir()
	if err != nil {
		return nil, err
	}
	filename := filepath.Join(dir, extension, themeFilename(name))
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("imported theme %q is not a regular file", id)
	}
	return readImported(filename, extension)
}

// DiscoverImports reads the managed catalog. Standalone theme files are outside
// this catalog even if they are present in the parent themes directory.
func DiscoverImports(dir string) ([]*Theme, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []*Theme
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() || !extensionIDPattern.MatchString(entry.Name()) {
			continue
		}
		themes, err := readImportedExtension(dir, entry.Name())
		result = append(result, themes...)
		if err != nil {
			problems = append(problems, err)
		}
	}
	return result, errors.Join(problems...)
}

func readImportedExtension(dir, extension string) ([]*Theme, error) {
	dir = filepath.Join(dir, extension)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var themes []*Theme
	var problems []error
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !storedThemeFilename.MatchString(entry.Name()) {
			continue
		}
		th, err := readImported(filepath.Join(dir, entry.Name()), extension)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if !isBundledTheme(extension, th.VSCodeID) {
			themes = append(themes, th)
		}
	}
	return themes, errors.Join(problems...)
}

func readImported(filename, extension string) (*Theme, error) {
	data, err := readThemeFile(filename)
	if err != nil {
		return nil, err
	}
	var saved themeData
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	if saved.ID == "" || filepath.Base(filename) != themeFilename(saved.ID) || !validUITheme(saved.Type) {
		return nil, fmt.Errorf("invalid imported theme ID or uiTheme: %q (%q)", saved.ID, saved.Type)
	}
	return saved.theme(importedPrefix + extension + "/" + saved.ID), nil
}
