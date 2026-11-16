package app

import "charm.land/bubbles/v2/key"

func binding(display, description string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(display, description))
}

// Bindings also supply the shortcut labels shown in commands and menus.
var workbenchKeys = struct {
	Quit, Copy, Files, Recent, Commands, Position, Search, Replace                     key.Binding
	Explorer, Changes, History, Sidebar, Focus, Output, Refresh                        key.Binding
	DiffLayout, Save, Close, NextTab, PreviousTab, Find, FindNext, Escape, ContextMenu key.Binding
}{
	Quit:        binding("Ctrl+Q", "Quit", "ctrl+q"),
	Copy:        binding("Ctrl+C", "Copy", "ctrl+c"),
	Files:       binding("Ctrl+P", "Search files", "ctrl+p"),
	Recent:      binding("Ctrl+R", "Recent folders", "ctrl+r"),
	Commands:    binding("F1", "Commands", "ctrl+shift+p", "f1"),
	Position:    binding("Ctrl+G", "Go to line", "ctrl+g"),
	Search:      binding("Ctrl+Shift+F", "Search workspace", "ctrl+shift+f", "ctrl+4"),
	Replace:     binding("Ctrl+Shift+H", "Replace in workspace", "ctrl+shift+h"),
	Explorer:    binding("Ctrl+1", "Explorer", "ctrl+1"),
	Changes:     binding("Ctrl+2", "Changes", "ctrl+2"),
	History:     binding("Ctrl+3", "History", "ctrl+3"),
	Sidebar:     binding("Ctrl+L", "Toggle sidebar", "ctrl+l"),
	Focus:       binding("F6", "Switch focus", "f6"),
	Output:      binding("Ctrl+J", "Toggle output", "ctrl+j"),
	Refresh:     binding("F5", "Refresh", "f5"),
	DiffLayout:  binding("Ctrl+D", "Toggle diff layout", "ctrl+d"),
	Save:        binding("Ctrl+S", "Save", "ctrl+s"),
	Close:       binding("Ctrl+W", "Close tab", "ctrl+w"),
	NextTab:     binding("Alt+Right", "Next tab", "alt+right"),
	PreviousTab: binding("Alt+Left", "Previous tab", "alt+left"),
	Find:        binding("Ctrl+F", "Find", "ctrl+f"),
	FindNext:    binding("F3", "Next match", "f3"),
	Escape:      binding("Esc", "Dismiss", "esc"),
	ContextMenu: binding("Shift+F10", "Context menu", "shift+f10"),
}

var pickerKeys = struct{ Up, Down, PageUp, PageDown, First, Last, Accept key.Binding }{
	Up:       binding("↑", "Previous", "up"),
	Down:     binding("↓", "Next", "down"),
	PageUp:   binding("PgUp", "Previous page", "pgup"),
	PageDown: binding("PgDn", "Next page", "pgdown"),
	First:    binding("Home", "First", "home"),
	Last:     binding("End", "Last", "end"),
	Accept:   binding("Enter", "Select", "enter"),
}
