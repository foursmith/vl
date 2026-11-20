package app

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/fileicon"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/theme"
)

type breadcrumbLayout struct {
	text, dir                   string
	prefixWidth, separatorWidth int
	fileStart, fileEnd          int
}

func (b breadcrumbLayout) parentDirectoryAt(x int) (string, bool) {
	if x >= b.fileStart && x < b.fileEnd {
		return b.dir, true
	}
	if x < 1 || x >= 1+b.prefixWidth {
		return "", false
	}
	start, end := 1, 0
	for part := range strings.SplitSeq(b.dir, "/") {
		width := ansi.StringWidth(safe(part))
		end += len(part)
		if x >= start && x < start+width {
			return filepath.ToSlash(filepath.Dir(b.dir[:end])), true
		}
		start += width + b.separatorWidth
		end++
	}
	return "", false
}

func (m *Model) breadcrumb(path string, width int) string {
	return m.layoutBreadcrumb(path, width).text
}

func (m *Model) layoutBreadcrumb(path string, width int) breadcrumbLayout {
	if width <= 0 {
		return breadcrumbLayout{}
	}
	if filepath.IsAbs(path) {
		if relative, err := filepath.Rel(m.repo.Root, path); err == nil {
			path = relative
		}
	}
	path = filepath.ToSlash(path)
	icon := fileicon.For(path, m.nerd)
	bg := m.colors.BreadcrumbBackground
	fg := theme.Readable(m.colors.BreadcrumbForeground, bg)
	separator := " " + m.symbol(symbolRight) + " "
	separatorWidth := ansi.StringWidth(separator)
	separator = theme.Style(fg, bg).Render(separator)
	iconText := row(icon.Glyph, 2, icon.Color, bg)
	name := safe(filepath.Base(path))
	tail := iconText + name
	dir := filepath.ToSlash(filepath.Dir(path))
	prefix := ""
	if dir != "." {
		prefix = strings.Join(strings.Split(safe(dir), "/"), separator)
	}
	content := tail
	prefixWidth := ansi.StringWidth(prefix)
	fileStart := 1
	if prefix != "" {
		content = prefix + separator + tail
		fileStart += prefixWidth + separatorWidth
	}
	available := max(0, width-2)
	if ansi.StringWidth(content) > available {
		fileStart, prefixWidth = 1, 0
		if ansi.StringWidth(tail) <= available {
			room := available - ansi.StringWidth(tail) - separatorWidth
			if room >= 1 && prefix != "" {
				short := ansi.Truncate(prefix, room, "…")
				content = short + separator + tail
				prefixWidth = ansi.StringWidth(short) - 1
				fileStart += prefixWidth + 1 + separatorWidth
			} else {
				content = tail
			}
		} else {
			content = iconText + ansi.Truncate(name, max(0, available-ansi.StringWidth(iconText)), "…")
		}
	}
	return breadcrumbLayout{
		text: row(" "+content, width, fg, bg), dir: dir,
		prefixWidth: prefixWidth, separatorWidth: separatorWidth,
		fileStart: fileStart, fileEnd: min(width, 1+ansi.StringWidth(content)),
	}
}

// Cache the current file's language for the footer.
func (m *Model) language(path string) highlight.LanguageInfo {
	if m.languagePath != path || m.languageInfo.ID == "" {
		m.languagePath = path
		m.languageInfo = highlight.Language(path)
	}
	return m.languageInfo
}
