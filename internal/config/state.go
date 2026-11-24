package config

import (
	"crypto/sha256"
	"fmt"
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

type CommandState struct {
	LastCommand string `json:"lastCommand,omitempty"`
}

func CommandStatePath() (string, error) { return statePath("state.json") }

func LoadCommandState(path string) (CommandState, error) {
	var state CommandState
	err := loadJSON(path, &state)
	return state, err
}

func SaveCommandState(path string, state CommandState) error { return saveJSON(path, state) }

func fileStatePath(dir, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Join(dir, fmt.Sprintf("%x.json", sha256.Sum256([]byte(filepath.Clean(abs))))), nil
}
