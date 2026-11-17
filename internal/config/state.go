package config

import (
	"os"
	"path/filepath"
)

// statePath resolves the runtime state directory independently of preferences.
func statePath(name string) (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "vl", name), nil
}
