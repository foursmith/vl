package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/fileicon"
	searchservice "github.com/ourongxing/vl/internal/search"
	"github.com/ourongxing/vl/internal/theme"
)

// Keep files in a directory together, ahead of its descendants.
func (m *Model) sortSearchFiles() {
	sort.SliceStable(m.search.result.Files, func(i, j int) bool {
		a, b := m.search.result.Files[i].Path, m.search.result.Files[j].Path
		if ad, bd := filepath.Dir(a), filepath.Dir(b); ad != bd {
			return ad < bd
		}
		return strings.ToLower(filepath.Base(a)) < strings.ToLower(filepath.Base(b))
	})
}

func (m *Model) searchControlWidth() int {
	if !m.sideShown() {
		return m.width
	}
	return m.sideWidth()
}

func (m *Model) searchHoveredRow() int {
	if m.modal != "" || m.resizingSidebar || !m.sidebarAt(m.hoverX) || m.hoverY < m.searchResultY() || m.hoverY >= m.bodyHeight() {
		return -1
	}
	i := m.search.top + m.hoverY - m.searchResultY()
	if i >= len(m.searchRows()) {
		return -1
	}
	return i
}
func (m *Model) searchRowAction(index int) bool {
	return index == m.searchHoveredRow() || (m.search.focus == 2 && index == m.search.selected)
}
func (m *Model) searchResultRow(index, w int, r searchRow) string {
	s := &m.search
	f := s.result.Files[r.file]
	bg := m.colors.Panel
	hovered := m.searchHoveredRow()
	if index == hovered {
		bg = m.colors.Hover
	}
	if index == s.selected && s.focus == 2 {
		bg = m.colors.InactiveSelection
		if m.sidebar {
			bg = m.colors.Selection
		}
	}
	action := index == hovered || (s.focus == 2 && index == s.selected)
	end := max(0, w-2)
	var text string
	if r.match < 0 {
		rel, _ := filepath.Rel(m.repo.Root, f.Path)
		parent := filepath.ToSlash(filepath.Dir(rel))
		if parent == "." {
			parent = ""
		}
		icon := fileicon.For(f.Path, m.nerd)
		prefix := " " + m.treeChevron(!s.folded[f.Path]) + " " + row(icon.Glyph, 2, icon.Color, bg)
		badge := theme.Style(m.colors.Foreground, m.colors.DiffPlaceholder).Render(fmt.Sprintf(" %d ", len(f.Matches)))
		budget := max(0, end-ansi.StringWidth(prefix)-ansi.StringWidth(badge)-1)
		name := ansi.Truncate(safe(filepath.Base(f.Path)), budget, "…")
		label := prefix + theme.Style(m.colors.Foreground, bg).Bold(true).Render(name)
		remaining := budget - ansi.StringWidth(name)
		if parent != "" && remaining > 2 {
			label += theme.Style(m.colors.Muted, bg).Render(" " + ansi.Truncate(safe(parent), remaining-1, "…"))
		}
		text = row(label, max(0, end-ansi.StringWidth(badge)), m.colors.Foreground, bg) + badge
	} else {
		number := fmt.Sprintf("%d", f.Matches[r.match].Line)
		prefix := "   " + theme.Style(m.colors.Muted, bg).Render(number) + "  "
		content := m.searchSnippet(f, r.match, max(0, end-ansi.StringWidth(prefix)), bg)
		text = row(prefix+content, end, m.colors.Foreground, bg)
	}
	button := ""
	if action {
		button = m.symbol(symbolClose)
	}
	text += row("", 1, m.colors.Foreground, bg) + row(button, 1, m.colors.Foreground, bg)
	if m.monochrome && m.sidebar && index == s.selected && s.focus == 2 {
		text = ">" + ansi.Cut(text, 1, w)
	}
	return row(text, w, m.colors.Foreground, bg)
}

// Render one source line, keeping its first match visible in a narrow sidebar.
func (m *Model) searchSnippet(f searchservice.File, index, width int, bg string) string {
	if width <= 0 {
		return ""
	}
	hit := f.Matches[index]
	start := strings.LastIndexByte(f.Text[:hit.Start], '\n') + 1
	end := strings.IndexByte(f.Text[hit.Start:], '\n')
	if end < 0 {
		end = len(f.Text)
	} else {
		end += hit.Start
	}
	trim := start
	for trim < hit.Start && (f.Text[trim] == ' ' || f.Text[trim] == '\t') {
		trim++
	}
	left := hit.Start
	cells := 0
	for left > trim {
		r, size := utf8.DecodeLastRuneInString(f.Text[trim:left])
		n := ansi.StringWidth(safe(string(r)))
		if cells+n > width/3 {
			break
		}
		cells += n
		left -= size
	}
	prefix := ""
	if left > trim {
		prefix = "…"
	}
	right := left
	cells = ansi.StringWidth(prefix)
	for right < end && cells < width {
		r, size := utf8.DecodeRuneInString(f.Text[right:end])
		cells += ansi.StringWidth(safe(string(r)))
		right += size
	}
	var b strings.Builder
	b.WriteString(prefix)
	pos := left
	for j := index; j < len(f.Matches) && f.Matches[j].Line == hit.Line; j++ {
		match := f.Matches[j]
		a, z := max(left, match.Start), min(right, match.End)
		if match.Start >= right {
			break
		}
		if z < a {
			continue
		}
		b.WriteString(theme.Style(m.colors.Foreground, bg).Render(safe(f.Text[pos:a])))
		b.WriteString(theme.Style(m.colors.Foreground, m.colors.FindMatch).Render(safe(f.Text[a:z])))
		pos = z
	}
	b.WriteString(theme.Style(m.colors.Foreground, bg).Render(safe(f.Text[pos:right])))
	suffix := ""
	if right < end {
		suffix = "…"
	}
	return ansi.Truncate(b.String(), max(0, width-ansi.StringWidth(suffix)), "") + suffix
}

// Removing a result only excludes it from this search and subsequent replacements.
func (m *Model) dismissSearchResult(index int) {
	rows := m.searchRows()
	if index < 0 || index >= len(rows) {
		return
	}
	r := rows[index]
	s := &m.search
	f := &s.result.Files[r.file]
	if r.match < 0 {
		s.result.Count -= len(f.Matches)
		s.result.Files = slices.Delete(s.result.Files, r.file, r.file+1)
	} else {
		end := r.match + 1
		for end < len(f.Matches) && f.Matches[end].Line == f.Matches[r.match].Line {
			end++
		}
		s.result.Count -= end - r.match
		f.Matches = slices.Delete(f.Matches, r.match, end)
		if len(f.Matches) == 0 {
			s.result.Files = slices.Delete(s.result.Files, r.file, r.file+1)
		}
	}
	m.clampSearch()
}
