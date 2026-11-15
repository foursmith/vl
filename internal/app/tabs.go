package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

func (m *Model) keepEditedTabs() {
	for i := range m.tabs {
		if m.tabs[i].dirty() {
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
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(m.repo.Root, path)
	}
	abs = filepath.Clean(abs)
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	for i := range m.tabs {
		if m.tabs[i].doc != nil && m.tabs[i].doc.Path == abs {
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
	m.activateTab(m.insertTab(tab{doc: d, preview: preview}))
	return true
}

func (m *Model) insertTab(next tab) int {
	defer m.syncWatch()
	m.keepEditedTabs()
	if next.preview {
		for i := range m.tabs {
			if m.tabs[i].preview && !m.tabs[i].dirty() {
				m.tabs[i].close()
				m.tabs[i] = next
				return i
			}
		}
	}
	m.tabs = append(m.tabs, next)
	return len(m.tabs) - 1
}

func (t tab) dirty() bool { return t.doc != nil && t.doc.Dirty() }

func (t tab) clickKey() string {
	if t.diff != nil {
		return fmt.Sprintf("tab:diff:%p", t.diff)
	}
	return fmt.Sprintf("tab:file:%p", t.doc)
}

func (t tab) path() string {
	if t.diff != nil {
		return t.diff.diffPath
	}
	return t.doc.Path
}

func (m *Model) editing() bool { return m.document() != nil }

func (m *Model) activateTab(index int) {
	m.stopTextDrag()
	m.activeTab = index
	m.pendingHistory = ""
	m.diffState = m.emptyDiff
	if t := &m.tabs[index]; t.diff != nil {
		m.diffState = t.diff
		m.lastFileRepository = t.diff.diffRepository
		m.viewport.SetWidth(m.mainWidth())
		m.viewport.SetHeight(m.contentHeight())
		m.renderDiff()
	} else {
		m.lastFileRepository = m.fileRepository(t.doc.Path)
	}
	m.sidebar = false
	m.ensureEditor()
	m.ensureTabVisible()
}

func (m *Model) keepTab() tea.Cmd {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		m.tabs[m.activeTab].preview = false
	}
	return nil
}

func (m *Model) closeTab() tea.Cmd {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return nil
	}
	return m.requestCloseTabs([]tab{m.tabs[m.activeTab]})
}

func (m *Model) closeAllTabs() tea.Cmd {
	return m.requestCloseTabs(append([]tab(nil), m.tabs...))
}

func (m *Model) closeOtherTabs() tea.Cmd {
	var close []tab
	for i, t := range m.tabs {
		if i != m.activeTab {
			close = append(close, t)
		}
	}
	return m.requestCloseTabs(close)
}

func (m *Model) requestCloseTabs(tabs []tab) tea.Cmd {
	var docs []*editor.Document
	for _, t := range tabs {
		if t.diff != nil {
			m.removeDiff(t.diff)
		} else {
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
	for i, t := range m.tabs {
		if t.doc == doc && doc != nil {
			m.removeTabAt(i)
			return
		}
	}
}

func (m *Model) removeDiff(state *diffState) {
	for i, t := range m.tabs {
		if t.diff == state && state != nil {
			m.removeTabAt(i)
			return
		}
	}
}

func (m *Model) removeTabAt(index int) {
	t := &m.tabs[index]
	t.close()
	wasActive := index == m.activeTab
	copy(m.tabs[index:], m.tabs[index+1:])
	m.tabs[len(m.tabs)-1] = tab{}
	m.tabs = m.tabs[:len(m.tabs)-1]
	if index < m.activeTab {
		m.activeTab--
	}
	if len(m.tabs) == 0 {
		m.activeTab = -1
		m.diffState = m.emptyDiff
		m.sidebar = true
	} else if wasActive {
		sidebar := m.sidebar
		m.activateTab(min(index, len(m.tabs)-1))
		m.sidebar = sidebar
	}
	m.syncWatch()
	m.ensureEditor()
	m.ensureTabVisible()
}

func (t *tab) close() {
	t.highlightJob.stop()
	if t.diff != nil {
		t.diff.close()
	}
}

type tabSlot struct {
	x, width, index int
	label           string
	control         int
}

func (m *Model) tabLabel(index int) string {
	t := m.tabs[index]
	label := filepath.Base(t.path())
	if d := t.diff; d != nil {
		if d.diffCommit != "" {
			return safe(label) + " · " + d.diffCommit[:min(7, len(d.diffCommit))]
		}
		if d.diffStaged {
			return safe(label) + " · 暂存"
		}
		return safe(label) + " · Diff"
	}
	for i, other := range m.tabs {
		if i != index && other.doc != nil && filepath.Base(other.doc.Path) == label {
			label += " · " + filepath.Base(filepath.Dir(t.doc.Path))
			break
		}
	}
	return safe(label)
}

func (m *Model) tabLayout() []tabSlot {
	var all []tabSlot
	for index := range m.tabs {
		all = append(all, tabSlot{index: index, label: m.tabLabel(index)})
	}
	if len(all) == 0 {
		return nil
	}
	w := m.mainWidth()
	total := 0
	for i := range all {
		extra := 0
		if m.tabs[all[i].index].dirty() {
			extra = 2
		}
		maxWidth := 28
		if m.tabs[all[i].index].diff != nil && m.tabs[all[i].index].diff.diffCommit != "" {
			maxWidth = 60
		}
		all[i].width = min(maxWidth, max(12, ansi.StringWidth(all[i].label)+extra+7))
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
	count := len(m.tabs)
	m.tabStart = min(max(0, m.tabStart), max(0, count-1))
	for tries := 0; tries < count; tries++ {
		for _, slot := range m.tabLayout() {
			if slot.control == 0 && slot.index == m.activeTab {
				return
			}
		}
		if m.activeTab < m.tabStart {
			m.tabStart = max(0, m.activeTab)
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
				m.tabStart = min(max(0, count-1), max(0, m.tabStart+slot.control))
			}
			return nil
		}
		close := msg.Button == tea.MouseMiddle || msg.Y == tabRow && x >= slot.x+slot.width-3
		if close {
			m.lastClickKey = ""
			return m.requestCloseTabs([]tab{m.tabs[slot.index]})
		}
		m.activateTab(slot.index)
		if m.doubleClick(m.tabs[slot.index].clickKey()) {
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
		text += row("", slot.x-x, m.colors.Foreground, m.colors.Panel)
		if slot.control != 0 {
			glyph := " " + m.symbol(symbolLeft) + " "
			if slot.control > 0 {
				glyph = " " + m.symbol(symbolRight) + " "
			}
			text += row(glyph, slot.width, m.colors.Muted, m.colors.Panel)
		} else {
			active := slot.index == m.activeTab
			fg, bg := m.colors.Muted, m.colors.Panel
			if active {
				fg, bg = m.colors.Strong, m.colors.Background
			}
			t := m.tabs[slot.index]
			icon := fileicon.For(t.path(), m.nerd)
			glyph, color, preview := icon.Glyph, icon.Color, t.preview
			label := slot.label
			suffix := ""
			if t.dirty() {
				suffix = " " + m.symbol(symbolDot)
			}
			label = ansi.Truncate(label, max(0, slot.width-7-ansi.StringWidth(suffix)), "…") + suffix
			label += strings.Repeat(" ", max(0, slot.width-7-ansi.StringWidth(label)))
			closeColor := m.colors.Muted
			if active {
				closeColor = fg
			}
			content := " " + row(glyph, 2, color, bg) + theme.Style(fg, bg).Italic(preview).Render(label) + theme.Style(closeColor, bg).Render(" "+m.symbol(symbolClose)+" ") + theme.Style(m.colors.Border, bg).Render("│")
			text += row(content, slot.width, fg, bg)
		}
		x = slot.x + slot.width
	}
	return row(text, w, m.colors.Foreground, m.colors.Panel)
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
		if tab.doc != nil && tab.doc.Path == path && tab.doc.Dirty() {
			return true
		}
	}
	return false
}

func (m *Model) hasDirtyTabs() bool {
	for _, t := range m.tabs {
		if t.dirty() {
			return true
		}
	}
	return false
}
