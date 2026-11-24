package app

import (
	"fmt"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type shortcutContext uint8

const (
	workspaceOnly shortcutContext = 1 << iota
	fromSidebarSearch
	fromFileFind
	blurFileFind
	beforeInput
)

const (
	fromSearchFields = fromSidebarSearch | fromFileFind
	fromSearch       = fromSearchFields | blurFileFind
)

type workbenchShortcut struct {
	id      string
	binding key.Binding
	context shortcutContext
	command commandID
	when    func(*Model) bool
}

func (s workbenchShortcut) has(context shortcutContext) bool {
	return s.context&context != 0
}

type workbenchKeymap struct {
	byKey     map[string][]workbenchShortcut
	byCommand map[commandID][]workbenchShortcut
	byID      map[string]workbenchShortcut
}

type workbenchKeyOverride struct {
	id      string
	binding key.Binding // An empty binding removes this rule.
}

var defaultWorkbenchKeymap = compileWorkbenchKeymap(workbenchKeybindings)

// User overrides can assign, replace or remove keys by stable rule ID.
// They keep the command and focus conditions and run after the defaults.
func buildWorkbenchKeymap(overrides []workbenchKeyOverride) (*workbenchKeymap, error) {
	rules := slices.Clone(workbenchKeybindings)
	remaining := make(map[string]int, len(rules))
	for i, rule := range rules {
		remaining[rule.ruleID()] = i
	}
	for _, override := range overrides {
		index, known := remaining[override.id]
		if !known {
			return nil, fmt.Errorf("unknown or repeated keybinding %q", override.id)
		}
		delete(remaining, override.id)
		rule := rules[index]
		rules[index].binding = key.Binding{}
		rule.binding = override.binding
		rules = append(rules, rule)
	}
	return compileWorkbenchKeymap(rules), nil
}

func (s workbenchShortcut) ruleID() string {
	if s.id != "" {
		return s.id
	}
	return string(s.command)
}

func compileWorkbenchKeymap(rules []workbenchShortcut) *workbenchKeymap {
	keymap := &workbenchKeymap{
		byKey:     make(map[string][]workbenchShortcut),
		byCommand: make(map[commandID][]workbenchShortcut),
		byID:      make(map[string]workbenchShortcut),
	}
	for _, rule := range rules {
		if !rule.binding.Enabled() {
			continue
		}
		id := rule.ruleID()
		if _, exists := keymap.byID[id]; exists {
			panic("duplicate keybinding ID: " + id)
		}
		rule.binding.SetKeys(slices.Clone(rule.binding.Keys())...)
		keymap.byID[id] = rule
		keymap.byCommand[rule.command] = append(keymap.byCommand[rule.command], rule)
		for _, name := range rule.binding.Keys() {
			keymap.byKey[name] = append(keymap.byKey[name], rule)
		}
	}
	return keymap
}

func (m *Model) activeWorkbenchKeymap() *workbenchKeymap {
	if m.workbenchKeymap == nil {
		return defaultWorkbenchKeymap
	}
	return m.workbenchKeymap
}

func (m *Model) resolveWorkbenchShortcut(name string) workbenchShortcut {
	return m.resolveWorkbenchRule(name, func(rule workbenchShortcut) bool {
		if m.editorOnly && rule.has(workspaceOnly) {
			return false
		}
		if rule.has(beforeInput) {
			return true
		}
		if m.fileFindFocused() && !rule.has(fromFileFind) ||
			m.sidebar && m.section == search && !rule.has(fromSidebarSearch) {
			return false
		}
		if m.sidebar {
			// Check local navigation without consuming its pending sequence.
			navigation := m.navigationState
			scope := ""
			if navigation.pending != "" || navigation.count != 0 {
				scope = m.sidebarNavigationScope(*m.sidebarList().selected)
			}
			if _, _, handled := navigation.key(scope, name); handled {
				return false
			}
		}
		return true
	})
}

func (s workbenchShortcut) applies(m *Model) bool {
	return s.when == nil || s.when(m)
}

// Widgets explicitly opt in to individual workbench commands.
func (m *Model) matchWorkbenchCommand(name string, id commandID) bool {
	return m.widgetWorkbenchCommand(name, id) == id
}

func (m *Model) widgetWorkbenchCommand(name string, accepted ...commandID) commandID {
	return m.resolveWorkbenchRule(name, func(rule workbenchShortcut) bool {
		return slices.Contains(accepted, rule.command)
	}).command
}

func (m *Model) resolveWorkbenchRule(name string, accepts func(workbenchShortcut) bool) workbenchShortcut {
	for _, rule := range slices.Backward(m.activeWorkbenchKeymap().byKey[name]) {
		if rule.applies(m) && (accepts == nil || accepts(rule)) {
			return rule
		}
	}
	return workbenchShortcut{}
}

func (m *Model) workbenchRuleHint(id string) string {
	return m.activeWorkbenchKeymap().byID[id].binding.Help().Key
}

func (m *Model) contentCopyFocused() bool {
	return m.textEditFocused() || m.modal == "" && !m.sidebar && m.diffOpen
}

func (m *Model) toggleEditorFocus() {
	if m.sidebar {
		m.blurFileFind()
		m.focusEditor()
		return
	}
	m.sidebar = true
	m.sidebarVisible = true
	m.resize()
}

func (m *Model) openFocusedContextMenu() tea.Cmd {
	if m.fileFindFocused() {
		return m.openContextMenu(m.fileFindX()+2, m.contentRow()+fileFindQueryRow)
	}
	if m.sidebar {
		if m.section == search && m.searchField(m.search.focus) != nil {
			return m.openContextMenu(m.sideX()+2, m.searchPanelLayout().query)
		}
		return m.openSidebarContextMenu()
	}
	if cursor := m.editorCursor(); cursor != nil {
		return m.openContextMenu(cursor.X, cursor.Y)
	}
	return m.openContextMenu(m.mainX()+2, m.contentRow())
}
