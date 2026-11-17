package app

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/fileicon"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
)

func (m *Model) breadcrumb(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if filepath.IsAbs(path) {
		if relative, err := filepath.Rel(m.repo.Root, path); err == nil {
			path = relative
		}
	}
	path = filepath.ToSlash(path)
	icon := fileicon.For(path, m.nerd)
	separator := theme.Style(m.colors.BreadcrumbForeground, m.colors.BreadcrumbBackground).Render(" " + m.symbol(symbolRight) + " ")
	iconText := row(icon.Glyph, 2, icon.Color, m.colors.BreadcrumbBackground)
	name := safe(filepath.Base(path))
	tail := iconText + name
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
			content = iconText + ansi.Truncate(name, max(0, available-ansi.StringWidth(iconText)), "…")
		}
	}
	return row(" "+content, width, m.colors.BreadcrumbForeground, m.colors.BreadcrumbBackground)
}

// Cache the current file's language for the footer.
func (m *Model) language(path string) highlight.LanguageInfo {
	if m.languagePath != path || m.languageInfo.ID == "" {
		m.languagePath = path
		m.languageInfo = highlight.Language(path)
	}
	return m.languageInfo
}
