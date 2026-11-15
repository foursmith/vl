// Package watcher coalesces parent-directory notifications for the UI loop.
package watcher

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounce = 150 * time.Millisecond
const maxWait = time.Second

type Config struct{ Directories, Recursive []string }
type Batch struct {
	Paths []string
	Err   error
}
type Watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	configs chan Config
	events  chan Batch
	done    chan struct{}
	syncMu  sync.Mutex
}

func New(ctx context.Context) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &Watcher{ctx: ctx, cancel: cancel, configs: make(chan Config, 1), events: make(chan Batch, 1), done: make(chan struct{})}
	go w.run(fw)
	return w, nil
}

// Sync replaces the desired watches without blocking the UI on filesystem I/O.
func (w *Watcher) Sync(c Config) {
	c.Directories = slices.Clone(c.Directories)
	c.Recursive = slices.Clone(c.Recursive)
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	select {
	case <-w.ctx.Done():
		return
	default:
	}
	select {
	case <-w.configs:
	default:
	}
	select {
	case w.configs <- c:
	case <-w.ctx.Done():
	}
}
func (w *Watcher) Next() (Batch, bool) {
	select {
	case b, ok := <-w.events:
		return b, ok
	case <-w.ctx.Done():
		return Batch{}, false
	}
}
func (w *Watcher) Close() { w.cancel(); <-w.done }

func (w *Watcher) run(fw *fsnotify.Watcher) {
	defer close(w.done)
	defer close(w.events)
	defer fw.Close()
	pending := make(map[string]struct{})
	failures := make(map[string]string)
	var pendingErr error
	var first time.Time
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var alarm <-chan time.Time
	arm := func() {
		now := time.Now()
		if first.IsZero() {
			first = now
		}
		delay := min(debounce, max(time.Duration(0), time.Until(first.Add(maxWait))))
		timer.Reset(delay)
		alarm = timer.C
	}
	reportFailure := func(key string, err error) {
		if message := err.Error(); failures[key] != message {
			failures[key] = message
			pendingErr = err
			arm()
		}
	}
	reconcile := func(c Config) {
		wanted := make(map[string]struct{})
		for _, dir := range c.Directories {
			if real, err := filepath.EvalSymlinks(dir); err == nil {
				dir = real
			}
			wanted[filepath.Clean(dir)] = struct{}{}
		}
		for _, root := range c.Recursive {
			err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
				if w.ctx.Err() != nil {
					return w.ctx.Err()
				}
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						return nil
					}
					return err
				}
				if e.IsDir() {
					if real, err := filepath.EvalSymlinks(path); err == nil {
						path = real
					}
					wanted[filepath.Clean(path)] = struct{}{}
				}
				return nil
			})
			if err != nil {
				reportFailure("walk:"+root, err)
			} else {
				delete(failures, "walk:"+root)
			}
		}
		current := make(map[string]bool)
		for _, dir := range fw.WatchList() {
			current[dir] = true
			if _, ok := wanted[dir]; !ok {
				_ = fw.Remove(dir)
			}
		}
		for dir := range wanted {
			if current[dir] {
				continue
			}
			if err := fw.Add(dir); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					reportFailure(dir, err)
				}
			} else {
				delete(failures, dir)
				// Reconcile changes that raced with installation of a new watch.
				pending[dir] = struct{}{}
				arm()
			}
		}
	}
	for {
		select {
		case <-w.ctx.Done():
			return
		case config := <-w.configs:
			reconcile(config)
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod {
				continue
			}
			pending[filepath.Clean(ev.Name)] = struct{}{}
			arm()
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			pendingErr = err
			arm()
		case <-alarm:
			paths := make([]string, 0, len(pending))
			for path := range pending {
				paths = append(paths, path)
			}
			slices.Sort(paths)
			select {
			case w.events <- Batch{Paths: paths, Err: pendingErr}:
				clear(pending)
				pendingErr = nil
				first = time.Time{}
				alarm = nil
			default:
				timer.Reset(debounce)
				alarm = timer.C
			}
		}
	}
}
