package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/jsonc"
)

func (m *Model) LoadKeybindings() error {
	path, err := config.KeybindingsPath()
	if err == nil {
		m.workbenchKeymap, err = loadWorkbenchKeymap(path)
	}
	if err != nil {
		m.notice = m.tr("Invalid keybindings: %s", err.Error())
	}
	return err
}

func (m *Model) openKeybindings() tea.Cmd {
	path, err := config.KeybindingsPath()
	if err == nil {
		err = config.EnsureKeybindingsFile(path)
	}
	if err != nil {
		m.notice = m.tr("Open keyboard shortcuts file: %s", err.Error())
		return nil
	}
	return m.openFile(path)
}

func loadWorkbenchKeymap(path string) (*workbenchKeymap, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Command string          `json:"command"`
		Keys    json.RawMessage `json:"keys"`
		When    string          `json:"when"`
	}
	if err := jsonc.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	rules := slices.Clone(workbenchKeybindings)
	remaining := make(map[string]int, len(rules))
	for index, rule := range rules {
		remaining[rule.ruleID()] = index
	}
	var firstError error
	skipped := 0
	for i, entry := range entries {
		index, available := remaining[entry.Command]
		var shortcut key.Binding
		var err error
		switch {
		case !available:
			err = fmt.Errorf("unknown or repeated command %q", entry.Command)
		case entry.When != "":
			err = fmt.Errorf("command %q uses an unsupported when condition", entry.Command)
		default:
			shortcut, err = configuredBinding(entry.Keys)
		}
		if err != nil {
			skipped++
			if firstError == nil {
				firstError = fmt.Errorf("entry %d: %w", i+1, err)
			}
			continue
		}
		delete(remaining, entry.Command)
		// Keep the command and its context, and put overrides after defaults
		// so the last matching customized binding takes precedence.
		rule := rules[index]
		rule.binding = shortcut
		rules[index].binding = key.Binding{}
		rules = append(rules, rule)
	}
	keymap := compileWorkbenchKeymap(rules)
	if firstError != nil {
		return keymap, fmt.Errorf("%s: skipped %d entries (%w)", path, skipped, firstError)
	}
	return keymap, nil
}

func configuredBinding(raw json.RawMessage) (key.Binding, error) {
	var shortcuts []string
	var single string
	switch {
	case len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &single) == nil:
		if strings.TrimSpace(single) != "" {
			shortcuts = []string{single}
		}
	case len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, &shortcuts) == nil:
	default:
		return key.Binding{}, errors.New("keys must be a shortcut or an array of shortcuts")
	}
	var names []string
	var display string
	for _, shortcut := range shortcuts {
		shortcut = strings.TrimSpace(shortcut)
		chord, err := parseKeyChord(shortcut)
		if err != nil {
			return key.Binding{}, err
		}
		if display == "" {
			display = shortcut
			if chord.Mod == 0 && unicode.IsLetter(chord.Code) {
				display = string(chord.Code)
			}
		}
		names = append(names, chord.Keystroke())
		if chord.Mod.Contains(tea.ModShift) && unicode.IsLetter(chord.Code) {
			// Terminals can report a shifted letter as either case.
			chord.Code = unicode.ToUpper(chord.Code)
			names = append(names, chord.Keystroke())
			if chord.Mod == tea.ModShift {
				names = append(names, string(chord.Code))
			}
		}
	}
	if len(names) == 0 {
		return key.Binding{}, nil
	}
	return binding(display, "", names...), nil
}

var namedBindingKeys = func() map[string]rune {
	keys := make(map[string]rune)
	for _, code := range []rune{
		tea.KeyEscape, tea.KeyEnter, tea.KeyTab, tea.KeySpace, tea.KeyBackspace,
		tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight, tea.KeyInsert,
		tea.KeyDelete, tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown,
	} {
		keys[(tea.Key{Code: code}).Keystroke()] = code
	}
	for code := tea.KeyF1; code <= tea.KeyF63; code++ {
		keys[(tea.Key{Code: code}).Keystroke()] = code
	}
	keys["pageup"], keys["pagedown"] = tea.KeyPgUp, tea.KeyPgDown
	return keys
}()

var bindingModifiers = map[string]tea.KeyMod{
	"ctrl": tea.ModCtrl, "alt": tea.ModAlt, "shift": tea.ModShift,
	"meta": tea.ModMeta, "hyper": tea.ModHyper, "super": tea.ModSuper,
}

func parseKeyChord(display string) (tea.Key, error) {
	var chord tea.Key
	name := display
	for name != "+" {
		prefix, rest, found := strings.Cut(name, "+")
		if !found {
			break
		}
		mod := bindingModifiers[strings.ToLower(strings.TrimSpace(prefix))]
		if mod == 0 || chord.Mod.Contains(mod) {
			return tea.Key{}, fmt.Errorf("invalid shortcut %q", display)
		}
		chord.Mod |= mod
		name = strings.TrimSpace(rest)
	}
	chord.Code = namedBindingKeys[strings.ToLower(name)]
	if chord.Code == 0 {
		code, size := utf8.DecodeRuneInString(name)
		if size != len(name) || !unicode.IsGraphic(code) || name == "" {
			return tea.Key{}, fmt.Errorf("invalid shortcut %q", display)
		}
		chord.Code = unicode.ToLower(code)
	}
	return chord, nil
}
