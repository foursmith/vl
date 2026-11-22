package app

import (
	"sort"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) vimKey(k string) (bool, tea.Cmd) {
	v := &m.tabs[m.activeTab].vim
	d := m.editorDocument()
	if d.ReadOnly && !v.CommandPending() && !v.VisualMode {
		if k == "enter" {
			return m.toggleNearestDiffFold()
		}
		if k == "[" {
			m.nextHunk(-1)
			return true, nil
		}
		if k == "]" {
			m.nextHunk(1)
			return true, nil
		}
	}
	if isHalfPageKey(k) {
		m.scrollTextHalfPage(k)
		return true, nil
	}
	if !v.InsertMode && !v.CommandPending() && (k == "pgup" || k == "pgdown") {
		rows := m.contentHeight()
		if k == "pgup" {
			rows = -rows
		}
		before, _ := d.Position()
		v.MovePage(d, rows)
		m.moveAcrossFolds(before)
		m.ensureTextCursor()
		return true, nil
	}
	if k == "f" {
		if operator, ok := v.FlashOperator(); ok {
			return true, m.startFlash(operator)
		}
	}
	if !v.InsertMode && !v.VisualMode && (k == ":" || k == "/" || k == "n") && !v.CommandPending() {
		v.CancelPending()
		if k == ":" {
			return true, m.openQuickPicker(":")
		}
		if k == "/" {
			return true, m.openFileFind()
		}
		return true, m.findNext()
	}
	before, _ := d.Position()
	vertical := !v.InsertMode && !v.CommandPending() && (k == "j" || k == "k" || k == "up" || k == "down")
	// Every undo/redo binding must cancel pending operators and invalidate insert replay.
	switch {
	case matchesShortcut(k, editorKeys.Undo):
		k = "ctrl+z"
	case matchesShortcut(k, editorKeys.Redo):
		k = "ctrl+shift+z"
	}
	handled, text := v.Handle(d, k, m.vimRegister)
	if !handled {
		return false, nil
	}
	if vertical {
		m.moveAcrossFolds(before)
	}
	m.ensureTextCursor()
	if text != "" {
		m.vimRegister = text
	}
	if action := v.TakeFoldAction(); action != "" && m.editing() {
		m.editorFold(action)
	}
	return true, nil
}

func (m *Model) scrollTextHalfPage(k string) {
	d := m.editorDocument()
	if d == nil {
		return
	}
	t := &m.tabs[m.activeTab]
	height := m.contentHeight()
	revealCursor := true
	t.vim.ScrollPage(d, pageDelta(k, height), func(line, delta int) int {
		if m.editing() {
			m.syncMarkdownImageRows(t)
			top := t.editorDisplayTop()
			cursor := min(top+height-1, max(top, t.editorDisplayRow(line)))
			t.setEditorDisplayTop(top + delta)
			m.clampEditorScroll(t)
			target, image, _ := t.editorDisplayAt(cursor + delta)
			// An embedded image can fill the viewport without a text row to reveal.
			if image != nil && t.editorDisplayRow(target) < t.editorDisplayTop() {
				next, _, _ := t.editorDisplayAt(image.first + image.region.Rows)
				if next < len(d.Lines()) && t.editorDisplayRow(next) < t.editorDisplayTop()+height {
					return next
				}
				revealCursor = false
			}
			return target
		}
		if len(m.reviewRows) == 0 {
			return line
		}
		top := m.viewport.YOffset()
		cursor := m.reviewRows[min(line, len(m.reviewRows)-1)]
		target := min(top+height-1, max(top, cursor)) + delta
		m.viewport.SetYOffset(top + delta)
		index := sort.SearchInts(m.reviewRows, target)
		if delta < 0 && index > 0 && (index == len(m.reviewRows) || m.reviewRows[index] != target) {
			index--
		}
		return min(index, len(m.reviewRows)-1)
	})
	if revealCursor {
		m.ensureTextCursor()
	}
}
