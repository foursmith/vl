package app

import (
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

func (m *Model) keepEditedTabs() {
	for i := range m.tabs {
		if m.tabs[i].doc.Dirty() {
			m.tabs[i].preview = false
		}
	}
}

func (m *Model) openFile(path string) tea.Cmd {
	m.openDocument(path, false)
	return nil
}

func (m *Model) openPreviewFile(path string) tea.Cmd {
	m.openDocument(path, true)
	return nil
}

func (m *Model) openDocument(path string, preview bool) bool {
	defer m.syncWatch()
	m.keepEditedTabs()
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(m.repo.Root, path)
	}
	abs = filepath.Clean(abs)
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	for i := range m.tabs {
		if m.tabs[i].doc.Path == abs {
			if !preview {
				m.tabs[i].preview = false
			}
			m.activateTab(i)
			return true
		}
	}
	d, err := editor.Open(abs)
	if err != nil {
		m.log(err.Error())
		return false
	}
	next := tab{doc: d, preview: preview}
	if preview {
		for i := range m.tabs {
			if m.tabs[i].preview && !m.tabs[i].doc.Dirty() {
				m.tabs[i].highlightJob.stop()
				m.tabs[i] = next
				m.activateTab(i)
				return true
			}
		}
	}
	m.tabs = append(m.tabs, next)
	m.activateTab(len(m.tabs) - 1)
	return true
}

func (m *Model) activateTab(index int) {
	m.activeTab = index
	m.showEditor = true
	m.sidebar = false
	m.ensureEditor()
}

func (m *Model) keepTab() tea.Cmd {
	if m.showEditor && m.document() != nil {
		m.tabs[m.activeTab].preview = false
	}
	return nil
}

func (m *Model) closeTab() tea.Cmd {
	if !m.showEditor {
		m.closeDiff()
		return nil
	}
	if d := m.document(); d != nil {
		return m.requestClose([]*editor.Document{d})
	}
	return nil
}

func (m *Model) closeAllTabs() tea.Cmd {
	m.closeDiff()
	var docs []*editor.Document
	for _, t := range m.tabs {
		docs = append(docs, t.doc)
	}
	return m.requestClose(docs)
}

func (m *Model) closeOtherTabs() tea.Cmd {
	var keep *editor.Document
	if m.showEditor {
		keep = m.document()
		m.closeDiff()
	}
	var docs []*editor.Document
	for _, t := range m.tabs {
		if t.doc != keep {
			docs = append(docs, t.doc)
		}
	}
	return m.requestClose(docs)
}

func (m *Model) requestClose(docs []*editor.Document) tea.Cmd {
	m.closeQueue = docs
	return m.advanceClose()
}

func (m *Model) advanceClose() tea.Cmd {
	for len(m.closeQueue) > 0 {
		d := m.closeQueue[0]
		if d.Dirty() {
			m.modal = "close"
			m.modalTitle = "Save changes to " + filepath.Base(d.Path) + "?"
			m.input.Blur()
			return nil
		}
		m.removeTab(d)
		m.closeQueue = m.closeQueue[1:]
	}
	m.closeQueue = nil
	m.modal = ""
	return nil
}

func (m *Model) chooseClose(choice string) tea.Cmd {
	if choice == "cancel" {
		m.closeQueue = nil
		m.modal = ""
		return nil
	}
	if len(m.closeQueue) == 0 {
		m.modal = ""
		return nil
	}
	d := m.closeQueue[0]
	if choice == "save" {
		if m.busy {
			m.notice = "Wait for the current Git operation before saving"
			return nil
		}
		if err := d.Save(); err != nil {
			m.log(err.Error())
			return nil
		}
	}
	m.removeTab(d)
	m.closeQueue = m.closeQueue[1:]
	return m.advanceClose()
}

func (m *Model) removeTab(doc *editor.Document) {
	defer m.syncWatch()
	for i, t := range m.tabs {
		if t.doc != doc {
			continue
		}
		m.tabs[i].highlightJob.stop()
		m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
		if i < m.activeTab {
			m.activeTab--
		} else if i == m.activeTab {
			m.activeTab = min(i, len(m.tabs)-1)
		}
		if len(m.tabs) == 0 {
			m.activeTab = -1
			m.showEditor = false
			if !m.diffOpen {
				m.sidebar = true
			}
		}
		m.ensureEditor()
		m.ensureTabVisible()
		return
	}
}

func (m *Model) closeDiff() {
	m.diffOpen = false
	if m.diffCancel != nil {
		m.diffCancel()
		m.diffCancel = nil
	}
	m.diffSeq++
	m.diffLoading = false
	m.diffTitle = ""
	m.diffPath = ""
	m.diffChange = nil
	m.clearDiffContent()
	m.diffHunk = -1
	if !m.showEditor {
		if m.document() != nil {
			m.showEditor = true
		} else {
			m.sidebar = true
		}
	}
	m.ensureTabVisible()
}

type tabSlot struct {
	x, width, index int
	label           string
	control         int
}

func (m *Model) tabLabel(index int) string {
	if index < 0 {
		if m.diffPath != "" {
			suffix := ""
			if m.diffStaged {
				suffix = " · 暂存"
			}
			return safe(filepath.Base(m.diffPath)) + suffix
		}
		if m.diffTitle != "" {
			return safe(m.diffTitle)
		}
		return "Changes"
	}
	t := m.tabs[index]
	label := filepath.Base(t.doc.Path)
	for i, other := range m.tabs {
		if i != index && filepath.Base(other.doc.Path) == label {
			label += " · " + filepath.Base(filepath.Dir(t.doc.Path))
			break
		}
	}
	return safe(label)
}

func (m *Model) tabLayout() []tabSlot {
	var all []tabSlot
	if m.diffOpen {
		all = append(all, tabSlot{index: -1, label: m.tabLabel(-1)})
	}
	for i := range m.tabs {
		all = append(all, tabSlot{index: i, label: m.tabLabel(i)})
	}
	if len(all) == 0 {
		return nil
	}
	w := m.mainWidth()
	total := 0
	for i := range all {
		extra := 0
		if all[i].index >= 0 && m.tabs[all[i].index].doc.Dirty() {
			extra = 2
		}
		all[i].width = min(28, max(12, ansi.StringWidth(all[i].label)+extra+7))
		total += all[i].width
	}
	overflow := total > w
	available := w
	x, start := 0, 0
	var slots []tabSlot
	if overflow {
		available = max(1, w-6)
		x = 3
		start = min(m.tabStart, len(all)-1)
		slots = append(slots, tabSlot{x: 0, width: 3, control: -1})
	}
	for i := start; i < len(all); i++ {
		slot := all[i]
		slot.width = min(slot.width, available)
		if slot.width < 7 || x+slot.width > w-3 && overflow || x+slot.width > w {
			break
		}
		slot.x = x
		slots = append(slots, slot)
		x += slot.width
	}
	if overflow {
		slots = append(slots, tabSlot{x: w - 3, width: 3, control: 1})
	}
	return slots
}

func (m *Model) ensureTabVisible() {
	active := -1
	if m.showEditor {
		active = m.activeTab
	}
	count := len(m.tabs)
	if m.diffOpen {
		count++
	}
	m.tabStart = min(max(0, m.tabStart), max(0, count-1))
	for tries := 0; tries < count; tries++ {
		for _, slot := range m.tabLayout() {
			if slot.control == 0 && slot.index == active {
				return
			}
		}
		ordinal := active
		if m.diffOpen {
			ordinal++
		}
		if active < 0 {
			ordinal = 0
		}
		if ordinal < m.tabStart {
			m.tabStart = ordinal
		} else {
			m.tabStart++
		}
		m.tabStart = min(max(0, m.tabStart), max(0, count-1))
	}
}

func (m *Model) doubleClick(key string) bool {
	now := time.Now()
	double := m.lastClickKey == key && now.Sub(m.lastClickTime) < 400*time.Millisecond
	m.lastClickKey, m.lastClickTime = key, now
	if double {
		m.lastClickKey = ""
	}
	return double
}

func (m *Model) clickTab(msg tea.MouseClickMsg) tea.Cmd {
	for _, slot := range m.tabLayout() {
		x := msg.X - m.mainX()
		if x < slot.x || x >= slot.x+slot.width {
			continue
		}
		if slot.control != 0 {
			if msg.Button == tea.MouseLeft {
				count := len(m.tabs)
				if m.diffOpen {
					count++
				}
				m.tabStart = min(max(0, count-1), max(0, m.tabStart+slot.control))
			}
			return nil
		}
		close := msg.Button == tea.MouseMiddle || msg.Y == tabRow && x >= slot.x+slot.width-3
		if close {
			m.lastClickKey = ""
			if slot.index < 0 {
				m.closeDiff()
				return nil
			}
			return m.requestClose([]*editor.Document{m.tabs[slot.index].doc})
		}
		if slot.index < 0 {
			m.showEditor = false
			m.sidebar = false
			return nil
		}
		m.activateTab(slot.index)
		if m.doubleClick("tab:" + m.tabs[slot.index].doc.Path) {
			m.keepTab()
		}
		return nil
	}
	return nil
}

func (m *Model) tabBar() string {
	w := m.mainWidth()
	text := ""
	x := 0
	for _, slot := range m.tabLayout() {
		text += row("", slot.x-x, theme.Foreground, theme.Panel)
		if slot.control != 0 {
			glyph := " " + m.symbol(symbolLeft) + " "
			if slot.control > 0 {
				glyph = " " + m.symbol(symbolRight) + " "
			}
			text += row(glyph, slot.width, theme.Muted, theme.Panel)
		} else {
			active := !m.showEditor && slot.index < 0 || m.showEditor && slot.index == m.activeTab
			fg, bg := theme.Muted, theme.Panel
			if active {
				fg, bg = "#FFFFFF", theme.Background
			}
			glyph, color, preview := m.symbol(symbolDiff), theme.Muted, false
			if slot.index < 0 && m.diffPath != "" {
				icon := fileicon.For(m.diffPath, m.nerd)
				glyph, color = icon.Glyph, icon.Color
			}
			label := slot.label
			suffix := ""
			if slot.index >= 0 {
				t := m.tabs[slot.index]
				icon := fileicon.For(t.doc.Path, m.nerd)
				glyph, color, preview = icon.Glyph, icon.Color, t.preview
				if t.doc.Dirty() {
					suffix = " " + m.symbol(symbolDot)
				}
			}
			label = ansi.Truncate(label, max(0, slot.width-7-ansi.StringWidth(suffix)), "…") + suffix
			label += strings.Repeat(" ", max(0, slot.width-7-ansi.StringWidth(label)))
			closeColor := theme.Muted
			if active {
				closeColor = fg
			}
			content := " " + row(glyph, 2, color, bg) + theme.Style(fg, bg).Italic(preview).Render(label) + theme.Style(closeColor, bg).Render(" "+m.symbol(symbolClose)+" ") + theme.Style(theme.Border, bg).Render("│")
			text += row(content, slot.width, fg, bg)
		}
		x = slot.x + slot.width
	}
	return row(text, w, theme.Foreground, theme.Panel)
}

func (m *Model) clickModal(msg tea.MouseClickMsg) tea.Cmd {
	if m.isPicker() {
		return m.clickPicker(msg)
	}
	if m.modal != "close" {
		return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
	if msg.Button != tea.MouseLeft {
		return m.chooseClose("cancel")
	}
	w := min(76, m.width-6)
	x := (m.width - w) / 2
	y := min(3, max(0, m.height-6))
	if msg.Y != y+3 || msg.X <= x || msg.X >= x+w-1 {
		return m.chooseClose("cancel")
	}
	column := msg.X - x - 1
	if column < 6 {
		return m.chooseClose("save")
	}
	if column < 15 {
		return m.chooseClose("discard")
	}
	if column < 23 {
		return m.chooseClose("cancel")
	}
	return m.chooseClose("cancel")
}

func (m *Model) fileDirty(path string) bool {
	path = filepath.Join(m.repo.Root, path)
	for _, tab := range m.tabs {
		if tab.doc.Path == path && tab.doc.Dirty() {
			return true
		}
	}
	return false
}

func (m *Model) hasDirtyTabs() bool {
	for _, t := range m.tabs {
		if t.doc.Dirty() {
			return true
		}
	}
	return false
}

func (m *Model) clearDiffContent() {
	if m.diffSyntaxCancel != nil {
		m.diffSyntaxCancel()
		m.diffSyntaxCancel = nil
	}
	m.diffSyntax = nil
	m.diffEmphasis = nil
	m.patch = diff.Patch{}
	m.diffRows = nil
	m.diffLayout = diffLayoutKey{}
	m.diffRowHunks = nil
	m.viewport.SetContent("")
}
