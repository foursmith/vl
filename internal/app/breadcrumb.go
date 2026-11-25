package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/fileicon"
	"github.com/foursmith/vl/internal/theme"
)

type breadcrumbLayout struct {
	text, dir                   string
	prefixWidth, separatorWidth int
	fileStart, fileEnd          int
}

func (b breadcrumbLayout) parentDirectoryAt(x int) (string, bool) {
	if b.dir == "" {
		return "", false
	}
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
		if relative, err := filepath.Rel(m.repo.Root, path); err == nil && filepath.IsLocal(relative) {
			path = relative
		}
	}
	dir := filepath.ToSlash(filepath.Dir(path))
	displayDir := dir
	if filepath.IsAbs(path) {
		// Go to File cannot resolve directory queries outside its base.
		dir = ""
		if home, err := os.UserHomeDir(); err == nil {
			// Open documents use canonical paths, including symlinked homes.
			if real, err := filepath.EvalSymlinks(home); err == nil {
				home = real
			}
			if relative, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(relative) {
				displayDir = filepath.ToSlash(filepath.Join("~", filepath.Dir(relative)))
			}
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
	prefix := ""
	if displayDir != "." {
		if rest, absolute := strings.CutPrefix(displayDir, "/"); absolute {
			prefix = "/"
			if rest != "" {
				prefix += separator + strings.Join(strings.Split(safe(rest), "/"), separator)
			}
		} else {
			prefix = strings.Join(strings.Split(safe(displayDir), "/"), separator)
		}
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
