package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	searchservice "github.com/foursmith/vl/internal/search"
	"github.com/foursmith/vl/internal/theme"
)

type pickerLayout struct {
	x, y, width, start, count int
}

const recentRemoveWidth = 3 // " × "

func (m *Model) pickerHeight() int { return min(16, max(1, m.height-7)) }

func (m *Model) hasPickerIgnoreToggle() bool {
	return !m.editorOnly && m.modal == "quick" && !m.commandPicker()
}

func (l pickerLayout) indexAt(x, y int) int {
	i := y - l.y - 3
	if x < l.x || x >= l.x+l.width || i < 0 || i >= l.count {
		return -1
	}
	return l.start + i
}

func (m *Model) hoveredPickerIndex() int {
	if !m.isPicker() {
		return -1
	}
	return m.pickerLayout(m.filteredActions()).indexAt(m.hoverX, m.hoverY)
}

func (m *Model) pickerLayout(rows []action) pickerLayout {
	w := min(96, m.width-4)
	return pickerLayout{(m.width - w) / 2, 1, w, m.pickerTop, min(m.pickerHeight(), max(0, len(rows)-m.pickerTop))}
}

func (m *Model) pickerIgnoreControl(l pickerLayout) searchPanelControl {
	label := m.symbol(symbolIgnoreRules)
	width := max(2, ansi.StringWidth(label))
	return searchPanelControl{id: "ignore", label: label, x: l.x + l.width - 2 - width, y: l.y + 1, width: width, embedded: true}
}

func (m *Model) pickerIgnoreAt(l pickerLayout, x, y int) bool {
	if !m.hasPickerIgnoreToggle() {
		return false
	}
	c := m.pickerIgnoreControl(l)
	return y == c.y && x >= c.x && x < c.x+c.width
}

func (m *Model) pickerOverlay(base []string) []string {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	bg := m.colors.Panel
	w := l.width - 2
	border := theme.Style(m.colors.PopupBorder, bg)
	shown := m.input
	inputWidth, controls := w, ""
	if m.hasPickerIgnoreToggle() {
		c := m.pickerIgnoreControl(l)
		inputWidth = c.x - l.x - 1
		shown.SetWidth(max(1, inputWidth-3))
		controls = m.searchControlView(c, bg, !m.fileListIncludeIgnored, m.pickerIgnoreAt(l, m.hoverX, m.hoverY)) +
			row("", w-inputWidth-c.width, m.colors.InputForeground, bg)
	}
	box := []string{
		inputBorderRule(border, l.width, "╭", "╮"),
		border.Render("│") + row(" "+m.inputView(shown, l.x+2, l.y+1), inputWidth, m.colors.InputForeground, bg) + controls + border.Render("│"),
		inputBorderRule(border, l.width, "├", "┤"),
	}
	hovered := l.indexAt(m.hoverX, m.hoverY)
	for index := l.start; index < l.start+l.count; index++ {
		box = append(box, border.Render("│")+m.pickerRow(rows[index], index, l.width-2, hovered)+border.Render("│"))
	}

	if len(rows) == 0 {
		message := m.tr("No matches")
		if m.modal == "quick" && !m.commandPicker() && m.fileListLoading && len(m.snapshot.Files) == 0 {
			message = m.tr("Loading…")
		} else if m.modal == "recent" && m.input.Value() == "" {
			message = m.tr("No recent workspaces")
		}
		box = append(box, border.Render("│")+row("  "+message, w, theme.Readable(m.colors.Muted, bg), bg)+border.Render("│"))
	}
	box = append(box, inputBorderRule(border, l.width, "╰", "╯"))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) clickPicker(msg tea.MouseClickMsg) tea.Cmd {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	if msg.Button == tea.MouseRight && m.modal == "recent" {
		if index := l.indexAt(msg.X, msg.Y); index >= 0 {
			m.selectPickerIndex(index)
			folder := rows[index].id
			m.contextMenu = &contextMenu{x: msg.X, y: msg.Y, modal: m.modal, actions: []action{
				{label: m.tr("Remove from workspace list"), run: func() tea.Cmd { return m.removeWorkspace(folder) }},
			}}
		}
		return nil
	}
	if msg.Button == tea.MouseLeft && msg.X >= l.x && msg.X < l.x+l.width {
		if m.pickerIgnoreAt(l, msg.X, msg.Y) {
			return m.togglePickerIgnore()
		}
		if msg.Y >= l.y && msg.Y < l.y+3 {
			return nil
		}
		if index := l.indexAt(msg.X, msg.Y); index >= 0 {
			if m.pickerHeading(rows[index]) {
				return nil
			}
			right := l.x + l.width - 1
			if m.modal == "recent" && msg.X >= right-recentRemoveWidth && msg.X < right &&
				index == l.indexAt(m.hoverX, m.hoverY) {
				return m.removeWorkspace(rows[index].id)
			}
			m.selectPickerIndex(index)
			return m.selectPickerChoice()
		}
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

func (m *Model) isPicker() bool {
	switch m.modal {
	case "searchOptions", "stashPicker", "stashLoading", "quick", "remotes", "githubPublish", "refs", "recent", "themes", "languages", "statusBarStyles", "fileSyntax", "pullRequestFilter":
		return true
	default:
		return false
	}
}

func (m *Model) selectPickerChoice() tea.Cmd {
	m.syncPicker()
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[m.pickerIndex]
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	if a.run == nil {
		if m.modal == "quick" && !m.commandPicker() && a.id != "" {
			m.dismissDialog()
			return m.openFileInExplorer(a.id)
		}
		return nil
	}
	if m.commandPicker() {
		m.lastCommand = a.id
	}
	if m.modal == "themes" {
		if a.id == themeImportID {
			return a.run()
		}
		m.themeBeforePreview = nil
	}
	m.dismissDialog()
	return a.run()
}

func (m *Model) openQuickFiles() tea.Cmd { return m.openQuickPicker("") }

func (m *Model) refreshWorkspaceFiles() tea.Cmd {
	if m.fileListCancel != nil {
		m.fileListCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.fileListCancel = cancel
	m.fileListSeq++
	m.fileListLoading = true
	seq, root, includeIgnored := m.fileListSeq, m.repo.Root, m.fileListIncludeIgnored
	return func() tea.Msg {
		defer cancel()
		paths, err := searchservice.ListFilePaths(ctx, root, includeIgnored)
		return workspaceFilesMsg{seq, paths, err}
	}
}

func (m *Model) togglePickerIgnore() tea.Cmd {
	m.fileListIncludeIgnored = !m.fileListIncludeIgnored
	m.snapshot.Files = nil
	m.palette = nil
	m.syncQuickPicker()
	m.syncPicker()
	return m.refreshWorkspaceFiles()
}

func (m *Model) clearQuickFilePicker() {
	if m.quickFilesFilter.base == nil {
		return
	}
	if len(m.palette) > 0 && m.quickFilesFilter.base == &m.palette[0] {
		m.palette = nil
	}
	m.pickerFilter = pickerFilterCache{}
	m.quickFilesFilter = quickFilesFilter{}
}

func (m *Model) commandPicker() bool {
	return m.modal == "quick" && strings.HasPrefix(m.input.Value(), ">")
}

func (m *Model) openQuickPicker(value string) tea.Cmd {
	restore := m.cancelThemePreview()
	m.palette = nil
	cmd := m.prompt("quick", "", value)
	m.syncQuickPicker()
	m.syncPicker()
	return tea.Batch(restore, cmd)
}

func (m *Model) syncQuickPicker() {
	if m.modal != "quick" {
		return
	}
	title := m.tr("Go to File")
	if m.commandPicker() {
		title = m.tr("Commands")
	}
	if m.modalTitle == title && m.palette != nil {
		return
	}
	m.modalTitle = title
	m.pickerIndex = 0
	m.quickFilesFilter = quickFilesFilter{}
	if m.commandPicker() {
		m.input.Placeholder = ""
		m.palette = m.commandActions()
		return
	}
	m.input.Placeholder = m.tr("Go to File (name or path, add :line or :line:column)")
	m.palette = make([]action, 0, len(m.snapshot.Files))
	for _, path := range m.snapshot.Files {
		m.palette = append(m.palette, action{id: path, label: path})
	}
	if len(m.palette) > 0 {
		m.quickFilesFilter.base = &m.palette[0]
	}
}
