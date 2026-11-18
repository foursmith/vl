package theme

import "charm.land/lipgloss/v2"

// Palette maps supported VS Code workbench color IDs to terminal surfaces.
// See docs/THEME.md for pinned sources and opaque diff overlay composition.
type Palette struct {
	TabActiveBorderTop                                                                 string
	StickyBorder                                                                       string
	BreadcrumbForeground, BreadcrumbBackground, StickyBackground                       string
	SideBarBorder                                                                      string
	LineNumberActive                                                                   string
	NavigationForeground, NavigationInactiveForeground                                 string
	TabBarBackground, TabActiveForeground, TabBorder                                   string
	MenuBackground, MenuForeground, MenuSelection, MenuSelectionForeground, MenuBorder string
	OutputBackground, OutputForeground                                                 string
	StatusBackground, StatusForeground                                                 string

	FindMatch                   string
	FindMatchHighlight          string
	FoldBackground              string
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

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(Composite(fg, bg))).Background(lipgloss.Color(bg))
}

func (f FontStyle) Apply(style lipgloss.Style) lipgloss.Style {
	return style.Bold(f&Bold != 0).Italic(f&Italic != 0).
		Underline(f&Underline != 0).Strikethrough(f&Strikethrough != 0)
}
