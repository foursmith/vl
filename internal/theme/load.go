package theme

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed themes/*.json
var files embed.FS

// Theme is immutable after loading and can be shared by background syntax jobs.
// Type uses VS Code's extension uiTheme identifiers: vs, vs-dark, hc-light, hc-black.
type Theme struct {
	ID, Name, Type  string
	UIThemeOverride string
	colors          map[string]string
	tokens          []rawToken
	syntax          *scopeTrie
	palette         Palette
}

type rawToken struct {
	Scope    json.RawMessage `json:"scope"`
	Settings struct {
		Foreground    string  `json:"foreground"`
		Background    string  `json:"background"`
		Selection     string  `json:"selection"`
		LineHighlight string  `json:"lineHighlight"`
		FontStyle     *string `json:"fontStyle"`
	} `json:"settings"`
}

type themeDocument struct {
	Name        string                     `json:"name"`
	Type        string                     `json:"type"`
	Include     string                     `json:"include"`
	Colors      map[string]json.RawMessage `json:"colors"`
	TokenColors json.RawMessage            `json:"tokenColors"`
	Settings    []rawToken                 `json:"settings"`
}

var builtins sync.Map
var BuiltinIDs = []string{"dark_modern", "dark_plus", "dark_vs", "light_modern", "light_plus", "light_vs"}

func Builtin(id string) *Theme {
	if t, ok := builtins.Load(id); ok {
		return t.(*Theme)
	}
	kind := "vs-dark"
	if strings.HasPrefix(id, "light_") {
		kind = "vs"
	}
	t, err := load("themes/"+id+".json", kind, files.ReadFile, path.Join)
	if err != nil {
		panic(err)
	}
	t.ID = id
	actual, _ := builtins.LoadOrStore(id, t)
	return actual.(*Theme)
}

// Load accepts a builtin identifier or a local VS Code JSON/JSONC/tmTheme file.
// uiTheme may come from the extension's contributes.themes[].uiTheme. A theme's
// type is a standalone-file fallback; an explicit uiTheme or extension contribution wins.
func Load(name, uiTheme string) (*Theme, error) {
	switch uiTheme {
	case "", "vs", "vs-dark", "hc-light", "hc-black":
	default:
		return nil, fmt.Errorf("unknown uiTheme %q", uiTheme)
	}

	if strings.HasPrefix(name, localPrefix) {
		dir, err := LocalDir()
		if err != nil {
			return nil, err
		}
		return loadLocal(dir, name, uiTheme)
	}
	for _, id := range BuiltinIDs {
		if name == id {
			return Builtin(id), nil
		}
	}
	filename, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	explicitType := uiTheme
	metadata := extensionTheme(filename)
	if uiTheme == "" {
		uiTheme = metadata.UITheme
	}
	result, err := load(filename, uiTheme, readThemeFile, filepath.Join)
	if err == nil {
		result.UIThemeOverride = explicitType
	}
	if err == nil && metadata.Label != "" {
		result.Name = metadata.Label
	}
	return result, err
}

func readThemeFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("theme exceeds 8 MiB: %s", name)
	}
	return b, err
}

func load(filename, kind string, read func(string) ([]byte, error), join func(...string) string) (*Theme, error) {
	honorDeclaredType := kind == ""
	if kind == "" {
		kind = "vs-dark"
	}
	t := &Theme{ID: filename, Type: kind, colors: map[string]string{}}
	visiting := map[string]bool{}
	var visit func(string, int) error
	visit = func(name string, depth int) error {
		if depth >= 32 || visiting[name] {
			return fmt.Errorf("theme include cycle or depth limit: %s", name)
		}
		visiting[name] = true
		defer delete(visiting, name)
		data, err := read(name)
		if err != nil {
			return err
		}
		var d themeDocument
		if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
			d, err = readPlist(data)
		} else {
			var clean []byte
			clean, err = jsonc(data)
			if err == nil {
				if len(bytes.TrimSpace(clean)) == 0 || bytes.TrimSpace(clean)[0] != '{' {
					return fmt.Errorf("theme must be an object: %s", name)
				}
				err = json.Unmarshal(clean, &d)
			}
		}
		if err != nil {
			return fmt.Errorf("theme %s: %w", name, err)
		}
		if d.Include != "" {
			if err = visit(join(path.Dir(strings.ReplaceAll(name, "\\", "/")), d.Include), depth+1); err != nil {
				return err
			}
		}
		if d.Name != "" {
			t.Name = d.Name
		}
		if honorDeclaredType && depth == 0 && d.Type != "" {
			switch d.Type {
			case "light", "vs":
				t.Type = "vs"
			case "dark", "vs-dark":
				t.Type = "vs-dark"
			case "hc", "hc-black":
				t.Type = "hc-black"
			case "hcLight", "hc-light":
				t.Type = "hc-light"
			default:
				return fmt.Errorf("unknown theme type %q", d.Type)
			}
		}
		if d.Settings != nil {
			t.addLegacy(d.Settings)
			return nil
		}
		for key, raw := range d.Colors {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				continue
			}
			if value == "default" {
				delete(t.colors, key)
			} else if color := normalizeHex(value); color != "" {
				t.colors[key] = color
			}
		}
		if len(d.TokenColors) > 0 && string(d.TokenColors) != "null" {
			var ref string
			if json.Unmarshal(d.TokenColors, &ref) == nil {
				data, err := read(join(path.Dir(strings.ReplaceAll(name, "\\", "/")), ref))
				if err != nil {
					return err
				}
				legacy, err := readPlist(data)
				if err != nil {
					return fmt.Errorf("tokenColors %s: %w", ref, err)
				}
				t.addLegacy(legacy.Settings)
			} else {
				var entries []rawToken
				if err = json.Unmarshal(d.TokenColors, &entries); err != nil {
					return fmt.Errorf("tokenColors: %w", err)
				}
				t.tokens = append(t.tokens, entries...)
			}
		}
		return nil
	}
	if err := visit(filename, 0); err != nil {
		return nil, err
	}
	switch t.Type {
	case "vs", "vs-dark", "hc-black", "hc-light":
	default:
		return nil, fmt.Errorf("unknown uiTheme %q", t.Type)
	}
	if t.Name == "" {
		t.Name = path.Base(filename)
	}
	t.palette = t.makePalette()
	t.syntax = compileTokens(t.tokens)
	return t, nil
}

func (t *Theme) IsLight() bool    { return t.Type == "vs" || t.Type == "hc-light" }
func (t *Theme) Palette() Palette { return t.palette }

// Legacy TextMate settings are converted before normal VS Code color defaults.
func (t *Theme) addLegacy(tokens []rawToken) {
	for _, entry := range tokens {
		if len(entry.Scope) == 0 || string(entry.Scope) == `""` || string(entry.Scope) == "null" {
			if c := normalizeHex(entry.Settings.Foreground); c != "" {
				t.colors["editor.foreground"] = c
			}
			if c := normalizeHex(entry.Settings.Background); c != "" {
				t.colors["editor.background"] = c
			}
			if c := normalizeHex(entry.Settings.Selection); c != "" {
				t.colors["editor.selectionBackground"] = c
			}
			if c := normalizeHex(entry.Settings.LineHighlight); c != "" {
				t.colors["editor.lineHighlightBackground"] = c
			}
		} else {
			t.tokens = append(t.tokens, entry)
		}
	}
}
