package theme

import "charm.land/lipgloss/v2"

// Workbench colors follow microsoft/vscode's Dark Modern theme.
// See docs/THEME.md for pinned upstream sources and diff alpha composition.
const (
	Background = "#1F1F1F"
	Panel      = "#181818"
	Foreground = "#CCCCCC"
	Muted      = "#9D9D9D"
	LineNumber = "#6E7681"
	Border     = "#2B2B2B"
	Accent     = "#0078D4"
	Selection  = "#04395E"
	Deleted    = "#F85149"
	// VS Code's diff overlays composited over Background (terminals have no alpha):
	// inserted line rgba(155,185,85,.2), text #9CCC2C33; removed #FF000033.
	AddedBackground   = "#383E2A"
	AddedText         = "#4C5A2A"
	DeletedBackground = "#4C1919"
	DeletedText       = "#701414"
	DiffPlaceholder   = "#424242" // #CCCCCC33 over Background
	GitAdded          = "#73C991"
	GitModified       = "#E2C08D"
	Ignored           = "#8C8C8C"
	Hover             = "#2A2D2E"
)

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg))
}
