package highlight

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
)

// These unmodified VS Code theme files are pinned and attributed in docs/THEME.md.
//
//go:embed themes/*.json
var themeFiles embed.FS

type FontStyle uint8

const (
	Bold FontStyle = 1 << iota
	Italic
	Underline
	Strikethrough
)

type tokenStyle struct {
	color string
	font  FontStyle
}
type tokenRule struct {
	scope      string
	parents    []string
	foreground string
	font       FontStyle
	setsFont   bool
}
type tokenTheme struct {
	foreground string
	rules      []tokenRule
}
type themeJSON struct {
	Include string            `json:"include"`
	Colors  map[string]string `json:"colors"`
	Tokens  []struct {
		Scope    json.RawMessage `json:"scope"`
		Settings struct {
			Foreground string  `json:"foreground"`
			FontStyle  *string `json:"fontStyle"`
		} `json:"settings"`
	} `json:"tokenColors"`
}

var modernTheme = loadModernTheme()

func loadModernTheme() tokenTheme {
	var theme tokenTheme
	var load func(string) error
	load = func(name string) error {
		data, err := themeFiles.ReadFile(name)
		if err != nil {
			return err
		}
		var file themeJSON
		if err = json.Unmarshal(data, &file); err != nil {
			return err
		}
		if file.Include != "" {
			if err = load(path.Join(path.Dir(name), file.Include)); err != nil {
				return err
			}
		}
		if fg := file.Colors["editor.foreground"]; fg != "" {
			theme.foreground = strings.ToUpper(fg)
		}
		for _, entry := range file.Tokens {
			var font FontStyle
			if entry.Settings.FontStyle != nil {
				for _, name := range strings.Fields(*entry.Settings.FontStyle) {
					switch name {
					case "bold":
						font |= Bold
					case "italic":
						font |= Italic
					case "underline":
						font |= Underline
					case "strikethrough":
						font |= Strikethrough
					}
				}
			}
			var selectors []string
			if len(entry.Scope) > 0 {
				var one string
				if json.Unmarshal(entry.Scope, &one) == nil {
					selectors = []string{one}
				} else if err = json.Unmarshal(entry.Scope, &selectors); err != nil {
					return err
				}
			}
			for _, group := range selectors {
				for _, selector := range strings.Split(group, ",") {
					parts := strings.Fields(selector)
					// The bundled theme only uses scope paths and ancestor selectors.
					if len(parts) == 0 {
						continue
					}
					theme.rules = append(theme.rules, tokenRule{
						scope: parts[len(parts)-1], parents: parts[:len(parts)-1],
						foreground: strings.ToUpper(entry.Settings.Foreground),
						font:       font, setsFont: entry.Settings.FontStyle != nil,
					})
				}
			}
		}
		return nil
	}
	if err := load("themes/dark_modern.json"); err != nil {
		panic(fmt.Sprintf("embedded Dark Modern theme: %v", err))
	}
	// Less specific rules apply first; stable order lets later inherited-theme
	// entries win ties. Font-only rules preserve the resolved foreground.
	slices.SortStableFunc(theme.rules, func(a, b tokenRule) int {
		if n := len(a.scope) - len(b.scope); n != 0 {
			return n
		}
		for i, j := len(a.parents)-1, len(b.parents)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
			if n := len(a.parents[i]) - len(b.parents[j]); n != 0 {
				return n
			}
		}
		return len(a.parents) - len(b.parents)
	})
	return theme
}

func scopeMatches(scope, selector string) bool {
	return scope == selector || strings.HasPrefix(scope, selector+".")
}

func (t tokenTheme) resolve(scopes []string) tokenStyle {
	style := tokenStyle{color: t.foreground}
	for level, scope := range scopes {
		for _, rule := range t.rules {
			if !scopeMatches(scope, rule.scope) {
				continue
			}
			ancestor := level - 1
			matched := true
			for parent := len(rule.parents) - 1; parent >= 0; parent-- {
				for ancestor >= 0 && !scopeMatches(scopes[ancestor], rule.parents[parent]) {
					ancestor--
				}
				if ancestor < 0 {
					matched = false
					break
				}
				ancestor--
			}
			if !matched {
				continue
			}
			if rule.foreground != "" {
				style.color = rule.foreground
			}
			if rule.setsFont {
				style.font = rule.font
			}
		}
	}
	return style
}
