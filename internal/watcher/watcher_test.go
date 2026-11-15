package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func waitPath(t *testing.T, w *Watcher, path string) Batch {
	t.Helper()
	for {
		batch, ok := w.Next()
		if !ok {
			t.Fatalf("watcher closed before %s", path)
		}
		if batch.Err != nil {
			t.Fatal(batch.Err)
		}
		if slices.Contains(batch.Paths, path) {
			return batch
		}
	}
}
func newTestWatcher(t *testing.T) (*Watcher, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	w, err := New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w.Sync(Config{Directories: []string{root}})
	waitPath(t, w, root)
	return w, root
}
func TestParentWatchSurvivesAtomicReplacement(t *testing.T) {
	w, root := newTestWatcher(t)
	target := filepath.Join(root, "file.txt")
	for i := 0; i < 2; i++ {
		temp := filepath.Join(root, fmt.Sprintf(".elegit-save-%d", i))
		if err := os.WriteFile(temp, []byte("updated"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(temp, target); err != nil {
			t.Fatal(err)
		}
		waitPath(t, w, target)
	}
	dir := filepath.Join(root, "folder")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	waitPath(t, w, dir)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	waitPath(t, w, target)
	hidden := filepath.Join(root, ".elegit-notes")
	if err := os.WriteFile(hidden, []byte("user file"), 0644); err != nil {
		t.Fatal(err)
	}
	waitPath(t, w, hidden)
}
func TestDebounceHasMaximumWaitDuringContinuousWrites(t *testing.T) {
	w, root := newTestWatcher(t)
	target := filepath.Join(root, "stream.txt")
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i++ {
			if err := os.WriteFile(target, []byte(fmt.Sprint(i)), 0644); err != nil {
				done <- err
				return
			}
			select {
			case <-stop:
				done <- nil
				return
			case <-ticker.C:
			}
		}
	}()
	start := time.Now()
	waitPath(t, w, target)
	elapsed := time.Since(start)
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("continuous writes delayed notification for %s", elapsed)
	}
}
func TestCancelUnblocksReceiverAndCloseIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w, err := New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, ok := w.Next(); ok {
		t.Fatal("receiver stayed open")
	}
	w.Close()
	w.Close()
}

func TestRecursiveWatchesIncludeNestedReferences(t *testing.T) {
	w, root := newTestWatcher(t)
	refs := filepath.Join(root, "refs")
	team := filepath.Join(refs, "heads", "team")
	if err := os.MkdirAll(team, 0755); err != nil {
		t.Fatal(err)
	}
	w.Sync(Config{Directories: []string{root}, Recursive: []string{refs}})
	waitPath(t, w, team)
	ref := filepath.Join(team, "feature")
	if err := os.WriteFile(ref, []byte("head"), 0644); err != nil {
		t.Fatal(err)
	}
	waitPath(t, w, ref)
}
