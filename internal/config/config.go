package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/MoeForge/moego/json"
)

// Dir is vl's preferences directory on every platform.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vl"), nil
}

func loadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeConfigFile(path, append(data, '\n'))
}

func writeConfigFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// CacheDir returns the application cache directory for kind.
func CacheDir(kind string) string {
	base := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(base) {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return ""
		}
	}
	return filepath.Join(base, "vl", kind)
}
