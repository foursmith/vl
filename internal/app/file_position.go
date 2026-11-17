package app

import (
	"errors"

	"github.com/ourongxing/vl/internal/config"
)

func documentPosition(t tab) config.FilePosition {
	line, column := t.doc.Position()
	return config.FilePosition{Line: line, Column: column, Top: t.top, Left: t.left}
}

func (m *Model) rememberFilePosition(t *tab) error {
	if t.doc == nil {
		return nil
	}
	position := documentPosition(*t)
	if t.positionBaseline != nil && *t.positionBaseline == position {
		return nil
	}
	if m.filePositionStore != nil {
		if err := m.filePositionStore.Save(t.doc.Path, position); err != nil {
			m.log(m.tr("Save file position: %s", err.Error()))
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
		m.log(m.tr("Load file position: %s", err.Error()))
		return
	}
	if exists {
		t.doc.SetPosition(saved.Line, saved.Column)
		t.top, t.left = max(0, saved.Top), max(0, saved.Left)
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
