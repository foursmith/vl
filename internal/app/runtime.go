package app

import (
	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

// Runtime owns one terminal for the entire application lifetime. Workspace
// models can be replaced while the renderer stays in the alternate screen.
type Runtime struct {
	current         *Model
	generation      uint64
	switchWorkspace func(*Model, *gitservice.Repository) *Model
}

type workspaceResult struct {
	generation uint64
	msg        tea.Msg
}

func NewRuntime(m *Model, switchWorkspace func(*Model, *gitservice.Repository) *Model) *Runtime {
	return &Runtime{current: m, generation: 1, switchWorkspace: switchWorkspace}
}

func (r *Runtime) Current() *Model { return r.current }
func (r *Runtime) View() tea.View  { return r.current.View() }
func (r *Runtime) Init() tea.Cmd {
	return workspaceCommand(r.generation, tea.Batch(r.current.Init(), r.current.prepareImageUploads()))
}

// Filter runs before Bubble Tea handles RawMsg and QuitMsg as well as before
// Update, so even terminal effects queued by a closed workspace are discarded.
func (r *Runtime) Filter(_ tea.Model, msg tea.Msg) tea.Msg {
	if result, ok := msg.(workspaceResult); ok {
		if result.generation != r.generation {
			return nil
		}
		msg = result.msg
	}
	if upload, ok := msg.(imageUploadMsg); ok {
		return r.current.imageUploadMessage(upload)
	}
	if retired, ok := msg.(imageRetirementMsg); ok {
		return r.current.imageRetirementMessage(retired)
	}
	return msg
}

// Model commands use Batch to compose effects. Wrap every leaf, including
// nested batches, without retaining the workspace model in the envelope.
func workspaceCommand(generation uint64, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			commands := make(tea.BatchMsg, len(batch))
			for i, child := range batch {
				commands[i] = workspaceCommand(generation, child)
			}
			msg = commands
		}
		return workspaceResult{generation: generation, msg: msg}
	}
}

func (r *Runtime) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	old := r.current
	_, cmd := old.Update(msg)
	if next := old.nextRepository; next != nil && r.switchWorkspace != nil {
		updated := r.switchWorkspace(old, next)
		updated.workbenchKeymap = old.workbenchKeymap
		// These describe the terminal, whose lifetime spans all workspaces.
		updated.width, updated.height = old.width, old.height
		updated.monochrome = old.monochrome
		updated.terminalBlurred = old.terminalBlurred
		updated.cellWidth, updated.cellHeight = old.cellWidth, old.cellHeight
		updated.kittySupported, updated.kittyProbeDone = old.kittySupported, old.kittyProbeDone
		updated.themeNotifications, updated.themeNotificationsOwned = old.themeNotifications, old.themeNotificationsOwned
		updated.appearance = old.appearance
		updated.resize()
		cleanup := old.ImageCleanupSequence()
		old.Close()
		r.current = updated
		r.generation++
		// Retire the old images before the new workspace can upload any.
		return r, tea.Sequence(tea.Raw(cleanup), r.Init())
	}
	return r, workspaceCommand(r.generation, tea.Batch(cmd, r.current.prepareImageUploads()))
}
