package app

import (
	"context"
	"io"
	"os"
	"runtime"
	"testing"
	"time"
	"weak"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/editor"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/watcher"
)

// Exercise real Bubble Tea command ownership: a sleeping command retains the
// program, which in turn retains its initial model even after Run returns.
type switchLifetimeModel struct{ *Model }
type switchLifetimeReadyMsg struct{}

func (m switchLifetimeModel) Init() tea.Cmd {
	return tea.Batch(m.tick(), m.probeImages(), func() tea.Msg { return switchLifetimeReadyMsg{} })
}

func closedWorkspaceDocument(t *testing.T) weak.Pointer[editor.Document] {
	t.Helper()
	root := t.TempDir()
	m := New(context.Background(), &gitservice.Repository{Root: root}, false)
	defer m.Close()
	var err error
	m.watcher, err = watcher.New(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	path := root + "/file.txt"
	if err := os.WriteFile(path, []byte("old workspace contents"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := editor.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m.tabs = []tab{{doc: doc}}
	m.activateTab(0)
	reference := weak.Make(doc)
	target := &gitservice.Repository{Root: root + "/next"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p := tea.NewProgram(switchLifetimeModel{m}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithWindowSize(120, 36),
		tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if _, ok := msg.(switchLifetimeReadyMsg); ok {
				return repositoryOpenedMsg{repo: target}
			}
			return msg
		}))
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if m.NextRepository() != target {
		t.Fatal("workspace did not switch")
	}
	return reference
}

func TestWorkspaceSwitchReleasesDocuments(t *testing.T) {
	for range 3 {
		doc := closedWorkspaceDocument(t)
		deadline := time.Now().Add(time.Second)
		for {
			runtime.GC()
			if doc.Value() == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("closed workspace document retained by a background command")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestDelayedMessageCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := delayedMessage(ctx, time.Hour, func(time.Time) tea.Msg {
		t.Error("canceled delay invoked its callback")
		return nil
	})
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	cancel()
	select {
	case msg := <-done:
		if msg != nil {
			t.Fatalf("canceled delay returned %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled delay kept waiting")
	}
	if got := delayedMessage(context.Background(), 0, func(time.Time) tea.Msg { return "ready" })(); got != "ready" {
		t.Fatalf("active delay returned %v", got)
	}
}
