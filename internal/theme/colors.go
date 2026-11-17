package theme

import (
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
	p.SideBarBorder = t.Color("sideBar.border")
	p.FocusBorder = get("focusBorder", p.Panel)
	p.Accent = t.Color("textLink.foreground")
	p.NavigationForeground = t.Color("activityBar.foreground")
	p.NavigationInactiveForeground = t.Color("activityBar.inactiveForeground")
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
	p.DeletedBackground = get("diffEditor.removedLineBackground", p.Background)
	p.DeletedText = get("diffEditor.removedTextBackground", p.DeletedBackground)
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
	p.Popup = get("quickInput.background", p.Background)
	p.PopupForeground = t.colorOr("quickInput.foreground", t.Color("foreground"))
	p.PopupSelection = get("quickInputList.focusBackground", p.Popup)
	p.PopupSelectionForeground = t.colorOr("quickInputList.focusForeground", p.PopupForeground)
	// A terminal cannot paint VS Code's widget shadow. This edge is an explicit
	// terminal adaptation, separate from the raw widget.border registration.
	p.PopupBorder = get("widget.border", p.Popup)
	p.PopupBorder = distinguish(p.PopupBorder, p.Popup, Composite(p.PopupForeground, p.Popup), 1.6)
	p.Input = get("input.background", p.Popup)
	p.InputForeground = t.Color("input.foreground")
	p.InputPlaceholder = t.Color("input.placeholderForeground")
	p.TabBarBackground = get("editorGroupHeader.tabsBackground", p.Background)
	p.TabActiveBackground = get("tab.activeBackground", p.TabBarBackground)
	p.TabInactiveBackground = get("tab.inactiveBackground", p.TabBarBackground)
	p.TabActiveForeground = t.Color("tab.activeForeground")
	p.TabInactiveForeground = t.Color("tab.inactiveForeground")
	p.TabBorder = t.Color("tab.border")
	p.TabActiveBorderTop = t.colorOr("tab.activeBorderTop", p.FocusBorder)
	p.BreadcrumbForeground = t.colorOr("breadcrumb.foreground", p.Muted)
	p.BreadcrumbBackground = get("breadcrumb.background", p.Background)
	p.StickyBackground = get("editorStickyScroll.background", p.Background)
	p.StickyBorder = t.colorOr("editorStickyScroll.border", p.Border)
	p.FindMatch = get("editor.findMatchBackground", p.PopupSelection)
	p.FindMatchHighlight = get("editor.findMatchHighlightBackground", p.PopupSelection)
	p.CurrentLine = get("editor.lineHighlightBackground", p.Background)
	if p.CurrentLine == p.Background {
		// Terminal cells cannot draw an outline without consuming source space.
		p.CurrentLine = get("editor.lineHighlightBorder", p.Background)
	}
	p.FoldBackground = t.colorOr("editor.foldBackground", WithOpacity(p.Accent, .12))
	p.Strong = t.Color("strongForeground")
	p.Brand = t.Color("disabledForeground")
	p.MenuBackground = get("menu.background", p.Background)
	p.MenuForeground = t.colorOr("menu.foreground", t.Color("foreground"))
	p.MenuSelection = get("menu.selectionBackground", p.MenuBackground)
	p.MenuSelectionForeground = t.colorOr("menu.selectionForeground", p.MenuForeground)
	p.MenuBorder = get("menu.border", p.MenuBackground)
	p.MenuBorder = distinguish(p.MenuBorder, p.MenuBackground, Composite(p.MenuForeground, p.MenuBackground), 1.6)
	p.OutputBackground = get("panel.background", p.Background)
	p.OutputForeground = t.Color("foreground")
	p.StatusBackground = get("statusBar.background", p.Panel)
	p.StatusForeground = t.Color("statusBar.foreground")
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
