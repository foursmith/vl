package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/editor"
	searchservice "github.com/ourongxing/vl/internal/search"
)

// Validate all targets before writing. Edits to open documents remain undoable.
func (m *Model) replaceSearch(all bool) tea.Cmd {
	s := &m.search
	if s.loading || s.err != "" || len(s.result.Files) == 0 {
		return nil
	}
	if m.busy {
		m.notice = searchText("Wait for the current Git operation before replacing")
		return nil
	}
	files, selectedMatch := s.result.Files, -1
	if !all {
		selected, ok := m.selectedSearchRow()
		if !ok {
			return nil
		}
		files = files[selected.file : selected.file+1]
		selectedMatch = selected.match
	}
	if s.result.Unreadable > 0 && selectedMatch < 0 {
		m.notice = searchText("Search is incomplete; resolve unreadable files before bulk replacement")
		return nil
	}
	matcher, err := s.options.Compile()
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	type edit struct {
		doc     *editor.Document
		text    string
		tab     int
		matches int
	}
	replacement := s.replacement.Value()
	edits := make([]edit, 0, len(files))
	for _, f := range files {
		var d *editor.Document
		tabIndex := -1
		for i, t := range m.tabs {
			if t.doc != nil && t.doc.Path == f.Path {
				d = t.doc
				tabIndex = i
				break
			}
		}
		if real, err := filepath.EvalSymlinks(f.Path); err != nil || real != f.Path {
			m.notice = searchText("Search file path changed; search again")
			return m.runSearch()
		}
		if d == nil {
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
		matches := matcher.Find(f.Text, replacement)
		targets := f.Matches
		if selectedMatch >= 0 {
			targets = targets[selectedMatch : selectedMatch+1]
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
		nextText := searchservice.ReplaceText(f.Text, retained)
		if nextText == d.Text {
			continue
		}
		if d.EncodedSize(nextText) > editor.MaxFileSize {
			m.notice = searchText("Replacement exceeds editor file size limit")
			return nil
		}

		edits = append(edits, edit{d, nextText, tabIndex, len(retained)})
	}
	var lastError string
	var count, savedFiles, failedFiles int
	for _, e := range edits {
		line, col := 0, 0
		if e.tab >= 0 {
			line, col = e.doc.Position()
		}
		if err := e.doc.SaveEdit(e.text); err != nil {
			path, relErr := filepath.Rel(m.repo.Root, e.doc.Path)
			if relErr != nil {
				path = e.doc.Path
			}
			lastError = searchText("Could not replace in %s: %s", path, err.Error())
			m.log(lastError)
			failedFiles++
			continue
		}
		savedFiles++
		count += e.matches
		if e.tab >= 0 {
			t := &m.tabs[e.tab]
			t.reloadSeq++
			t.vim.Reset(e.doc)
			e.doc.SetPosition(line, col)
			m.clampEditorScroll(t)
		}
	}
	m.notice = searchText("Replaced %d matches; saved %d files", count, savedFiles)
	if failedFiles > 0 {
		m.notice += searchText("; %d files failed (%s)", failedFiles, lastError)
	}
	if savedFiles > 0 {
		return tea.Batch(m.refresh(), m.runSearch())
	}
	return m.runSearch()
}
