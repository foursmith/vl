package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/watcher"
)

type watchReadyMsg struct {
	watcher  *watcher.Watcher
	metadata []string
	err      error
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
	ctx, repo := m.ctx, m.repo
	return func() tea.Msg {
		dirs, err := repo.MetadataDirectories(ctx)
		if err != nil {
			return watchReadyMsg{err: err}
		}
		w, err := watcher.New(ctx)
		return watchReadyMsg{watcher: w, metadata: dirs, err: err}
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
	for _, entry := range m.treeEntries {
		if entry.Directory && m.expanded[entry.Path] {
			config.Directories = append(config.Directories, filepath.Join(m.repo.Root, entry.Path))
		}
	}
	for _, t := range m.tabs {
		config.Directories = append(config.Directories, filepath.Dir(t.doc.Path))
	}
	for _, path := range m.snapshot.Repositories {
		config.Directories = append(config.Directories, filepath.Join(m.repo.Root, path))
	}
	config.Directories = append(config.Directories, m.snapshot.GitDirectories...)
	for _, dir := range m.snapshot.GitDirectories {
		config.Recursive = append(config.Recursive, filepath.Join(dir, "refs"))
	}
	config.Directories = append(config.Directories, m.watchMetadata...)
	for _, dir := range m.watchMetadata {
		config.Recursive = append(config.Recursive, filepath.Join(dir, "refs"))
	}
	m.watcher.Sync(config)
}
func (m *Model) handleWatch(batch watcher.Batch) tea.Cmd {
	if batch.Err != nil {
		m.watchDirty = true
		m.log("文件监视: " + batch.Err.Error())
	}
	for _, path := range batch.Paths {
		metadata := false
		for _, dir := range m.watchMetadata {
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
		changed := paths == nil
		for _, path := range paths {
			if path == t.doc.Path || strings.HasPrefix(t.doc.Path, path+string(filepath.Separator)) {
				changed = true
				break
			}
		}
		if !changed {
			continue
		}
		t.reloadSeq++
		original, path, seq := t.doc, t.doc.Path, t.reloadSeq
		commands = append(commands, func() tea.Msg { fresh, err := editor.Open(path); return fileReloadMsg{original, fresh, seq, err} })
	}
	return tea.Batch(commands...)
}
func (m *Model) finishReload(msg fileReloadMsg) {
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc != msg.original || t.reloadSeq != msg.seq {
			continue
		}
		if msg.err != nil {
			t.reloadWhenClean = false
			notice := "文件在磁盘上不可用: " + filepath.Base(t.doc.Path) + ": " + msg.err.Error()
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
			notice := "文件已被外部修改，保留未保存内容: " + filepath.Base(t.doc.Path)
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
		if t.reloadWhenClean && !t.doc.Dirty() {
			t.reloadWhenClean = false
			commands = append(commands, m.reloadWatchedFiles([]string{t.doc.Path}))
		}
	}
	return tea.Batch(commands...)
}
