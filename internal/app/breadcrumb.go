package app

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

func (m *Model) breadcrumb(path string, width int) string {
	if width <= 0 {
		return ""
	}
	language := m.language(path)
	if filepath.IsAbs(path) {
		if relative, err := filepath.Rel(m.repo.Root, path); err == nil {
			path = relative
		}
	}
	path = filepath.ToSlash(path)
	icon := fileicon.For(path, m.nerd)
	separator := theme.Style(theme.Muted, theme.Background).Render(" " + m.symbol(symbolRight) + " ")
	iconText := row(icon.Glyph, 2, icon.Color, theme.Background)
	name := safe(filepath.Base(path))
	languageText := theme.Style(theme.Muted, theme.Background).Render(m.symbol(symbolWrench) + " " + language.ID)
	file := iconText + name
	tail := file + separator + languageText
	dir := filepath.ToSlash(filepath.Dir(path))
	prefix := ""
	if dir != "." {
		prefix = strings.Join(strings.Split(safe(dir), "/"), separator)
	}
	content := tail
	if prefix != "" {
		content = prefix + separator + tail
	}
	available := max(0, width-2)
	if ansi.StringWidth(content) > available {
		if ansi.StringWidth(tail) <= available {
			room := available - ansi.StringWidth(tail) - 3
			if room >= 1 && prefix != "" {
				content = ansi.Truncate(prefix, room, "…") + separator + tail
			} else {
				content = tail
			}
		} else {
			room := available - ansi.StringWidth(iconText) - 3 - ansi.StringWidth(languageText)
			if room >= 6 {
				content = iconText + ansi.Truncate(name, room, "…") + separator + languageText
			} else {
				content = iconText + ansi.Truncate(name, max(0, available-ansi.StringWidth(iconText)), "…")
			}
		}
	}
	return row(" "+content, width, theme.Muted, theme.Background)
}

// Breadcrumb and footer share the current file's language lookup, including
// unknown extensions whose Chroma filename matching is comparatively expensive.
func (m *Model) language(path string) highlight.LanguageInfo {
	if m.languagePath != path || m.languageInfo.ID == "" {
		m.languagePath = path
		m.languageInfo = highlight.Language(path)
	}
	return m.languageInfo
}
