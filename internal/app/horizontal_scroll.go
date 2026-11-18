package app

// Horizontal wheel scrolling leaves the cursor and vertical position untouched.
func (m *Model) scrollHorizontal(delta int) {
	if m.editing() {
		t := &m.tabs[m.activeTab]
		if t.scrollText != t.doc.Text {
			t.scrollText, t.scrollWidth = t.doc.Text, 0
			tableRows := t.tableRows()
			for i, line := range t.doc.Lines() {
				t.scrollWidth = max(t.scrollWidth, displayWidth(line, 0, tableRows[i]))
			}
		}
		t.left = min(max(0, t.left+delta), max(0, t.scrollWidth-max(1, m.mainWidth()-7)))
	} else if m.diffOpen {
		m.diffLeft += delta
		m.clampDiffHorizontal()
	}
}

func (m *Model) clampDiffHorizontal() {
	_, width := m.diffPane(false)
	m.diffLeft = min(max(0, m.diffLeft), max(0, m.diffWidth-max(1, width-m.diffGutter)))
}
