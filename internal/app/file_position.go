package app

import (
	"errors"

	"github.com/foursmith/vl/internal/config"
)

func documentPosition(t tab) config.FilePosition {
	line, column := t.doc.Position()
	position := config.FilePosition{Line: line, Column: column, Top: t.top, Left: t.left}
	if t.scopeText == t.doc.Text && len(t.folds) > 0 {
		lines := t.doc.Lines()
		for _, fold := range t.folds {
			position.Folds = append(position.Folds, config.Fold{Start: fold.start, End: fold.end, First: lines[fold.start], Last: lines[fold.end]})
		}
	}
	return position
}

func (m *Model) rememberFilePosition(t *tab) error {
	if t.doc == nil {
		return nil
	}
	position := documentPosition(*t)
	if t.positionBaseline != nil && t.positionBaseline.Equal(position) {
		return nil
	}
	if m.filePositionStore != nil {
		if err := m.filePositionStore.Save(t.doc.Path, position); err != nil {
			m.notice = m.tr("Save file position: %s", err.Error())
			return err
		}
	}
	t.capturePosition()
	return nil
}

func (t *tab) capturePosition() {
	if t.doc != nil {
		position := documentPosition(*t)
		t.positionBaseline = &position
	}
}

func (m *Model) restoreFilePosition(t *tab) {
	if t.doc == nil || m.filePositionStore == nil {
		return
	}
	saved, exists, err := m.filePositionStore.Load(t.doc.Path)
	if err != nil {
		m.notice = m.tr("Load file position: %s", err.Error())
		return
	}
	if exists {
		t.doc.SetPosition(saved.Line, saved.Column)
		t.top, t.left = max(0, saved.Top), max(0, saved.Left)
		t.syncCodeScopes()
		t.folds = nil
		lines := t.doc.Lines()
		for _, fold := range saved.Folds {
			if fold.Start >= 0 && fold.End > fold.Start && fold.End < len(lines) && lines[fold.Start] == fold.First && lines[fold.End] == fold.Last {
				t.folds = append(t.folds, codeScope{start: fold.Start, end: fold.End})
			}
		}
		t.rebuildFoldRows()
	}
}

// SetFilePositionStore restores already-open standalone documents as well as
// enabling lazy restoration when files are opened from any workspace.
func (m *Model) SetFilePositionStore(store *config.FilePositionStore) {
	m.filePositionStore = store
	for i := range m.tabs {
		m.restoreFilePosition(&m.tabs[i])
	}
	m.ensureEditor()
	for i := range m.tabs {
		m.tabs[i].capturePosition()
	}
}

func (m *Model) SaveFilePositions() error {
	var failures []error
	for i := range m.tabs {
		if err := m.rememberFilePosition(&m.tabs[i]); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
