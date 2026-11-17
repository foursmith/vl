package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
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

func (m *Model) contentToolbar() string {
	w := m.mainWidth()
	if im := m.currentImage(); im != nil {
		label := m.tr(" %.0f%% · 0 Fit ", m.imageRegion().Scale*100)
		if w >= 52 {
			label = m.tr(" %.0f%% · +/- Zoom · Arrows Pan · 0 Fit ", m.imageRegion().Scale*100)
		}
		width := min(w, ansi.StringWidth(label))
		return m.breadcrumb(im.Path, w-width) + row(label, width, m.colors.Muted, m.colors.Background)
	}
	if m.editing() {
		return m.breadcrumb(m.document().Path, w)
	}
	if !m.diffOpen {
		return row("", w, m.colors.Foreground, m.colors.Background)
	}
	if m.images != nil {
		label := " " + m.diffContext() + m.tr(" · Image comparison ")
		width := min(w, ansi.StringWidth(label))
		return m.breadcrumb(m.diffPath, w-width) + row(label, width, m.colors.Muted, m.colors.Background)
	}
	button := m.diffModeButton()
	stats := ""
	if m.patch.Added > 0 {
		stats += theme.Style(m.colors.GitAdded, m.colors.Background).Render(fmt.Sprintf("+%d", m.patch.Added))
	}
	if m.patch.Deleted > 0 {
		if stats != "" {
			stats += " "
		}
		stats += theme.Style(m.colors.Deleted, m.colors.Background).Render(fmt.Sprintf("−%d", m.patch.Deleted))
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
	left := row("", leftWidth, m.colors.Muted, m.colors.Background)
	if m.diffPath != "" {
		left = m.breadcrumb(m.diffPath, leftWidth)
	}
	text := left + row(stats, min(statsWidth, button.x), m.colors.Muted, m.colors.Background)
	buttonColor := m.colors.Foreground
	if w < 68 {
		buttonColor = m.colors.Muted
	}
	text += row(" "+button.label+" ", button.width, buttonColor, m.colors.Panel)
	return row(text, w, m.colors.Foreground, m.colors.Background)
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
