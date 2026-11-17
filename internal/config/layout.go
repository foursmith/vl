package config

import (
	"sync"
	"sync/atomic"
)

// Layout stores user preferences independently of the current terminal size.
type Layout struct {
	Language           string `json:"language,omitempty"`
	WorkspaceCollapsed bool   `json:"workspaceCollapsed,omitempty"`
	LastCommand        string `json:"lastCommand,omitempty"`
	DarkTheme          string `json:"darkTheme,omitempty"`
	LightTheme         string `json:"lightTheme,omitempty"`
	DarkThemeType      string `json:"darkThemeType,omitempty"`
	LightThemeType     string `json:"lightThemeType,omitempty"`
	ThemeType          string `json:"themeType,omitempty"`
	Theme              string `json:"theme,omitempty"`
	SidebarWidth       int    `json:"sidebarWidth"`
	SidebarRight       bool   `json:"sidebarRight"`
}

func LayoutPath() (string, error) { return configPath("layout.json") }

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
