package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

type toolbarButton struct {
	x, width int
	label    string
}

func (m *Model) diffModeButton() toolbarButton {
	label := "统一"
	if m.split && m.mainWidth() >= 68 {
		label = "并排"
	}
	w := min(8, m.mainWidth())
	return toolbarButton{x: m.mainWidth() - w, width: w, label: label}
}

func (m *Model) diffContext() string {
	context := "工作区"
	if m.diffStaged {
		context = "暂存区"
	}
	if m.diffCommit != "" {
		context = "提交 · " + m.diffCommit[:min(7, len(m.diffCommit))] + " · 只读"
	}
	for _, line := range strings.Split(m.patch.Raw, "\n") {
		if strings.HasPrefix(line, "new file mode ") {
			context += " · 新文件"
			if strings.HasSuffix(line, "755") {
				context += " · 可执行"
			}
			break
		}
		if strings.HasPrefix(line, "deleted file mode ") {
			context += " · 已删除"
			break
		}
	}
	if c := m.diffChange; c != nil && c.Original != "" {
		context += " · 重命名自 " + safe(c.Original)
	}
	oldMode, newMode := "", ""
	for _, line := range strings.Split(m.patch.Raw, "\n") {
		if strings.HasPrefix(line, "old mode ") {
			oldMode = strings.TrimPrefix(line, "old mode ")
		}
		if strings.HasPrefix(line, "new mode ") {
			newMode = strings.TrimPrefix(line, "new mode ")
		}
	}
	if oldMode != "" && newMode != "" {
		context += " · 权限 " + oldMode + " → " + newMode
	}
	return context
}

func (m *Model) contentToolbar() string {
	w := m.mainWidth()
	if m.editing() {
		return m.breadcrumb(m.document().Path, w)
	}
	if !m.diffOpen {
		return row("", w, m.colors.Foreground, m.colors.Background)
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
