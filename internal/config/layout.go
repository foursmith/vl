package config

import (
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
)

// Layout stores user preferences independently of the current terminal size.
type Layout struct {
	Language              string            `json:"language,omitempty"`
	LastCommand           string            `json:"lastCommand,omitempty"`
	DarkTheme             string            `json:"darkTheme,omitempty"`
	LightTheme            string            `json:"lightTheme,omitempty"`
	Theme                 string            `json:"theme,omitempty"`
	ThemePinned           bool              `json:"themePinned,omitempty"`
	StatusBarStyle        string            `json:"statusBarStyle,omitempty"`
	SidebarWidth          int               `json:"sidebarWidth"`
	SidebarRight          bool              `json:"sidebarRight"`
	AutoSwitchInputMethod InputMethodSwitch `json:"autoSwitchInputMethod,omitzero"`
}

type InputMethodSwitch struct {
	Enable bool     `json:"enable"`
	Cmd    []string `json:"cmd,omitempty"`
}

func (l Layout) Equal(other Layout) bool { return reflect.DeepEqual(l, other) }

func LayoutPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vl.json"), nil
}

func LoadLayout(path string) (Layout, error) {
	var layout Layout
	err := loadJSON(path, &layout)
	return layout, err
}

func SaveLayout(path string, layout Layout) error { return saveJSON(path, layout) }

// LayoutStore orders asynchronous writes, including across workspace switches.
// A prepared write becomes obsolete when a newer snapshot is prepared.
type LayoutStore struct {
	path     string
	revision atomic.Uint64
	mu       sync.Mutex
}

func NewLayoutStore(path string) *LayoutStore { return &LayoutStore{path: path} }

func (s *LayoutStore) Prepare(layout Layout) func() error {
	revision := s.revision.Add(1)
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if revision != s.revision.Load() {
			return nil
		}
		return SaveLayout(s.path, layout)
	}
}
