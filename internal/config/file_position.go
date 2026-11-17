package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FilePositionStore shares file positions across all workspaces and editor modes.
// Each file has its own record, so saving one file cannot overwrite another.
type FilePositionStore struct{ Dir string }

type FilePosition struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	Top    int `json:"top,omitempty"`
	Left   int `json:"left,omitempty"`
}

func NewFilePositionStore() (*FilePositionStore, error) {
	dir, err := statePath("files")
	if err != nil {
		return nil, err
	}
	return &FilePositionStore{Dir: dir}, nil
}

func filePositionKey(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Clean(abs), nil
}

func (s *FilePositionStore) recordPath(path string) string {
	return filepath.Join(s.Dir, fmt.Sprintf("%x.json", sha256.Sum256([]byte(path))))
}

func (s *FilePositionStore) Load(path string) (FilePosition, bool, error) {
	key, err := filePositionKey(path)
	if err != nil {
		return FilePosition{}, false, err
	}
	data, err := os.ReadFile(s.recordPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return FilePosition{}, false, nil
	}
	if err != nil {
		return FilePosition{}, false, err
	}
	var position FilePosition
	if err := json.Unmarshal(data, &position); err != nil {
		return FilePosition{}, false, err
	}
	return position, true, nil
}

func (s *FilePositionStore) Save(path string, position FilePosition) error {
	key, err := filePositionKey(path)
	if err != nil {
		return err
	}
	return saveJSON(s.recordPath(key), position)
}
