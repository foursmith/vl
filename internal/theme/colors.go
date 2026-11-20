package theme

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

func normalizeHex(s string) string {
	if len(s) == 0 || s[0] != '#' {
		return ""
	}
	switch len(s) {
	case 4, 5:
		var b strings.Builder
		b.WriteByte('#')
		for _, c := range s[1:] {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		s = b.String()
	case 7, 9:
	default:
		return ""
	}
	if _, err := hex.DecodeString(s[1:]); err != nil {
		return ""
	}
	s = strings.ToUpper(s)
	if len(s) == 9 && strings.HasSuffix(s, "FF") {
		s = s[:7]
	}
	return s
}
func opaque(color, bg string) string {
	color = normalizeHex(color)
	if color == "" {
		return bg
	}
	if len(color) == 7 {
		return color
	}
	rgba, _ := hex.DecodeString(color[1:])
	rgb, _ := hex.DecodeString(bg[1:7])
	a := float64(rgba[3]) / 255
	return fmt.Sprintf("#%02X%02X%02X", int(math.Round(float64(rgba[0])*a+float64(rgb[0])*(1-a))), int(math.Round(float64(rgba[1])*a+float64(rgb[1])*(1-a))), int(math.Round(float64(rgba[2])*a+float64(rgb[2])*(1-a))))
}

// Palette is a component adapter over VS Code's registered colors. Foregrounds
// retain RGBA; only backgrounds are composed here because their parent is known.
func (t *Theme) makePalette() Palette {
	get := func(id, bg string) string { return opaque(t.Color(id), bg) }
	p := Palette{}
	p.Background = get("editor.background", "#000000")
	p.Foreground = t.Color("editor.foreground")
	p.Panel = get("sideBar.background", p.Background)
	p.SideForeground = t.colorOr("sideBar.foreground", t.Color("foreground"))
	p.Muted = t.Color("descriptionForeground")
	p.LineNumber = t.Color("editorLineNumber.foreground")
	p.LineNumberActive = t.Color("editorLineNumber.activeForeground")
	p.Border = get("panel.border", p.Panel)
	p.FocusBorder = get("focusBorder", p.Panel)
	p.FocusOutline = t.Color("contrastActiveBorder")
	p.Accent = t.Color("textLink.foreground")
	p.NavigationForeground = t.Color("activityBarTop.foreground")
	p.NavigationInactiveForeground = t.Color("activityBarTop.inactiveForeground")
	if (t.Type == "hc-black" || t.Type == "hc-light") &&
		Readable(p.NavigationForeground, p.Panel) == Readable(p.NavigationInactiveForeground, p.Panel) {
		// Some themes only specify distinct icon colors for the side activity bar.
		p.NavigationForeground = cmp.Or(t.colors["activityBarTop.foreground"], t.colors["activityBar.foreground"], p.NavigationForeground)
		inactive := t.colors["activityBar.inactiveForeground"]
		active := Readable(p.NavigationForeground, p.Panel)
		if inactive == "" || Readable(inactive, p.Panel) == active {
			inactive = WithOpacity(active, .6)
			if Readable(inactive, p.Panel) == active {
				p.NavigationForeground = Readable("", p.Panel)
			}
		}
		p.NavigationInactiveForeground = inactive
	}
	p.Selection = get("list.activeSelectionBackground", p.Panel)
	p.SelectionForeground = t.Color("list.activeSelectionForeground")
	p.InactiveSelection = get("list.inactiveSelectionBackground", p.Panel)
	p.InactiveSelectionForeground = t.Color("list.inactiveSelectionForeground")
	p.Hover = get("list.hoverBackground", p.Panel)
	p.HoverForeground = t.Color("list.hoverForeground")
	p.EditorSelection = t.Color("editor.selectionBackground")
	p.EditorSelectionForeground = t.Color("editor.selectionForeground")
	p.Deleted = t.Color("errorForeground")
	p.AddedBackground = get("diffEditor.insertedLineBackground", p.Background)
	p.AddedText = get("diffEditor.insertedTextBackground", p.AddedBackground)
	p.AddedBorder = t.Color("diffEditor.insertedTextBorder")
	p.DeletedBackground = get("diffEditor.removedLineBackground", p.Background)
	p.DeletedText = get("diffEditor.removedTextBackground", p.DeletedBackground)
	p.DeletedBorder = t.Color("diffEditor.removedTextBorder")
	p.DiffPlaceholder = get("diffEditor.diagonalFill", p.Background)
	p.GitAdded = t.Color("gitDecoration.addedResourceForeground")
	p.GitModified = t.Color("gitDecoration.modifiedResourceForeground")
	p.GitUntracked = t.Color("gitDecoration.untrackedResourceForeground")
	p.GitDeleted = t.Color("gitDecoration.deletedResourceForeground")
	p.GitRenamed = t.Color("gitDecoration.renamedResourceForeground")
	p.GitConflicting = t.Color("gitDecoration.conflictingResourceForeground")
	p.GitStageModified = t.Color("gitDecoration.stageModifiedResourceForeground")
	p.GitStageDeleted = t.Color("gitDecoration.stageDeletedResourceForeground")
	p.GitSubmodule = t.Color("gitDecoration.submoduleResourceForeground")
	p.Ignored = t.Color("gitDecoration.ignoredResourceForeground")
	p.PopupForeground = t.colorOr("quickInput.foreground", t.Color("foreground"))
	p.PopupSelection = p.selectionTint(get("quickInputList.focusBackground", p.Panel), p.Panel, 1.6)
	p.PopupSelectionForeground = t.colorOr("quickInputList.focusForeground", p.PopupForeground)
	// A terminal cannot paint VS Code's widget shadow. This edge is an explicit
	// terminal adaptation, separate from the raw widget.border registration.
	p.PopupBorder = get("widget.border", p.Panel)
	p.PopupBorder = distinguish(p.PopupBorder, p.Panel, Composite(p.PopupForeground, p.Panel), 1.6)
	p.Input = get("input.background", get("quickInput.background", p.Background))
	p.InputForeground = t.Color("input.foreground")
	p.InputPlaceholder = t.Color("input.placeholderForeground")
	p.TabActiveBackground = get("tab.activeBackground", p.Panel)
	p.TabActiveForeground = t.Color("tab.activeForeground")
	p.TabInactiveForeground = t.Color("tab.inactiveForeground")
	p.TabBorder = t.Color("tab.border")
	p.TabActiveBorderTop = t.colorOr("tab.activeBorderTop", p.FocusBorder)
	p.BreadcrumbForeground = t.colorOr("breadcrumb.foreground", p.Muted)
	p.BreadcrumbBackground = get("breadcrumb.background", p.Background)
	p.StickyBackground = get("editorStickyScroll.background", p.Background)
	p.StickyBorder = t.colorOr("editorStickyScroll.border", p.Border)
	p.FindMatch = t.Color("editor.findMatchBackground")
	p.FindMatchForeground = t.Color("editor.findMatchForeground")
	p.FindMatchBorder = t.Color("editor.findMatchBorder")
	p.FindMatchHighlight = t.Color("editor.findMatchHighlightBackground")
	p.FindMatchHighlightForeground = t.Color("editor.findMatchHighlightForeground")
	p.FindMatchHighlightBorder = t.Color("editor.findMatchHighlightBorder")
	p.CurrentLine = get("editor.lineHighlightBackground", p.Background)
	if p.CurrentLine == p.Background {
		p.CurrentLineBorder = t.Color("editor.lineHighlightBorder")
	}
	p.IndentGuide = t.Color("editorIndentGuide.background1")
	bracketColors := [...]string{
		t.Color("editorBracketHighlight.foreground1"),
		t.Color("editorBracketHighlight.foreground2"),
		t.Color("editorBracketHighlight.foreground3"),
		t.Color("editorBracketHighlight.foreground4"),
		t.Color("editorBracketHighlight.foreground5"),
		t.Color("editorBracketHighlight.foreground6"),
	}
	for i, color := range bracketColors {
		if len(color) == 7 || len(color) == 9 && color[7:] != "00" {
			slot := p.bracketColorCount
			p.bracketColors[slot] = color
			p.bracketGuides[slot] = t.colorOr(fmt.Sprintf("editorBracketPairGuide.activeBackground%d", i+1), color)
			p.bracketColorCount++
		}
	}
	p.FoldBackground = t.Color("editor.foldBackground")
	p.Strong = t.Color("strongForeground")
	p.Brand = t.Color("disabledForeground")
	p.MenuForeground = t.colorOr("menu.foreground", t.Color("foreground"))
	p.MenuSelection = p.selectionTint(get("menu.selectionBackground", p.Panel), p.Panel, 1.6)
	p.MenuSelectionForeground = t.colorOr("menu.selectionForeground", p.MenuForeground)
	p.MenuBorder = get("menu.border", p.Panel)
	p.MenuBorder = distinguish(p.MenuBorder, p.Panel, Composite(p.MenuForeground, p.Panel), 1.6)
	return p
}

func luminance(color string) float64 {
	rgb, _ := hex.DecodeString(color[1:7])
	channel := func(c byte) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return .2126*channel(rgb[0]) + .7152*channel(rgb[1]) + .0722*channel(rgb[2])
}
func contrast(a, b string) float64 {
	x, y := luminance(a), luminance(b)
	if x < y {
		x, y = y, x
	}
	return (x + .05) / (y + .05)
}

// Readable retains the composited theme color if it passes the text contrast
// floor. Otherwise it moves toward the higher-contrast neutral color.
func Readable(foreground, background string) string {
	background = normalizeHex(background)
	if len(background) != 7 {
		return foreground
	}
	foreground = normalizeHex(foreground)
	if foreground != "" {
		foreground = opaque(foreground, background)
		if contrast(foreground, background) >= 4.5 {
			return foreground
		}
	}
	target := "#FFFFFF"
	if contrast("#000000", background) > contrast(target, background) {
		target = "#000000"
	}
	if foreground == "" {
		return target
	}
	return distinguish(foreground, background, target, 4.5)
}

// SidebarSelection converts a theme's focus outline into a selection tint
// when the terminal row needs a separate background.
func (p Palette) SidebarSelection(focused bool) string {
	fill, minimum := p.InactiveSelection, 1.25
	if focused {
		fill, minimum = p.Selection, 1.6
	}
	return p.selectionTint(fill, p.Panel, minimum)
}

func (p Palette) selectionTint(fill, surface string, minimum float64) string {
	if fill != surface && p.FocusOutline == "" {
		return fill
	}
	tint := p.FocusOutline
	if tint == "" {
		tint = p.Accent
	}
	return distinguish(fill, surface, Readable(tint, surface), minimum)
}

// VS Code can separate low-contrast widgets with shadows. Terminal cells have
// no shadow layer, so retain the theme hue while making their edge/focus visible.
func distinguish(color, surface, foreground string, minimum float64) string {
	if contrast(color, surface) >= minimum {
		return color
	}
	from, _ := hex.DecodeString(color[1:7])
	to, _ := hex.DecodeString(foreground[1:7])
	for step := 1; step <= 20; step++ {
		a := float64(step) / 20
		mixed := fmt.Sprintf("#%02X%02X%02X", int(math.Round(float64(from[0])*(1-a)+float64(to[0])*a)), int(math.Round(float64(from[1])*(1-a)+float64(to[1])*a)), int(math.Round(float64(from[2])*(1-a)+float64(to[2])*a)))
		if contrast(mixed, surface) >= minimum {
			return mixed
		}
	}
	return foreground
}

// VS Code renders propagated folder decoration bubbles with opacity 0.4.
func DecorationBubble(color, bg string) string { return Composite(WithOpacity(color, .4), bg) }
