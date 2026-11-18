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

	FindMatch                    string
	FindMatchForeground          string
	FindMatchBorder              string
	FindMatchHighlight           string
	FindMatchHighlightForeground string
	FindMatchHighlightBorder     string

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
	FocusOutline                string
	Selection                   string
	Deleted                     string
	AddedBackground             string
	AddedText                   string
	AddedBorder                 string
	DeletedBackground           string
	DeletedText                 string
	DeletedBorder               string
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
	CurrentLineBorder           string
	IndentGuide                 string
	Strong                      string
	SelectionForeground         string
	Brand                       string
	bracketColors               [6]string
	bracketGuides               [6]string
	bracketColorCount           int
}

func (p Palette) BracketColor(depth int) string {
	if p.bracketColorCount == 0 {
		return ""
	}
	return p.bracketColors[depth%p.bracketColorCount]
}

func (p Palette) BracketGuide(depth int) string {
	if p.bracketColorCount == 0 {
		return ""
	}
	return p.bracketGuides[depth%p.bracketColorCount]
}

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(Composite(fg, bg))).Background(lipgloss.Color(bg))
}

// TextStyle applies the terminal's text contrast floor after compositing.
func TextStyle(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(Readable(fg, bg))).Background(lipgloss.Color(bg))
}

func CodeStyle(fg, bg, underline string, font FontStyle) lipgloss.Style {
	style := font.Apply(TextStyle(fg, bg))
	if underline != "" {
		style = style.Underline(true).UnderlineColor(lipgloss.Color(Readable(underline, bg)))
	}
	return style
}

func (p Palette) FindColors(fg, bg string, current bool) (string, string, string) {
	fill, text, border := p.FindMatchHighlight, p.FindMatchHighlightForeground, p.FindMatchHighlightBorder
	if current {
		fill, text, border = p.FindMatch, p.FindMatchForeground, p.FindMatchBorder
	}
	if text != "" {
		fg = text
	}
	if fill != "" {
		bg = Composite(fill, bg)
	}
	return fg, bg, border
}

func (f FontStyle) Apply(style lipgloss.Style) lipgloss.Style {
	return style.Bold(f&Bold != 0).Italic(f&Italic != 0).
		Underline(f&Underline != 0).Strikethrough(f&Strikethrough != 0)
}
