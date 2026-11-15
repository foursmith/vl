package theme

import "charm.land/lipgloss/v2"

// Palette contains VS Code Modern workbench colors owned by each model.
// See docs/THEME.md for pinned sources and opaque diff overlay composition.
type Palette struct {
	Background          string
	Panel               string
	Foreground          string
	Muted               string
	LineNumber          string
	Border              string
	Accent              string
	SearchMatch         string
	Selection           string
	Deleted             string
	AddedBackground     string
	AddedText           string
	DeletedBackground   string
	DeletedText         string
	DiffPlaceholder     string
	GitAdded            string
	GitModified         string
	Ignored             string
	Hover               string
	Popup               string
	Input               string
	CurrentLine         string
	Strong              string
	SelectionForeground string
	Brand               string
}

func Dark() Palette {
	return Palette{
		Background:          "#1F1F1F",
		Panel:               "#181818",
		Foreground:          "#CCCCCC",
		Muted:               "#9D9D9D",
		LineNumber:          "#6E7681",
		Border:              "#2B2B2B",
		Accent:              "#0078D4",
		Selection:           "#04395E",
		SearchMatch:         "#613315",
		Deleted:             "#F85149",
		AddedBackground:     "#383E2A",
		AddedText:           "#4C5A2A",
		DeletedBackground:   "#4C1919",
		DeletedText:         "#701414",
		DiffPlaceholder:     "#424242",
		GitAdded:            "#73C991",
		GitModified:         "#E2C08D",
		Ignored:             "#8C8C8C",
		Hover:               "#2A2D2E",
		Popup:               "#222222",
		Input:               "#313131",
		CurrentLine:         "#252526",
		Strong:              "#FFFFFF",
		SelectionForeground: "#FFFFFF",
		Brand:               "#50565E",
	}
}

func Light() Palette {
	return Palette{
		Background:          "#FFFFFF",
		Panel:               "#F8F8F8",
		Foreground:          "#3B3B3B",
		Muted:               "#616161",
		LineNumber:          "#6E7681",
		Border:              "#E5E5E5",
		Accent:              "#005FB8",
		Selection:           "#ADD6FF",
		SearchMatch:         "#F3C6A5",
		Deleted:             "#AD0707",
		AddedBackground:     "#EBF1DD",
		AddedText:           "#D7E8B1",
		DeletedBackground:   "#FFCCCC",
		DeletedText:         "#FFA3A3",
		DiffPlaceholder:     "#D3D3D3",
		GitAdded:            "#587C0C",
		GitModified:         "#895503",
		Ignored:             "#767676",
		Hover:               "#F2F2F2",
		Popup:               "#F8F8F8",
		Input:               "#FFFFFF",
		CurrentLine:         "#F3F3F3",
		Strong:              "#1F1F1F",
		SelectionForeground: "#000000",
		Brand:               "#A0A0A0",
	}
}

func Style(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg))
}
