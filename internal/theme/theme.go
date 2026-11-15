package theme

import "charm.land/lipgloss/v2"

// Workbench colors follow microsoft/vscode's Dark Modern theme.
const (
	Background        = "#1F1F1F"
	Panel             = "#181818"
	Foreground        = "#CCCCCC"
	Muted             = "#9D9D9D"
	LineNumber        = "#6E7681"
	Border            = "#2B2B2B"
	Accent            = "#0078D4"
	Selection         = "#04395E"
	Deleted           = "#F85149"
	AddedBackground   = "#203426"
	DeletedBackground = "#3C2426"
	GitAdded          = "#73C991"
	GitModified       = "#E2C08D"
	Ignored           = "#8C8C8C"
	Hover             = "#2A2D2E"
)

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg))
}
