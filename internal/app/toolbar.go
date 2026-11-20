package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type toolbarButton struct {
	x, width int
	label    string
}

func (m *Model) diffModeButton() toolbarButton {
	if m.images != nil {
		return toolbarButton{}
	}
	label := m.symbol(symbolDiffUnified)
	if m.splitDiff() {
		label = m.symbol(symbolDiffSplit)
	}
	w := min(ansi.StringWidth(label)+2, m.mainWidth())
	return toolbarButton{x: m.mainWidth() - w, width: w, label: label}
}

func (m *Model) diffContext() string {
	context := m.tr("Working Tree")
	if m.diffStaged {
		context = m.tr("Staged")
	}
	if m.diffCommit != "" {
		context = m.tr("Commit · ") + shortOID(m.diffCommit) + m.tr(" · Read-only")
	}
	if m.diffPR.Number != 0 {
		context = fmt.Sprintf("PR #%d", m.diffPR.Number) + m.tr(" · Read-only")
	}
	fileStatus := ""
	oldMode, newMode := "", ""
	for _, parsed := range m.patch.Lines {
		line := parsed.Text
		switch {
		case fileStatus == "" && strings.HasPrefix(line, "new file mode "):
			fileStatus = m.tr(" · New file")
			if strings.HasSuffix(line, "755") {
				fileStatus += m.tr(" · Executable")
			}
		case fileStatus == "" && strings.HasPrefix(line, "deleted file mode "):
			fileStatus = m.tr(" · Deleted")
		case strings.HasPrefix(line, "old mode "):
			oldMode = strings.TrimPrefix(line, "old mode ")
		case strings.HasPrefix(line, "new mode "):
			newMode = strings.TrimPrefix(line, "new mode ")
		}
	}
	context += fileStatus
	if c := m.diffChange; c != nil && c.Original != "" {
		context += m.tr(" · Renamed from ") + safe(c.Original)
	}
	if oldMode != "" && newMode != "" {
		context += m.tr(" · Mode ") + oldMode + " → " + newMode
	}
	return context
}

type contentToolbarLayout struct {
	path   string
	width  int
	suffix string
}

func (m *Model) contentToolbar() string {
	l := m.layoutContentToolbar()
	if l.path == "" {
		return row("", l.width, m.colors.Foreground, m.colors.Background) + l.suffix
	}
	return m.breadcrumb(l.path, l.width) + l.suffix
}

func (m *Model) layoutContentToolbar() contentToolbarLayout {
	w := m.mainWidth()
	if im := m.currentImage(); im != nil {
		label := m.tr(" %.0f%% · 0 Fit ", m.imageRegion().Scale*100)
		if w >= 52 {
			label = m.tr(" %.0f%% · +/- Zoom · Arrows Pan · 0 Fit ", m.imageRegion().Scale*100)
		}
		width := min(w, ansi.StringWidth(label))
		return contentToolbarLayout{im.Path, w - width, row(label, width, m.colors.Muted, m.colors.Background)}
	}
	if m.editing() {
		return contentToolbarLayout{path: m.document().Path, width: w}
	}
	if !m.diffOpen {
		return contentToolbarLayout{width: w}
	}
	if m.images != nil {
		label := " " + m.diffContext() + m.tr(" · Image comparison ")
		width := min(w, ansi.StringWidth(label))
		return contentToolbarLayout{m.diffPath, w - width, row(label, width, m.colors.Muted, m.colors.Background)}
	}
	button := m.diffModeButton()
	stats := ""
	if m.patch.Added > 0 {
		stats += theme.TextStyle(m.colors.GitAdded, m.colors.Background).Render(fmt.Sprintf("+%d", m.patch.Added))
	}
	if m.patch.Deleted > 0 {
		if stats != "" {
			stats += " "
		}
		stats += theme.TextStyle(m.colors.Deleted, m.colors.Background).Render(fmt.Sprintf("−%d", m.patch.Deleted))
	}
	if stats != "" {
		stats += "  "
	}
	context := m.diffContext()
	context = ansi.Truncate(safe(context), max(0, button.x-12), "…")
	if context != "" {
		stats = theme.Style(m.colors.Muted, m.colors.Background).Render(context+"  ") + stats
	}
	statsWidth := ansi.StringWidth(stats)
	leftWidth := max(0, button.x-statsWidth)
	text := row(stats, min(statsWidth, button.x), m.colors.Muted, m.colors.Background)
	buttonColor := m.colors.Foreground
	if w < 68 {
		buttonColor = m.colors.Muted
	}
	text += row(" "+button.label+" ", button.width, buttonColor, m.colors.Background)
	return contentToolbarLayout{m.diffPath, leftWidth, text}
}

func (m *Model) clickContentToolbar(x int) tea.Cmd {
	if !m.editing() && m.diffOpen {
		button := m.diffModeButton()
		if x >= button.x && x < button.x+button.width {
			if m.mainWidth() < 68 {
				m.notice = m.tr("The editor needs at least 68 columns for side-by-side diff")
				return nil
			}
			m.split = !m.split
			m.renderDiff()
			return nil
		}
	}
	l := m.layoutContentToolbar()
	if l.path == "" || x < 0 || x >= l.width {
		return nil
	}
	dir, hit := m.layoutBreadcrumb(l.path, l.width).parentDirectoryAt(x)
	if !hit {
		return nil
	}
	if dir == "." {
		return m.openQuickFiles()
	}
	if strings.HasPrefix(dir, ">") {
		dir = "./" + dir
	}
	return m.openQuickPicker(dir + "/")
}

func hunkLabel(header string) string {
	fields := strings.Fields(header)
	if len(fields) < 4 {
		return safe(header)
	}
	rangeText := func(value string) string {
		parts := strings.Split(strings.TrimLeft(value, "+-"), ",")
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return value
		}
		count := 1
		if len(parts) > 1 {
			count, _ = strconv.Atoi(parts[1])
		}
		if count == 0 {
			return "∅"
		}
		if count == 1 {
			return strconv.Itoa(start)
		}
		return fmt.Sprintf("%d–%d", start, start+count-1)
	}
	label := rangeText(fields[1]) + " → " + rangeText(fields[2])
	if len(fields) > 4 {
		label += "  " + safe(strings.Join(fields[4:], " "))
	}
	return label
}
