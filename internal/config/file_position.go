package config

import (
	"errors"
	"os"
	"slices"

	"github.com/MoeForge/moego/json"
)

// FilePositionStore shares file positions across all workspaces and editor modes.
// Each file has its own record, so saving one file cannot overwrite another.
type FilePositionStore struct{ Dir string }

type FilePosition struct {
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Top    int    `json:"top,omitempty"`
	Left   int    `json:"left,omitempty"`
	Folds  []Fold `json:"folds,omitempty"`
}

type Fold struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	First string `json:"first"`
	Last  string `json:"last"`
}

func (p FilePosition) Equal(other FilePosition) bool {
	return p.Line == other.Line && p.Column == other.Column && p.Top == other.Top && p.Left == other.Left && slices.Equal(p.Folds, other.Folds)
}

func NewFilePositionStore() (*FilePositionStore, error) {
	dir, err := statePath("files")
	if err != nil {
		return nil, err
	}
	return &FilePositionStore{Dir: dir}, nil
}

func (s *FilePositionStore) Load(path string) (FilePosition, bool, error) {
	record, err := fileStatePath(s.Dir, path)
	if err != nil {
		return FilePosition{}, false, err
	}
	data, err := os.ReadFile(record)
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
	record, err := fileStatePath(s.Dir, path)
	if err != nil {
		return err
	}
	return saveJSON(record, position)
}
