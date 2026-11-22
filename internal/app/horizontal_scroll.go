package app

// Horizontal wheel scrolling leaves the cursor and vertical position untouched.
func (m *Model) scrollHorizontal(delta int) bool {
	if m.editing() {
		t := &m.tabs[m.activeTab]
		if t.scrollText != t.doc.Text {
			t.scrollText, t.scrollWidth = t.doc.Text, 0
			tableRows := t.tableRows()
			for i, line := range t.doc.Lines() {
				t.scrollWidth = max(t.scrollWidth, displayWidth(line, 0, tableRows[i]))
			}
		}
		left := t.left
		t.left = min(max(0, t.left+delta), max(0, t.scrollWidth-max(1, m.mainWidth()-7)))
		return t.left != left
	}
	if m.diffOpen {
		left := m.diffLeft
		m.diffLeft += delta
		m.clampDiffHorizontal()
		return m.diffLeft != left
	}
	return false
}

func (m *Model) clampDiffHorizontal() {
	_, width := m.diffPane(false)
	m.diffLeft = min(max(0, m.diffLeft), max(0, m.diffWidth-max(1, width-m.diffGutter)))
}
