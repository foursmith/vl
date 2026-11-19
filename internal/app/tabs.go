package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/fileicon"
	"github.com/foursmith/vl/internal/imagepreview"
	"github.com/foursmith/vl/internal/theme"
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

func (m *Model) openDocument(path string, preview bool) bool {
	show := m.selectTab
	if !preview {
		show = m.activateTab
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(m.repo.Root, path)
	}
	abs = filepath.Clean(abs)
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	for i := range m.tabs {
		if m.tabs[i].diff == nil && m.tabs[i].path() == abs {
			show(i)
			return true
		}
	}
	if imagepreview.Supported(abs) {
		im, err := imagepreview.Open(m.ctx, abs)
		if err != nil {
			m.notice = m.tr(err.Error())
			return false
		}
		show(m.insertTab(tab{image: newImageState(im, m.allocateImageID()), preview: preview}))
		return true
	}
	d, err := editor.Open(abs)
	if err != nil {
		m.notice = m.tr(err.Error())
		return false
	}
	show(m.insertTab(tab{doc: d, preview: preview}))
	return true
}

func (m *Model) insertTab(next tab) int {
	defer m.syncWatch()
	m.restoreFilePosition(&next)
	m.keepEditedTabs()
	if next.preview {
		for i := range m.tabs {
			if m.tabs[i].preview && !m.tabs[i].dirty() {
				m.rememberFilePosition(&m.tabs[i])
				m.retireImage(&m.tabs[i])
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
	if t.image != nil {
		return fmt.Sprintf("tab:image:%p", t.image)
	}
	return fmt.Sprintf("tab:file:%p", t.doc)
}

func (t tab) path() string {
	if t.diff != nil {
		return t.diff.diffPath
	}
	if t.image != nil {
		return t.image.Path
	}
	return t.doc.Path
}

func (m *Model) editing() bool { return m.document() != nil }

// selectTab changes the displayed content while leaving keyboard focus where it is.
func (m *Model) selectTab(index int) {
	m.blurFileFind()
	m.leaderTarget = ""
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) && m.activeTab != index {
		m.rememberFilePosition(&m.tabs[m.activeTab])
	}
	m.stopTextDrag()
	m.imagePan = nil
	m.activeTab = index
	m.diffState = m.emptyDiff
	if t := &m.tabs[index]; t.diff != nil {
		m.diffState = t.diff
		m.lastFileRepository = t.diff.diffRepository
		m.renderDiff()
	} else {
		m.lastFileRepository = m.fileRepository(t.path())
	}
	m.ensureEditor()
	if im := m.tabs[index].image; im != nil {
		m.imageUse++
		im.lastUsed = m.imageUse
	}
	m.trimImageTabs()
	if t := &m.tabs[index]; t.doc != nil && t.positionBaseline == nil {
		t.capturePosition()
	}
	m.ensureTabVisible()
}

func (m *Model) activateTab(index int) {
	m.selectTab(index)
	m.focusEditor()
}

func (m *Model) focusEditor() {
	m.sidebar = false
	m.keepTab()
}

func (m *Model) keepTab() tea.Cmd {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		m.tabs[m.activeTab].preview = false
	}
	return nil
}

func (m *Model) closeTab() tea.Cmd {
	return m.closeTabAt(m.activeTab)
}

func (m *Model) closeTabAt(index int) tea.Cmd {
	if index < 0 || index >= len(m.tabs) {
		return nil
	}
	return m.requestCloseTabs(m.tabs[index : index+1])
}

func (m *Model) closeAllTabs() tea.Cmd {
	return m.requestCloseTabs(append([]tab(nil), m.tabs...))
}

func (m *Model) closeOtherTabs() tea.Cmd {
	return m.closeOtherTabsExcept(m.activeTab)
}

func (m *Model) closeOtherTabsExcept(index int) tea.Cmd {
	var close []tab
	for i, t := range m.tabs {
		if i != index {
			close = append(close, t)
		}
	}
	return m.requestCloseTabs(close)
}

func (m *Model) requestCloseTabs(tabs []tab) tea.Cmd {
	var docs []*editor.Document
	for _, target := range tabs {
		if target.doc != nil {
			docs = append(docs, target.doc)
			continue
		}
		for i, t := range m.tabs {
			if t.diff == target.diff && t.image == target.image {
				m.removeTabAt(i)
				break
			}
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
			path, err := filepath.Rel(m.repo.Root, d.Path)
			if err != nil {
				path = d.Path
			}
			m.askSaveChanges(m.tr("Save changes to %s?", path), m.chooseClose)
			return nil
		}
		m.removeTab(d)
		m.closeQueue = m.closeQueue[1:]
	}
	m.closeQueue = nil
	m.dismissDialog()
	if m.editorOnly && len(m.tabs) == 0 {
		return tea.Quit
	}
	return nil
}

func (m *Model) chooseClose(choice string) tea.Cmd {
	if choice == "cancel" {
		m.closeQueue = nil
		m.dismissDialog()
		return nil
	}
	if len(m.closeQueue) == 0 {
		m.dismissDialog()
		return nil
	}
	d := m.closeQueue[0]
	if choice == "save" && !m.saveDialogDocument(d) {
		return nil
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

func (m *Model) removeTabAt(index int) {
	m.lastClickKey = ""
	t := &m.tabs[index]
	m.rememberFilePosition(t)
	m.retireImage(t)
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
		m.sidebar = !m.editorOnly
	} else if wasActive {
		next := min(index, len(m.tabs)-1)
		if m.sidebar {
			m.selectTab(next)
		} else {
			m.activateTab(next)
		}
	}
	m.syncWatch()
	m.ensureEditor()
	m.ensureTabVisible()
}

func (t *tab) close() {
	t.highlightJob.stop()
	if cache := t.markdownImages; cache != nil {
		if cache.cancel != nil {
			cache.cancel()
		}
		if cache.remoteScan != nil {
			cache.remoteScan.cancel()
		}
		for _, state := range cache.remote {
			if state.cancel != nil {
				state.cancel()
			}
		}
	}
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
		if d.diffPR.Number != 0 {
			return safe(label) + fmt.Sprintf(" · PR #%d", d.diffPR.Number)
		}
		if d.diffCommit != "" {
			return safe(label) + " · " + shortOID(d.diffCommit)
		}
		if d.diffStaged {
			return safe(label) + m.tr(" · Staged")
		}
		return safe(label) + m.tr(" · Diff")
	}
	for i, other := range m.tabs {
		if i != index && other.diff == nil && filepath.Base(other.path()) == label {
			label += " · " + filepath.Base(filepath.Dir(t.path()))
			break
		}
	}
	return safe(label)
}

// Tab decorations use the same status and colors as Explorer.
func (m *Model) tabDecoration(index int) item {
	t := m.tabs[index]
	if t.diff != nil || m.editorOnly {
		return item{}
	}
	path, err := filepath.Rel(m.repo.Root, t.path())
	if err != nil {
		return item{}
	}
	path = filepath.ToSlash(path)
	for i := range m.snapshot.Changes {
		c := &m.snapshot.Changes[i]
		if c.Path == path {
			staged := c.Worktree == ' '
			return item{status: changeStatus(c, staged), staged: staged, change: c}
		}
	}
	return item{}
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
		if m.tabs[all[i].index].dirty() || m.tabDecoration(all[i].index).status != 0 {
			extra = 2
		}
		maxWidth := 28
		if d := m.tabs[all[i].index].diff; d != nil && (d.diffCommit != "" || d.diffPR.Number != 0) {
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
			return m.closeTabAt(slot.index)
		}
		m.lastClickKey = ""
		m.activateTab(slot.index)
		return nil
	}
	return nil
}

func (m *Model) tabBar() string {
	w := m.mainWidth()
	text := ""
	x := 0
	for _, slot := range m.tabLayout() {
		text += row("", slot.x-x, m.colors.Foreground, m.colors.TabBarBackground)
		if slot.control != 0 {
			glyph := " " + m.symbol(symbolLeft) + " "
			if slot.control > 0 {
				glyph = " " + m.symbol(symbolRight) + " "
			}
			text += row(glyph, slot.width, theme.Readable(m.colors.Muted, m.colors.TabBarBackground), m.colors.TabBarBackground)
		} else {
			active := slot.index == m.activeTab
			fg, bg := m.colors.TabInactiveForeground, m.colors.TabInactiveBackground
			if active {
				fg, bg = m.colors.TabActiveForeground, m.colors.TabActiveBackground
			}
			t := m.tabs[slot.index]
			icon := fileicon.For(t.path(), m.nerd)
			glyph, color, preview := icon.Glyph, icon.Color, t.preview
			label := slot.label
			decoration := m.tabDecoration(slot.index)
			if decoration.status != 0 {
				fg = m.gitDecorationColor(decoration)
			}
			suffix := ""
			if decoration.status != 0 {
				suffix = " " + string(decoration.status)
			}
			if t.dirty() {
				suffix = " " + m.symbol(symbolDot)
			}
			label = ansi.Truncate(label, max(0, slot.width-7-ansi.StringWidth(suffix)), "…") + suffix
			label += strings.Repeat(" ", max(0, slot.width-7-ansi.StringWidth(label)))
			closeColor := m.colors.Muted
			if active {
				closeColor = m.colors.TabActiveForeground
			}
			divider := row(" ", 1, fg, bg)
			if m.colors.TabBorder != "" {
				divider = theme.Style(m.colors.TabBorder, bg).Render("│")
			}
			indicator := " "
			if active {
				indicator = theme.Style(m.colors.TabActiveBorderTop, bg).Render("▎")
			}
			content := indicator + row(glyph, 2, color, bg) + theme.TextStyle(fg, bg).Italic(preview).Underline(m.monochrome && active).Render(label) + theme.TextStyle(closeColor, bg).Render(" "+m.symbol(symbolClose)+" ") + divider
			text += row(content, slot.width, fg, bg)
		}
		x = slot.x + slot.width
	}
	return row(text, w, m.colors.Foreground, m.colors.TabBarBackground)
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
