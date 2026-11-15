package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Layout stores user preferences independently of the current terminal size.
type Layout struct {
	SidebarWidth int  `json:"sidebarWidth"`
	SidebarRight bool `json:"sidebarRight"`
}

func LayoutPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "elegit", "layout.json"), nil
}

func LoadLayout(path string) (Layout, error) {
	var layout Layout
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return layout, nil
	}
	if err != nil {
		return layout, err
	}
	err = json.Unmarshal(data, &layout)
	return layout, err
}

func SaveLayout(path string, layout Layout) error {
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".layout-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
