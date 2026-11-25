package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"weak"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	gitservice "github.com/foursmith/vl/internal/git"
)

type runtimeCheckpoint struct{ root string }

func TestRuntimeReleasesWorkspaceDocuments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	roots := []string{t.TempDir(), t.TempDir()}
	for _, root := range roots {
		if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("workspace contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	open := func(repo *gitservice.Repository) *Model {
		m := New(t.Context(), repo, false)
		if !m.openDocument("file.txt", false) {
			t.Fatal("cannot open workspace document")
		}
		return m
	}
	var documents []weak.Pointer[editor.Document]
	r := NewRuntime(open(&gitservice.Repository{Root: roots[0]}), func(old *Model, repo *gitservice.Repository) *Model {
		documents = append(documents, weak.Make(old.document()))
		return open(repo)
	})
	defer func() { r.Current().Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ready := make(chan struct{})
	p := tea.NewProgram(r, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(),
		tea.WithFilter(func(model tea.Model, msg tea.Msg) tea.Msg {
			if _, ok := msg.(runtimeCheckpoint); ok {
				close(ready)
				return nil
			}
			return r.Filter(model, msg)
		}))
	checked := make(chan bool, 1)
	go func() {
		for i := range 10 {
			p.Send(repositoryOpenedMsg{repo: &gitservice.Repository{Root: roots[(i+1)%2]}})
		}
		p.Send(runtimeCheckpoint{})
		select {
		case <-ready:
		case <-ctx.Done():
			checked <- false
			return
		}
		// Keep the program alive while checking that canceled workspace commands
		// release their documents; exiting the program must not be required.
		deadline := time.Now().Add(time.Second)
		for {
			runtime.GC()
			retained := false
			for _, doc := range documents {
				if doc.Value() != nil {
					retained = true
				}
			}
			if !retained || time.Now().After(deadline) {
				checked <- !retained
				p.Send(tea.QuitMsg{})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if !<-checked {
		t.Fatal("closed workspace documents retained by background commands")
	}
}

func TestRuntimeSwitchKeepsAlternateScreen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	first, second := t.TempDir(), t.TempDir()
	m := New(t.Context(), &gitservice.Repository{Root: first}, false)
	m.workbenchKeymap = testKeybindings(t, []byte(`[{"command": "workbench.openCommandPalette", "keys": "F8"}]`))
	m.themeNotifications, m.themeNotificationsOwned = true, true
	m.Update(tea.WindowSizeMsg{Width: 92, Height: 28})
	r := NewRuntime(m, func(old *Model, repo *gitservice.Repository) *Model {
		updated := New(t.Context(), repo, false)
		updated.SetPreferences(old.Preferences())
		return updated
	})
	defer func() { r.Current().Close() }()
	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p := tea.NewProgram(r, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(&output), tea.WithWindowSize(92, 28),
		tea.WithFilter(func(model tea.Model, msg tea.Msg) tea.Msg {
			if check, ok := msg.(runtimeCheckpoint); ok {
				current := r.Current()
				if current.repo.Root != check.root || current.width != 92 || current.height != 28 {
					t.Errorf("wrong workspace or terminal size after switch: %s, %dx%d", current.repo.Root, current.width, current.height)
				}
				if !r.View().AltScreen || current.ThemeNotificationRestoreSequence() != ansi.ResetModeLightDark {
					t.Error("switch lost alternate screen or terminal subscription ownership")
				}
				if current.commandShortcutHint(commandPalette) != "F8" || current.resolveWorkbenchShortcut("f8").command != commandPalette {
					t.Error("switch lost the active shortcut configuration")
				}
				return nil
			}
			return r.Filter(model, msg)
		}))
	go func() {
		p.Send(repositoryOpenedMsg{repo: &gitservice.Repository{Root: second}})
		p.Send(runtimeCheckpoint{second})
		p.Send(repositoryOpenedMsg{repo: &gitservice.Repository{Root: first}})
		p.Send(runtimeCheckpoint{first})
		p.Send(tea.QuitMsg{})
	}()
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if m.ctx.Err() == nil {
		t.Fatal("old workspace background tasks were not canceled")
	}
	if r.generation != 3 {
		t.Fatalf("expected two switches, generation = %d", r.generation)
	}
	for _, sequence := range []string{"\x1b[?1049h", "\x1b[?1049l"} {
		if count := strings.Count(output.String(), sequence); count != 1 {
			t.Fatalf("alternate screen sequence %q occurred %d times; want once for the entire run", sequence, count)
		}
	}
}

func TestRuntimeDiscardsOldWorkspaceCommands(t *testing.T) {
	m := New(t.Context(), &gitservice.Repository{Root: t.TempDir()}, false)
	r := NewRuntime(m, nil)
	defer m.Close()
	// Include nested batches and terminal effects, which Bubble Tea handles
	// before Update. They must follow the same ownership rules as snapshots.
	cmd := workspaceCommand(r.generation, tea.Batch(
		func() tea.Msg { return snapshotMsg{seq: 1} },
		tea.Batch(tea.Raw("old image upload"), tea.Quit),
	))
	var results []tea.Msg
	var collect func(tea.Cmd)
	collect = func(command tea.Cmd) {
		result := command()
		msg := r.Filter(r, result)
		if batch, ok := msg.(tea.BatchMsg); ok {
			results = append(results, result)
			for _, child := range batch {
				collect(child)
			}
		} else {
			if msg == nil {
				t.Fatal("current workspace command was dropped")
			}
			results = append(results, result)
		}
	}
	collect(cmd)
	r.generation++
	for _, result := range results {
		if msg := r.Filter(r, result); msg != nil {
			t.Fatalf("stale workspace command survived switch: %T", msg)
		}
	}
	if msg := r.Filter(r, tea.WindowSizeMsg{Width: 80, Height: 24}); msg == nil {
		t.Fatal("terminal events should still reach the new workspace")
	}
}
