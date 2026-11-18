package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/editor"
	searchservice "github.com/ourongxing/vl/internal/search"
)

// Replace buffers only after validating every target. Each file gets one undo step.
func (m *Model) replaceSearch(all bool) tea.Cmd {
	s := &m.search
	if s.loading || s.err != "" || len(s.result.Files) == 0 {
		return nil
	}
	m.clampSearch()
	rows := m.searchRows()
	selected := rows[s.selected]
	if s.result.Unreadable > 0 && (all || selected.match < 0) {
		m.notice = searchText("Search is incomplete; resolve unreadable files before bulk replacement")
		return nil
	}
	matcher, err := s.options.Compile()
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	type edit struct {
		doc      *editor.Document
		text     string
		existing bool
	}
	var edits []edit
	count := 0
	for fi, f := range s.result.Files {
		if !all && fi != selected.file {
			continue
		}
		var d *editor.Document
		for _, t := range m.tabs {
			if t.doc != nil && t.doc.Path == f.Path {
				d = t.doc
				break
			}
		}
		existing := d != nil
		if d == nil {
			if real, err := filepath.EvalSymlinks(f.Path); err != nil || real != f.Path {
				m.notice = searchText("Search file path changed; search again")
				return m.runSearch()
			}
			var err error
			d, err = editor.Open(f.Path)
			if err != nil {
				m.notice = err.Error()
				return nil
			}
		}
		if d.Text != f.Text {
			m.notice = searchText("Search result changed; search again before replacing")
			return m.runSearch()
		}
		matches := matcher.Find(f.Text, s.replacement.Value())
		targets := f.Matches
		if !all && selected.match >= 0 {
			targets = targets[selected.match : selected.match+1]
		}
		retained := matches[:0]
		next := 0
		for _, match := range matches {
			if next < len(targets) && match.Start == targets[next].Start && match.End == targets[next].End {
				retained = append(retained, match)
				next++
			}
		}
		if next != len(targets) {
			return m.runSearch()
		}
		matches = retained
		nextText := searchservice.ReplaceText(f.Text, matches)
		count += len(matches)
		if len(nextText) > editor.MaxFileSize {
			m.notice = searchText("Replacement exceeds editor file size limit")
			return nil
		}

		edits = append(edits, edit{d, nextText, existing})
	}
	for _, e := range edits {
		for i := range m.tabs {
			if m.tabs[i].doc == e.doc {
				m.tabs[i].vim.Reset(e.doc)
				break
			}
		}
		e.doc.Edit(e.text)
		e.doc.SelectRange(0, 0)
		if !e.existing {
			m.insertTab(tab{doc: e.doc})
		}
	}
	if len(edits) > 0 {
		for i := range m.tabs {
			if m.tabs[i].doc == edits[0].doc {
				m.selectTab(i)
				break
			}
		}
	}
	m.keepEditedTabs()
	m.syncWatch()
	m.sidebar = true
	m.notice = searchText("Replaced %d matches; save edited files with Ctrl+S (Ctrl+Z to undo)", count)
	return m.runSearch()
}
