package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/MoeForge/moego/json"
	"github.com/foursmith/vl/internal/jsonc"
)

// Preferences stores user preferences independently of the current terminal size.
type Preferences struct {
	Language   string     `json:"language,omitempty"`
	Appearance Appearance `json:"appearance,omitzero"`
	Features   Features   `json:"features,omitzero"`
}

type Features struct {
	VimMode                 *bool             `json:"vimMode,omitempty"`
	ChineseWordSegmentation bool              `json:"chineseWordSegmentation,omitempty"`
	AutoSwitchInputMethod   InputMethodSwitch `json:"autoSwitchInputMethod,omitzero"`
}

func (f Features) VimEnabled() bool { return f.VimMode == nil || *f.VimMode }

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

type InputMethodSwitch struct {
	Enable bool     `json:"enable"`
	Cmd    []string `json:"cmd,omitempty"`
}

func (l Preferences) Equal(other Preferences) bool { return reflect.DeepEqual(l, other) }

func PreferencesPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vl.config.json"), nil
}

func LoadPreferences(path string) (Preferences, error) {
	data, err := readPreferencesFile(path)
	if err != nil {
		return Preferences{}, err
	}
	return decodePreferences(data)
}

const emptyPreferences = "{\n}\n"

// EnsurePreferencesFile creates a default configuration without replacing an existing file.
func EnsurePreferencesFile(path string) error {
	return ensureConfigFile(path, emptyPreferences)
}

func ensureConfigFile(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = file.WriteString(contents)
	return errors.Join(err, file.Close())
}

func readPreferencesFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte(emptyPreferences), nil
	}
	return data, err
}

func decodePreferences(data []byte) (Preferences, error) {
	var preferences Preferences
	err := jsonc.Unmarshal(data, &preferences)
	return preferences, err
}

// Update only changed leaf settings, leaving comments and unknown fields in place.
// Write default values explicitly when resetting a setting so its comment remains.
func updatePreferences(data []byte, path []string, before, after reflect.Value) ([]byte, error) {
	if reflect.DeepEqual(before.Interface(), after.Interface()) {
		return data, nil
	}
	if after.Kind() != reflect.Struct {
		value, err := json.Marshal(after.Interface())
		if err != nil {
			return nil, err
		}
		return jsonc.Set(data, path, value)
	}
	for i := range after.NumField() {
		name, _, _ := strings.Cut(after.Type().Field(i).Tag.Get("json"), ",")
		var err error
		data, err = updatePreferences(data, append(path, name), before.Field(i), after.Field(i))
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// PreferencesStore orders asynchronous writes, including across workspace switches.
// A prepared write becomes obsolete when a newer snapshot is prepared.
type PreferencesStore struct {
	path     string
	revision atomic.Uint64
	mu       sync.Mutex
	saved    *Preferences
	disk     []byte
}

func NewPreferencesStore(path string) *PreferencesStore { return &PreferencesStore{path: path} }

func (s *PreferencesStore) Load() (Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := readPreferencesFile(s.path)
	if err != nil {
		return Preferences{}, err
	}
	preferences, err := decodePreferences(data)
	if err == nil {
		s.disk = data
		s.saved = &preferences
	}
	return preferences, err
}

// SetBaseline records the preferences as restored by the app, where default
// values may be canonicalized. Loading alone must never rewrite those settings.
func (s *PreferencesStore) SetBaseline(preferences Preferences) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = &preferences
}

func (s *PreferencesStore) EnsureFile() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.path, EnsurePreferencesFile(s.path)
}

// SaveFile serializes editor saves of this file with automatic preference writes.
func (s *PreferencesStore) SaveFile(path string, save func() error) error {
	if real, err := filepath.EvalSymlinks(s.path); err == nil && path == real {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	return save()
}

func (s *PreferencesStore) Prepare(preferences Preferences) func() error {
	revision := s.revision.Add(1)
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if revision != s.revision.Load() || s.saved != nil && preferences.Equal(*s.saved) {
			return nil
		}
		current, err := readPreferencesFile(s.path)
		if err != nil {
			return err
		}
		if s.disk != nil && !bytes.Equal(current, s.disk) {
			return errors.New("configuration file changed on disk; run Reload VL to apply it")
		}
		baseline := s.saved
		if baseline == nil {
			loaded, err := decodePreferences(current)
			if err != nil {
				return err
			}
			baseline = &loaded
		}
		data, err := updatePreferences(current, nil, reflect.ValueOf(*baseline), reflect.ValueOf(preferences))
		if err != nil {
			return err
		}
		if !bytes.Equal(data, current) {
			path := s.path
			if target, err := filepath.EvalSymlinks(path); err == nil {
				path = target
			}
			if err := writeConfigFile(path, data); err != nil {
				return err
			}
		}
		s.disk = data
		s.saved = &preferences
		return nil
	}
}
