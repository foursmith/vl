package theme

import "charm.land/lipgloss/v2"

// Palette maps supported VS Code workbench color IDs to terminal surfaces.
// See docs/THEME.md for pinned sources and opaque diff overlay composition.
type Palette struct {
	SideBarBorder                                                                      string
	LineNumberActive                                                                   string
	NavigationForeground, NavigationInactiveForeground                                 string
	TabBarBackground, TabActiveForeground, TabBorder                                   string
	MenuBackground, MenuForeground, MenuSelection, MenuSelectionForeground, MenuBorder string
	OutputBackground, OutputForeground                                                 string
	StatusBackground, StatusForeground                                                 string

	FindMatch                   string
	FindMatchHighlight          string
	EditorSelection             string
	EditorSelectionForeground   string
	InputForeground             string
	InputPlaceholder            string
	SideForeground              string
	TabActiveBackground         string
	TabInactiveBackground       string
	TabInactiveForeground       string
	Background                  string
	Panel                       string
	Foreground                  string
	Muted                       string
	LineNumber                  string
	Border                      string
	Accent                      string
	FocusBorder                 string
	Selection                   string
	Deleted                     string
	AddedBackground             string
	AddedText                   string
	DeletedBackground           string
	DeletedText                 string
	DiffPlaceholder             string
	GitAdded                    string
	GitModified                 string
	GitUntracked                string
	GitDeleted                  string
	GitRenamed                  string
	GitConflicting              string
	GitStageModified            string
	GitStageDeleted             string
	GitSubmodule                string
	InactiveSelection           string
	InactiveSelectionForeground string
	HoverForeground             string
	Ignored                     string
	Hover                       string
	Popup                       string
	PopupForeground             string
	PopupBorder                 string
	PopupSelection              string
	PopupSelectionForeground    string
	Input                       string
	CurrentLine                 string
	Strong                      string
	SelectionForeground         string
	Brand                       string
}

func Dark() Palette  { return Builtin("dark_modern").Palette() }
func Light() Palette { return Builtin("light_modern").Palette() }

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(Composite(fg, bg))).Background(lipgloss.Color(bg))
}
