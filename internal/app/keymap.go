package app

import (
	"slices"

	"charm.land/bubbles/v2/key"
)

func binding(display, description string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(display, description))
}

// Input handlers that already decoded the event reuse its key string.
func matchesShortcut(name string, binding key.Binding) bool {
	return binding.Enabled() && slices.Contains(binding.Keys(), name)
}

// App-level bindings are grouped by the surface that owns the input.
// Commands, menus and hints use the same binding for their display.
const (
	sidebarRefreshRuleID = "git.refresh.sidebar"
	leftArrow            = "left"
	rightArrow           = "right"
)

var dismissKey = binding("Esc", "Dismiss", "esc")

// Declare defaults in precedence order. The last matching rule wins, so
// context-specific rules can share a key with a workbench fallback.
var workbenchKeybindings = []workbenchShortcut{
	{binding: binding("Ctrl+Q", "Quit", "ctrl+q"), context: fromSearch, command: commandQuit},
	{id: "app.quit.copyFallback", binding: binding("Ctrl+C", "Quit", "ctrl+c"), command: commandQuit, hideHint: true, when: func(m *Model) bool {
		return !m.contentCopyFocused()
	}},
	{binding: binding("Ctrl+C", "Copy", "ctrl+c"), command: commandCopy, when: (*Model).contentCopyFocused},
	{binding: binding("Ctrl+G", "Go to File", "ctrl+g"), context: fromSearch, command: commandFiles},
	{binding: binding("Ctrl+R", "Open Recent Workspace", "ctrl+r"), context: workspaceOnly | fromSidebarSearch, command: commandRecent},
	{binding: binding("Ctrl+P", "Commands", "ctrl+p"), context: fromSearch, command: commandPalette},
	{binding: binding("Ctrl+2", "Search", "ctrl+2"), context: workspaceOnly | fromSearch, command: commandSearch},
	{binding: binding("Ctrl+1", "Explorer", "ctrl+1"), context: workspaceOnly | fromSearch, command: commandExplorer},
	{binding: binding("Ctrl+3", "Changes", "ctrl+3"), context: workspaceOnly | fromSearch, command: commandChanges},
	{binding: binding("Ctrl+4", "History", "ctrl+4"), context: workspaceOnly | fromSearch, command: commandHistory},
	{binding: binding("Ctrl+5", "Pull Requests", "ctrl+5"), context: workspaceOnly | fromSearch, command: commandPullRequests},
	{binding: binding("Ctrl+L", "Toggle sidebar", "ctrl+l"), context: workspaceOnly | fromSearch, command: commandSidebar},
	{binding: binding("F2", "Switch focus", "f2"), context: workspaceOnly | fromSearch, command: commandFocus},
	{binding: binding("F5", "Refresh", "f5"), context: fromFileFind | blurFileFind, command: commandRefresh},
	{id: sidebarRefreshRuleID, binding: binding("r", "Refresh", "r", "R", "shift+r"), command: commandRefresh, when: func(m *Model) bool {
		return m.sidebar && m.section != search
	}},
	{context: workspaceOnly, command: commandDiffLayout},
	{binding: binding("Ctrl+S", "Save", "ctrl+s"), context: fromSidebarSearch | fromFileFind, command: commandSave},
	{binding: binding("Ctrl+W", "Close Editor", "ctrl+w"), context: fromSidebarSearch, command: commandClose},
	{binding: binding("Ctrl+Shift+W", "Close Other Editors", "ctrl+shift+w", "ctrl+shift+W"), context: beforeInput, command: commandCloseOthers},
	{command: commandCloseAll},
	{binding: binding("Ctrl+F", "Find", "ctrl+f"), command: commandFind},
	{binding: dismissKey, command: commandFocusSidebar},
	{binding: binding("F7", "Context menu", "f7"), command: commandContextMenu},
}

var editorKeys = struct {
	Cut, Paste, SelectAll, Undo, Redo key.Binding
}{
	Cut:       binding("Ctrl+X", "Cut", "ctrl+x"),
	Paste:     binding("Ctrl+V", "Paste", "ctrl+v"),
	SelectAll: binding("Ctrl+A", "Select All", "ctrl+a"),
	Undo:      binding("Ctrl+Z", "Undo", "ctrl+z"),
	Redo:      binding("Ctrl+Shift+Z", "Redo", "ctrl+shift+z", "ctrl+shift+Z"),
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

var pickerKeys = struct{ Up, Down, First, Last, Accept key.Binding }{
	Up:     binding("↑", "Previous", "up"),
	Down:   binding("↓", "Next", "down"),
	First:  binding("Home", "First", "home"),
	Last:   binding("End", "Last", "end"),
	Accept: binding("Enter", "Select", "enter"),
}

// pageDelta uses the height of the focused view, excluding its chrome.
func pageDelta(k string, height int) int {
	switch k {
	case "ctrl+u":
		return -max(1, height/2)
	case "ctrl+d":
		return max(1, height/2)
	case "pgup":
		return -height
	case "pgdown":
		return height
	default:
		return 0
	}
}

func isHalfPageKey(k string) bool { return k == "ctrl+u" || k == "ctrl+d" }

const (
	navigationFirstPrefix  = "g"
	navigationDeletePrefix = "d"
	navigationCopyPrefix   = "y"
	navigationLeftLetter   = "h"
	navigationRightLetter  = "l"
	sidebarDeleteSequence  = navigationDeletePrefix + navigationDeletePrefix
	sidebarCopySequence    = navigationCopyPrefix + navigationCopyPrefix
)

var navigationKeys = struct {
	Left, Right, Up, Down, Last key.Binding
}{
	Left:  binding("←", "Left", leftArrow, navigationLeftLetter),
	Right: binding("→", "Right", rightArrow, navigationRightLetter),
	Up:    binding("↑", "Up", "up", "k"),
	Down:  binding("↓", "Down", "down", "j"),
	Last:  binding("G", "Last", "G"),
}

var dialogKeys = struct{ Accept, Save, Discard, Toggle key.Binding }{
	Accept:  binding("Enter", "Confirm", "enter", "space"),
	Save:    binding("s", "Save", "s"),
	Discard: binding("d", "Don't Save", "d"),
	Toggle:  binding("Space", "Toggle option", "space", " "),
}

var completionAccept = binding("Tab", "Insert completion", "tab")
var eraseCharacter = binding("Backspace", "Delete previous character", "backspace")

var gitKeys = struct {
	Stage, StageScope, Discard, DiscardScope key.Binding
	Commit, Stash, Fetch, Pull, Push         key.Binding
}{
	Stage:        binding("s", "Stage or unstage", "s"),
	StageScope:   binding("S", "Stage or unstage scope", "S", "shift+s"),
	Discard:      binding("d", "Discard changes", "d"),
	DiscardScope: binding("D", "Discard changes in scope", "D", "shift+d"),
	Commit:       binding("c", "Commit staged changes", "c"),
	Stash:        binding("t", "Stash changes", "t"),
	Fetch:        binding("f", "Fetch", "f"),
	Pull:         binding("p", "Pull", "p"),
	Push:         binding("P", "Push", "P", "shift+p"),
}

func isDiffGitShortcut(name string) bool {
	for _, shortcut := range [...]key.Binding{gitKeys.Stage, gitKeys.StageScope, gitKeys.Discard, gitKeys.DiscardScope} {
		if matchesShortcut(name, shortcut) {
			return true
		}
	}
	return false
}

var leaderKeys = struct {
	Prefix, Previous, Next, Save, Close, SaveClose key.Binding
}{
	Prefix:    binding("Space", "Leader", "space", " "),
	Previous:  binding("←", "Previous tab", leftArrow),
	Next:      binding("→", "Next tab", rightArrow),
	Save:      binding("w", "Save", "w"),
	Close:     binding("q", "Close editor", "q"),
	SaveClose: binding("z", "Save and close editor", "z"),
}

var imageKeys = struct {
	ZoomIn, ZoomOut, Fit, Keep key.Binding
}{
	ZoomIn:  binding("+", "Zoom in", "+", "="),
	ZoomOut: binding("-", "Zoom out", "-"),
	Fit:     binding("0", "Fit to Window", "0"),
	Keep:    binding("Enter", "Keep editor", "enter"),
}
