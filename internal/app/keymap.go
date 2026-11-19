package app

import "charm.land/bubbles/v2/key"

func binding(display, description string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(display, description))
}

// Bindings also supply the shortcut labels shown in commands and menus.
var workbenchKeys = struct {
	Quit, Copy, Files, Recent, Commands                                         key.Binding
	Explorer, Search, Changes, History, Sidebar, Focus, Refresh, SidebarRefresh key.Binding
	PullRequests                                                                key.Binding
	DiffLayout, Save, Close, CloseOthers, Find, Escape, ContextMenu             key.Binding
}{
	Quit:         binding("Ctrl+Q", "Quit", "ctrl+q"),
	Copy:         binding("Ctrl+C", "Copy", "ctrl+c"),
	Files:        binding("Ctrl+P", "Go to File", "ctrl+p"),
	Recent:       binding("Ctrl+H", "Open Recent Workspace", "ctrl+h"),
	Commands:     binding("Ctrl+Shift+P", "Commands", "ctrl+shift+p", "f1"),
	Explorer:     binding("Ctrl+1", "Explorer", "ctrl+1"),
	Search:       binding("Ctrl+2", "Search", "ctrl+2", "ctrl+shift+f"),
	Changes:      binding("Ctrl+3", "Changes", "ctrl+3"),
	History:      binding("Ctrl+4", "History", "ctrl+4"),
	PullRequests: binding("Ctrl+5", "Pull Requests", "ctrl+5"),
	Sidebar:      binding("Ctrl+L", "Toggle sidebar", "ctrl+l"),
	Focus:        binding("F2", "Switch focus", "f2"),

	Refresh:        binding("F5", "Refresh", "f5"),
	SidebarRefresh: binding("r", "Refresh", "r", "R", "shift+r"),
	DiffLayout:     binding("Ctrl+D", "Toggle Inline View", "ctrl+d"),
	Save:           binding("Ctrl+S", "Save", "ctrl+s"),
	Close:          binding("Ctrl+W", "Close Editor", "ctrl+w"),
	CloseOthers:    binding("Ctrl+Shift+W", "Close Other Editors", "ctrl+shift+w", "ctrl+shift+W"),
	Find:           binding("Ctrl+F", "Find", "ctrl+f"),
	Escape:         binding("Esc", "Dismiss", "esc"),
	ContextMenu:    binding("Shift+F10", "Context menu", "shift+f10"),
}

// These workbench shortcuts take priority over both search input surfaces.
var searchWorkbenchKeys = []key.Binding{
	workbenchKeys.Quit, workbenchKeys.Files, workbenchKeys.Commands,
	workbenchKeys.Explorer, workbenchKeys.Search, workbenchKeys.Changes, workbenchKeys.History, workbenchKeys.PullRequests,
	workbenchKeys.Sidebar, workbenchKeys.Focus,
}

var searchKeys = struct {
	NextField, PreviousField, FocusQuery key.Binding
}{
	NextField:     binding("Tab", "Next field", "tab"),
	PreviousField: binding("Shift+Tab", "Previous field", "shift+tab"),
	FocusQuery:    binding("/", "Focus search query", "/"),
}

func shortcutHint(label string, binding key.Binding) string {
	return label + " (" + binding.Help().Key + ")"
}

var sidebarKeys = struct {
	Open, Edit, Delete, SplitFolders, Paste, OpenPullRequest key.Binding
}{
	Open:            binding("Enter", "Open selected item", "enter"),
	Edit:            binding("e", "Edit selected file", "e"),
	Delete:          binding("Del", "Delete selected item", "delete"),
	SplitFolders:    binding("L", "Split Folders", "L", "shift+l"),
	Paste:           binding("p", "Paste", "p"),
	OpenPullRequest: binding("o", "Open on GitHub", "o"),
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

const (
	stageKey                 = "s"
	stageScopeKey            = "S"
	changesCommitKey         = "c"
	discardKey               = "d"
	discardScopeKey          = "D"
	changesDiscardConfirmKey = "x"
	changesStashKey          = "t"
	changesFetchKey          = "f"
	changesPullKey           = "p"
	changesPushKey           = "P"
)
