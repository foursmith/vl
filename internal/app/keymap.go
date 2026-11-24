package app

import (
	"slices"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
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
	upArrow              = "up"
	downArrow            = "down"
)

var dismissKey = binding("Esc", "Dismiss", "esc")

// Declare defaults in precedence order. The last matching rule wins, so
// context-specific rules can share a key with a workbench fallback.
var workbenchKeybindings = []workbenchShortcut{
	{binding: binding("Ctrl+Q", "Quit", "ctrl+q"), context: fromSearch, command: commandQuit},
	{binding: binding("Ctrl+C", "Copy", "ctrl+c"), context: fromSearchFields, command: commandCopy, when: (*Model).contentCopyFocused},
	{binding: binding("Ctrl+X", "Cut", "ctrl+x"), context: fromSearchFields, command: commandCut, when: (*Model).textEditFocused},
	{binding: binding("Ctrl+V", "Paste", "ctrl+v"), context: fromSearchFields, command: commandPaste, when: (*Model).textEditFocused},
	{binding: binding("Ctrl+A", "Select All", "ctrl+a"), context: fromSearchFields, command: commandSelectAll, when: (*Model).textEditFocused},
	{binding: binding("Ctrl+Z", "Undo", "ctrl+z"), context: fromSearchFields, command: commandUndo, when: (*Model).textEditFocused},
	{binding: binding("Ctrl+Shift+Z", "Redo", "ctrl+shift+z", "ctrl+shift+Z"), context: fromSearchFields, command: commandRedo, when: (*Model).textEditFocused},
	{binding: binding("Ctrl+G", "Go to File", "ctrl+g"), context: fromSearch, command: commandFiles},
	{binding: binding("Ctrl+R", "Open Recent Workspace", "ctrl+r"), context: workspaceOnly | fromSidebarSearch, command: commandRecent},
	{binding: binding("Ctrl+P", "Commands", "ctrl+p"), context: fromSearch, command: commandPalette},
	{binding: binding("Ctrl+2", "Search", "ctrl+2"), context: workspaceOnly | fromSearch, command: commandSearch},
	{binding: binding("Ctrl+1", "Explorer", "ctrl+1"), context: workspaceOnly | fromSearch, command: commandExplorer},
	{binding: binding("Ctrl+3", "Changes", "ctrl+3"), context: workspaceOnly | fromSearch, command: commandChanges},
	{binding: binding("Ctrl+4", "History", "ctrl+4"), context: workspaceOnly | fromSearch, command: commandHistory},
	{binding: binding("Ctrl+5", "Pull Requests", "ctrl+5"), context: workspaceOnly | fromSearch, command: commandPullRequests},
	{binding: binding("F12", "Toggle sidebar visibility", "f12"), context: workspaceOnly | fromSearch, command: commandSidebar},
	{binding: binding("F2", "Switch focus", "f2"), context: workspaceOnly | fromSearch, command: commandFocus},
	{binding: binding("F5", "Refresh", "f5"), context: fromSearchFields, command: commandRefresh},
	{id: sidebarRefreshRuleID, binding: binding("r", "Refresh", "r", "R", "shift+r"), command: commandRefresh, when: func(m *Model) bool {
		return m.sidebar && m.section != search
	}},
	{context: workspaceOnly, command: commandDiffLayout},
	{binding: binding("Ctrl+S", "Save", "ctrl+s"), context: fromSearchFields, command: commandSave},
	{binding: binding("Ctrl+W", "Close Editor", "ctrl+w"), context: fromSidebarSearch, command: commandClose, when: func(m *Model) bool {
		return !m.sidebar || m.section != search || m.searchField(m.search.focus) == nil
	}},
	{binding: binding("Ctrl+Shift+W", "Close Other Editors", "ctrl+shift+w", "ctrl+shift+W"), context: beforeInput, command: commandCloseOthers},
	{command: commandCloseAll},
	{binding: binding("Ctrl+F", "Find", "ctrl+f"), context: fromSearchFields, command: commandFind},
	{binding: binding("F7", "Context menu", "f7"), context: fromSearchFields, command: commandContextMenu},
}

var inputKeys = textinput.KeyMap{
	CharacterBackward:       key.NewBinding(key.WithKeys(leftArrow)),
	CharacterForward:        key.NewBinding(key.WithKeys(rightArrow)),
	WordBackward:            key.NewBinding(key.WithKeys("ctrl+left")),
	WordForward:             key.NewBinding(key.WithKeys("ctrl+right")),
	LineStart:               key.NewBinding(key.WithKeys("home")),
	LineEnd:                 key.NewBinding(key.WithKeys("end")),
	DeleteCharacterBackward: key.NewBinding(key.WithKeys("backspace")),
	DeleteCharacterForward:  key.NewBinding(key.WithKeys("delete")),
	DeleteWordBackward:      key.NewBinding(key.WithKeys("ctrl+backspace")),
	DeleteWordForward:       key.NewBinding(key.WithKeys("ctrl+delete")),
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
	Open, Edit, Delete, SplitFolders, Paste, FocusOutline, FocusFileHistory, OpenPullRequest key.Binding
}{
	Open:             binding("Enter", "Open selected item", "enter"),
	Edit:             binding("e", "Edit selected file", "e"),
	Delete:           binding("Del", "Delete selected item", "delete"),
	SplitFolders:     binding("L", "Split Folders", "L", "shift+l"),
	Paste:            binding("p", "Paste", "p"),
	FocusFileHistory: binding("t", "Focus Timeline", "t"),
	FocusOutline:     binding("o", "Focus Outline", "o"),
	OpenPullRequest:  binding("o", "Open on GitHub", "o"),
}

var pickerKeys = struct{ Up, Down, First, Last, Accept, ToggleIgnore key.Binding }{
	Up:           binding("↑ / Ctrl+K", "Previous", upArrow, "ctrl+k"),
	Down:         binding("↓ / Ctrl+J", "Next", downArrow, "ctrl+j"),
	First:        binding("Home", "First", "home"),
	Last:         binding("End", "Last", "end"),
	Accept:       binding("Enter", "Select", "enter"),
	ToggleIgnore: binding("F4", "Toggle ignore rules", "f4"),
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
	Up:    binding("↑", "Up", upArrow, "k"),
	Down:  binding("↓", "Down", downArrow, "j"),
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
	ZoomIn, ZoomOut, Fit key.Binding
}{
	ZoomIn:  binding("+", "Zoom in", "+", "="),
	ZoomOut: binding("-", "Zoom out", "-"),
	Fit:     binding("0", "Fit to Window", "0"),
}
