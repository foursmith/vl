package theme

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/MoeForge/moego/json"
	"github.com/foursmith/vl/internal/jsonc"
)

// Theme is immutable after loading and can be shared by background syntax jobs.
// Type uses VS Code's extension uiTheme identifiers: vs, vs-dark, hc-light, hc-black.
type Theme struct {
	ID, Name, Type, VSCodeID string
	colors                   map[string]string
	syntax                   *scopeTrie
	palette                  Palette
}

type rawToken struct {
	Scope    json.RawMessage `json:"scope,omitempty"`
	Settings struct {
		Foreground    string  `json:"foreground,omitempty"`
		Background    string  `json:"background,omitempty"`
		Selection     string  `json:"selection,omitempty"`
		LineHighlight string  `json:"lineHighlight,omitempty"`
		FontStyle     *string `json:"fontStyle,omitempty"`
	} `json:"settings"`
}

type themeDocument struct {
	Name        string                     `json:"name"`
	Include     string                     `json:"include"`
	Colors      map[string]json.RawMessage `json:"colors"`
	TokenColors json.RawMessage            `json:"tokenColors"`
	Settings    []rawToken                 `json:"settings"`
}

// themeData is the self-contained format saved by the VS Code importer.
type themeData struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Colors      map[string]string `json:"colors"`
	TokenColors []rawToken        `json:"tokenColors"`
}

func (d themeData) theme(id string) *Theme {
	t := &Theme{ID: id, VSCodeID: d.ID, Name: d.Name, Type: d.Type, colors: d.Colors}
	t.palette = t.makePalette()
	t.syntax = compileTokens(d.TokenColors)
	return t
}

func validUITheme(kind string) bool {
	switch kind {
	case "vs", "vs-dark", "hc-black", "hc-light":
		return true
	default:
		return false
	}
}

// Load resolves a bundled or previously imported theme by ID.
func Load(id string) (*Theme, error) {
	if _, ok := builtinThemes[id]; ok {
		return Builtin(id), nil
	}
	return loadImportedID(id)
}

func readThemeFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readThemeData(f, name)
}

const maxThemeBytes = 8 << 20

func readThemeData(r io.Reader, name string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxThemeBytes+1))
	if len(b) > maxThemeBytes {
		return nil, fmt.Errorf("theme exceeds 8 MiB: %s", name)
	}
	return b, err
}

func sourcePath(name, ref string) string {
	return path.Join(path.Dir(name), strings.ReplaceAll(ref, "\\", "/"))
}

// loadSource resolves VS Code includes and TextMate files at the import boundary.
// The contribution (or bundled metadata) supplies uiTheme.
func loadSource(filename, kind string, read func(string) ([]byte, error)) (themeData, error) {
	if !validUITheme(kind) {
		return themeData{}, fmt.Errorf("unknown uiTheme %q", kind)
	}
	resolved := themeData{Type: kind, Colors: make(map[string]string)}
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
		var d *themeDocument
		if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
			var parsed themeDocument
			parsed, err = readPlist(data)
			d = &parsed
		} else {
			err = jsonc.Unmarshal(data, &d)
		}
		if err != nil {
			return fmt.Errorf("theme %s: %w", name, err)
		}
		if d == nil {
			return fmt.Errorf("theme must be an object: %s", name)
		}
		if d.Include != "" {
			if err = visit(sourcePath(name, d.Include), depth+1); err != nil {
				return err
			}
		}
		if d.Name != "" {
			resolved.Name = d.Name
		}
		if d.Settings != nil {
			resolved.addTextMateSettings(d.Settings)
			return nil
		}
		for key, raw := range d.Colors {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				continue
			}
			if value == "default" {
				delete(resolved.Colors, key)
			} else if color := normalizeHex(value); color != "" {
				resolved.Colors[key] = color
			}
		}
		if len(d.TokenColors) > 0 && string(d.TokenColors) != "null" {
			var ref string
			if json.Unmarshal(d.TokenColors, &ref) == nil {
				data, err := read(sourcePath(name, ref))
				if err != nil {
					return err
				}
				tokens, err := readPlist(data)
				if err != nil {
					return fmt.Errorf("tokenColors %s: %w", ref, err)
				}
				resolved.addTextMateSettings(tokens.Settings)
			} else {
				var entries []rawToken
				if err = json.Unmarshal(d.TokenColors, &entries); err != nil {
					return fmt.Errorf("tokenColors: %w", err)
				}
				resolved.TokenColors = append(resolved.TokenColors, entries...)
			}
		}
		return nil
	}
	if err := visit(filename, 0); err != nil {
		return themeData{}, err
	}
	if resolved.Name == "" {
		resolved.Name = path.Base(filename)
	}
	return resolved, nil
}

func (t *Theme) IsLight() bool    { return t.Type == "vs" || t.Type == "hc-light" }
func (t *Theme) Palette() Palette { return t.palette }

// TextMate settings supply editor colors as well as scoped token rules.
func (d *themeData) addTextMateSettings(tokens []rawToken) {
	for _, entry := range tokens {
		if len(entry.Scope) == 0 || string(entry.Scope) == `""` || string(entry.Scope) == "null" {
			if c := normalizeHex(entry.Settings.Foreground); c != "" {
				d.Colors["editor.foreground"] = c
			}
			if c := normalizeHex(entry.Settings.Background); c != "" {
				d.Colors["editor.background"] = c
			}
			if c := normalizeHex(entry.Settings.Selection); c != "" {
				d.Colors["editor.selectionBackground"] = c
			}
			if c := normalizeHex(entry.Settings.LineHighlight); c != "" {
				d.Colors["editor.lineHighlightBackground"] = c
			}
		} else {
			d.TokenColors = append(d.TokenColors, entry)
		}
	}
}
