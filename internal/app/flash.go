package app

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/ourongxing/vl/internal/editor"
)

const flashLabels = "asdfghjklqwertyuiopzxcvbnmASDFGHJKLQWERTYUIOPZXCVBNM0123456789"

type flashTarget struct {
	offset, end int
	next, label string
}
type flashState struct {
	doc                                      *editor.Document
	text, query                              string
	operator                                 string
	cursor, anchor, top, left, width, height int
	visual, linewise                         bool
	targets                                  []flashTarget
	labels                                   string
	page                                     int
	explicit                                 bool
	matches                                  map[int]bool
	marks                                    map[int]string
}

func (m *Model) startFlash(operator string) tea.Cmd {
	d := m.document()
	t := &m.tabs[m.activeTab]
	t.vim.CancelPending()
	m.flash = &flashState{operator: operator, doc: d, text: d.Text, cursor: d.Cursor, anchor: d.Anchor, top: t.top, left: t.left, width: m.mainWidth(), height: m.contentHeight(), visual: t.vim.VisualMode, linewise: t.vim.VisualLine}
	m.flashRevision++
	return nil
}
func (m *Model) cancelFlash() {
	if m.flash != nil {
		m.flash = nil
		m.flashRevision++
		m.hoverOnly = false
		m.hoverRender = hoverRenderCache{}
	}
}
func (m *Model) flashValid() bool {
	f := m.flash
	if f == nil || !m.vimEnabled || m.sidebar || m.modal != "" || m.contextMenu != nil || !m.editing() || m.document() != f.doc {
		return false
	}
	t := &m.tabs[m.activeTab]
	return f.doc.Text == f.text && f.doc.Cursor == f.cursor && f.doc.Anchor == f.anchor && t.top == f.top && t.left == f.left && m.mainWidth() == f.width && m.contentHeight() == f.height && !t.vim.InsertMode && t.vim.VisualMode == f.visual && t.vim.VisualLine == f.linewise
}
func (m *Model) flashKey(msg tea.KeyPressMsg) tea.Cmd {
	if !m.flashValid() {
		m.cancelFlash()
		return nil
	}
	f := m.flash
	k := msg.String()
	switch k {
	case "esc", "ctrl+[":
		m.cancelFlash()
		return nil
	case "backspace":
		iter := graphemes.FromString(f.query)
		end := 0
		for iter.Next() {
			end = iter.Start()
		}
		f.query = f.query[:end]
		f.page = 0
		f.explicit = false
		m.rebuildFlash()
		return nil
	case "tab", "shift+tab":
		if f.labels == "" {
			f.explicit = true
			m.labelFlash()
		} else if len(f.targets) > len(f.labels) {
			pages := (len(f.targets) + len(f.labels) - 1) / len(f.labels)
			delta := 1
			if k == "shift+tab" {
				delta = -1
			}
			f.page = (f.page + delta + pages) % pages
			m.labelFlash()
		}
		m.flashRevision++
		return nil
	case "enter":
		index := f.page * max(1, len(f.labels))
		if index < len(f.targets) {
			return m.finishFlash(f.targets[index].offset)
		}
		return nil
	}
	if msg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta) != 0 || msg.Text == "" {
		m.cancelFlash()
		return m.key(msg)
	}
	for _, target := range f.targets {
		if target.label != "" && target.label == msg.Text {
			return m.finishFlash(target.offset)
		}
	}
	return m.flashText(msg.Text)
}
func (m *Model) flashText(text string) tea.Cmd {
	if !m.flashValid() {
		m.cancelFlash()
		return nil
	}
	if strings.IndexFunc(text, unicode.IsControl) >= 0 {
		return nil
	}
	f := m.flash
	if len(f.query)+len(text) > 1024 {
		return nil
	}
	f.query += text
	f.page = 0
	f.explicit = false
	m.rebuildFlash()
	return nil
}
func (m *Model) finishFlash(offset int) tea.Cmd {
	f := m.flash
	if !m.flashValid() {
		m.cancelFlash()
		return nil
	}
	m.cancelFlash()
	v := &m.tabs[m.activeTab].vim
	v.ApplyFlash(f.doc, offset, f.query, f.operator)
	m.ensureEditor()
	if text, ready := v.TakeClipboardCopy(); ready {
		return func() tea.Msg { return clipboardMsg{err: clipboard.WriteAll(text)} }
	}
	return nil
}
func (m *Model) rebuildFlash() {
	f := m.flash
	f.targets = nil
	if f.query != "" {
		d := f.doc
		seen := map[int]bool{}
		for y := 0; y < m.contentHeight(); y++ {
			row := m.editorRowAt(y)
			if row >= len(d.Lines()) || seen[row] {
				continue
			}
			seen[row] = true
			text := d.Lines()[row]
			type boundary struct{ bytes, runes, cells int }
			var boundaries []boundary
			iter := graphemes.FromString(text)
			runes, cells := 0, 0
			for iter.Next() {
				boundaries = append(boundaries, boundary{iter.Start(), runes, cells})
				runes += utf8.RuneCountInString(iter.Value())
				cells += ansi.StringWidth(displayCluster(iter.Value(), cells))
			}
			boundaries = append(boundaries, boundary{len(text), runes, cells})
			for _, start := range boundaries[:len(boundaries)-1] {
				if start.cells < f.left || start.cells >= f.left+max(1, f.width-7) || !strings.HasPrefix(text[start.bytes:], f.query) {
					continue
				}
				endByte := start.bytes + len(f.query)
				j := sort.Search(len(boundaries), func(n int) bool { return boundaries[n].bytes >= endByte })
				if j >= len(boundaries) || boundaries[j].bytes != endByte || boundaries[j].cells > f.left+max(1, f.width-7) {
					continue
				}
				target := flashTarget{offset: d.LineOffset(row) + start.runes, end: d.LineOffset(row) + boundaries[j].runes}
				if j+1 < len(boundaries) {
					target.next = text[boundaries[j].bytes:boundaries[j+1].bytes]
				}
				f.targets = append(f.targets, target)
			}
		}
	}
	sort.SliceStable(f.targets, func(i, j int) bool {
		a, b := absOffset(f.targets[i].offset-f.cursor), absOffset(f.targets[j].offset-f.cursor)
		if a != b {
			return a < b
		}
		return f.targets[i].offset < f.targets[j].offset
	})
	m.labelFlash()
	m.flashRevision++
}
func absOffset(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (m *Model) labelFlash() {
	f := m.flash
	blocked := map[string]bool{}
	if !f.explicit {
		for _, target := range f.targets {
			blocked[target.next] = true
		}
	}
	f.labels = ""
	for _, label := range flashLabels {
		if !blocked[string(label)] {
			f.labels += string(label)
		}
	}
	f.marks = map[int]string{}
	f.matches = map[int]bool{}
	for i := range f.targets {
		target := &f.targets[i]
		target.label = ""
		if i >= f.page*len(f.labels) && i < (f.page+1)*len(f.labels) {
			target.label = string(f.labels[i-f.page*len(f.labels)])
			f.marks[target.offset] = target.label
		}
		for pos := target.offset; pos < target.end; {
			f.matches[pos] = true
			iter := graphemes.FromString(f.doc.Text[f.doc.ByteOffset(pos):])
			if !iter.Next() {
				break
			}
			pos += utf8.RuneCountInString(iter.Value())
		}
	}
}
func (m *Model) flashLabel() string {
	if !m.flashValid() {
		return ""
	}
	f := m.flash
	prefix := ""
	if f.operator != "" {
		prefix = f.operator + " "
	}
	return fmt.Sprintf("%sFLASH %s · %d", prefix, ansi.Truncate(f.query, 16, "…"), len(f.targets))
}
