package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/imagepreview"
	"github.com/ourongxing/elegit/internal/watcher"
)

type watchReadyMsg struct {
	watcher *watcher.Watcher
	err     error
}
type watchMsg struct {
	batch watcher.Batch
	open  bool
}
type fileReloadMsg struct {
	original *editor.Document
	fresh    *editor.Document
	seq      uint64
	err      error
}

func (m *Model) startWatcher() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		w, err := watcher.New(ctx)
		return watchReadyMsg{watcher: w, err: err}
	}
}
func (m *Model) nextWatch() tea.Cmd {
	w := m.watcher
	return func() tea.Msg { b, open := w.Next(); return watchMsg{b, open} }
}
func (m *Model) syncWatch() {
	if m.watcher == nil {
		return
	}
	config := watcher.Config{Directories: []string{m.repo.Root}}
	seen := map[string]bool{".": true, "": true}
	for _, entry := range m.treeEntries {
		// Watch compacted ancestors too so adding a sibling splits the row.
		path := entry.Path
		if !entry.Directory {
			path = filepath.ToSlash(filepath.Dir(path))
		}
		for !seen[path] {
			seen[path] = true
			config.Directories = append(config.Directories, filepath.Join(m.repo.Root, path))
			path = filepath.ToSlash(filepath.Dir(path))
		}
	}
	for _, t := range m.tabs {
		if t.diff == nil {
			config.Directories = append(config.Directories, filepath.Dir(t.path()))
		}
	}
	for _, path := range m.snapshot.Repositories {
		config.Directories = append(config.Directories, filepath.Join(m.repo.Root, path))
	}
	config.Directories = append(config.Directories, m.snapshot.GitDirectories...)
	for _, dir := range m.snapshot.GitDirectories {
		config.Recursive = append(config.Recursive, filepath.Join(dir, "refs"))
	}
	m.watcher.Sync(config)
}
func (m *Model) handleWatch(batch watcher.Batch) tea.Cmd {
	if batch.Err != nil {
		m.watchDirty = true
		m.log(m.tr("File watcher: %s", batch.Err.Error()))
	}
	for _, path := range batch.Paths {
		metadata := false
		for _, dir := range m.snapshot.GitDirectories {
			if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
				metadata = true
				break
			}
		}
		if metadata && strings.HasSuffix(path, ".lock") {
			continue
		}
		m.watchDirty = true
	}
	if batch.Err != nil {
		return m.reloadWatchedFiles(nil)
	}
	return m.reloadWatchedFiles(batch.Paths)
}

func (m *Model) reloadWatchedFiles(paths []string) tea.Cmd {
	var commands []tea.Cmd
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.diff != nil {
			continue
		}
		changed := paths == nil
		for _, path := range paths {
			if path == t.path() || strings.HasPrefix(t.path(), path+string(filepath.Separator)) {
				changed = true
				break
			}
		}
		if !changed {
			continue
		}
		t.reloadSeq++
		if t.image != nil {
			original, path, seq := t.image.Image, t.path(), t.reloadSeq
			commands = append(commands, func() tea.Msg {
				fresh, err := imagepreview.Open(path)
				return imageReloadMsg{original, fresh, seq, err}
			})
			continue
		}
		original, path, seq := t.doc, t.doc.Path, t.reloadSeq
		commands = append(commands, func() tea.Msg { fresh, err := editor.Open(path); return fileReloadMsg{original, fresh, seq, err} })
	}
	return tea.Batch(commands...)
}
func (m *Model) finishReload(msg fileReloadMsg) {
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc == nil {
			continue
		}
		if t.doc != msg.original || t.reloadSeq != msg.seq {
			continue
		}
		if msg.err != nil {
			t.reloadWhenClean = false
			notice := m.tr("File unavailable on disk: %s: %s", filepath.Base(t.doc.Path), m.tr(msg.err.Error()))
			if t.diskNotice != notice {
				t.diskNotice = notice
				m.log(notice)
			}
			return
		}
		if t.doc.SameDisk(msg.fresh) {
			t.reloadWhenClean = false
			t.diskNotice = ""
			return
		}
		if t.doc.Dirty() {
			t.reloadWhenClean = true
			notice := m.tr("File changed on disk; keeping unsaved changes: %s", filepath.Base(t.doc.Path))
			if t.diskNotice != notice {
				t.diskNotice = notice
				m.log(notice)
			}
			return
		}
		line, col := t.doc.Position()
		msg.fresh.SetPosition(line, col)
		t.highlightJob.stop()
		t.doc = msg.fresh
		t.highlightText, t.spans, t.renderedRows = "", nil, nil
		t.diskNotice = ""
		t.reloadWhenClean = false
		m.clampEditorScroll(t)
		m.syncWatch()
		return
	}
}

// Close releases watcher and background requests when the program exits.
func (m *Model) Close() {
	for i := range m.tabs {
		m.tabs[i].close()
	}
	m.cancel()
	if m.watcher != nil {
		m.watcher.Close()
	}
}

// A conflicting external edit can be applied once undo returns the buffer to
// its saved state. Clear the flag before scheduling so mouse events don't retry.
func (m *Model) reloadCleanConflicts() tea.Cmd {
	var commands []tea.Cmd
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc == nil {
			continue
		}
		if t.reloadWhenClean && !t.doc.Dirty() {
			t.reloadWhenClean = false
			commands = append(commands, m.reloadWatchedFiles([]string{t.doc.Path}))
		}
	}
	return tea.Batch(commands...)
}
