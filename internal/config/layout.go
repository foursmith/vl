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
	Appearance            Appearance        `json:"appearance,omitzero"`
	Vim                   VimPreferences    `json:"vim,omitzero"`
	AutoSwitchInputMethod InputMethodSwitch `json:"autoSwitchInputMethod,omitzero"`
}

type Appearance struct {
	Theme          ThemePreferences   `json:"theme,omitzero"`
	StatusBarStyle string             `json:"statusBarStyle,omitempty"`
	Sidebar        SidebarPreferences `json:"sidebar,omitzero"`
}

type ThemePreferences struct {
	Dark  string `json:"dark,omitempty"`
	Light string `json:"light,omitempty"`
}

type SidebarPreferences struct {
	Side   string `json:"side,omitempty"`
	Width  int    `json:"width,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
}

type VimPreferences struct {
	EnableJieba bool `json:"enableJieba,omitempty"`
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
	return filepath.Join(dir, "vl.config.json"), nil
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
	saved    *Layout
}

func NewLayoutStore(path string) *LayoutStore { return &LayoutStore{path: path} }

func (s *LayoutStore) HasPreparedWrite() bool { return s.revision.Load() != 0 }

func (s *LayoutStore) Prepare(layout Layout) func() error {
	revision := s.revision.Add(1)
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if revision != s.revision.Load() || s.saved != nil && layout.Equal(*s.saved) {
			return nil
		}
		if err := SaveLayout(s.path, layout); err != nil {
			return err
		}
		s.saved = &layout
		return nil
	}
}
