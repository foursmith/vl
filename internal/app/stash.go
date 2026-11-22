package app

import (
	"context"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

type stashPickerMsg struct {
	seq     int
	target  string
	stash   gitservice.Stash
	stashes []gitservice.Stash
	files   []gitservice.StashFile
	err     error
}

func (m *Model) stash(target string) tea.Cmd {
	return m.stashWithOptions(target, "Stash changes", "--include-untracked")
}

func (m *Model) stashWithOptions(target, title, option string) tea.Cmd {
	m.stashRepository, m.stashOption = target, option
	return m.prompt("stash", m.tr(title), "")
}

func (m *Model) openStashModes(target string) tea.Cmd {
	m.palette = nil
	for _, mode := range []struct{ title, option string }{
		{"Stash including untracked files", "--include-untracked"},
		{"Stash tracked changes", ""},
		{"Stash staged changes only", "--staged"},
		{"Stash changes and keep staged changes", "--keep-index"},
	} {
		m.palette = append(m.palette, action{label: m.tr(mode.title), run: func() tea.Cmd {
			return m.stashWithOptions(target, mode.title, mode.option)
		}})
	}
	return m.prompt("stashPicker", m.tr("New Stash")+" · "+m.repositoryName(target), "")
}

func (m *Model) loadStashPicker(target string, stash gitservice.Stash) tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation")
		return nil
	}
	m.stashPickerSeq++
	seq := m.stashPickerSeq
	ctx, workspace := m.ctx, m.repo
	m.palette = []action{{label: m.tr("Loading stash…")}}
	focus := m.prompt("stashLoading", m.tr("Stashes")+" · "+m.repositoryName(target), "")
	load := func() tea.Msg {
		msg := stashPickerMsg{seq: seq, target: target, stash: stash}
		repo, err := workspace.RepositoryAt(ctx, target)
		if err != nil {
			msg.err = err
			return msg
		}
		if stash.ID == "" {
			msg.stashes, msg.err = repo.Stashes(ctx)
		} else {
			msg.files, msg.err = repo.StashFiles(ctx, stash.ID)
		}
		return msg
	}
	return tea.Batch(focus, load)
}

func (m *Model) openStashes(target string) tea.Cmd {
	return m.loadStashPicker(target, gitservice.Stash{})
}

func (m *Model) finishStashPicker(msg stashPickerMsg) tea.Cmd {
	if m.modal != "stashLoading" || msg.seq != m.stashPickerSeq {
		return nil
	}
	if msg.err != nil {
		m.dismissDialog()
		m.notice = m.tr(msg.err.Error())
		return nil
	}
	target := msg.target
	m.palette = nil
	repoName := m.repositoryName(target)
	if msg.stash.ID == "" {
		for _, stash := range msg.stashes {
			m.palette = append(m.palette, action{label: stash.Ref() + "  " + stash.Subject, key: shortOID(stash.ID),
				run: func() tea.Cmd { return m.loadStashPicker(target, stash) }})
		}
		if len(msg.stashes) == 0 {
			m.palette = append(m.palette, action{label: m.tr("No stashes in this repository")})
		}
		m.palette = append(m.palette, action{label: m.tr("New Stash…"), run: func() tea.Cmd { return m.openStashModes(target) }})
		return m.prompt("stashPicker", m.tr("Stashes")+" · "+repoName, "")
	}
	for _, file := range msg.files {
		detail := string(file.Index)
		if file.Untracked {
			detail = m.tr("Untracked")
		}
		label := file.Path
		if file.Original != "" {
			label = file.Original + " → " + file.Path
		}
		m.palette = append(m.palette, action{label: label, key: detail, run: func() tea.Cmd {
			c := file.Change
			c.Repository = target
			c.Path = filepath.ToSlash(filepath.Join(target, c.Path))
			if c.Original != "" {
				c.Original = filepath.ToSlash(filepath.Join(target, c.Original))
			}
			return m.requestDiff(diffTarget{path: c.Path, repository: target, commit: file.Commit, change: &c, preview: true},
				func(ctx context.Context) (string, error) { return m.repo.CommitFileDiff(ctx, file.Commit, c) })
		}})
	}
	if len(msg.files) == 0 {
		m.palette = append(m.palette, action{label: m.tr("No changed files")})
	}
	m.palette = append(m.palette, action{label: m.tr("Stash Actions…"), run: func() tea.Cmd {
		return m.openStashActions(target, msg.stash)
	}})
	m.palette = append(m.palette, action{label: m.tr("Back to Stashes"), run: func() tea.Cmd { return m.openStashes(target) }})
	return m.prompt("stashPicker", msg.stash.Ref()+" · "+repoName+" · "+msg.stash.Subject, "")
}

func (m *Model) openStashActions(target string, stash gitservice.Stash) tea.Cmd {
	m.palette = nil
	for _, a := range []struct {
		label, operation string
		index            bool
	}{
		{"Apply and Keep Stash", "apply", false},
		{"Apply and Restore Staged Changes", "apply", true},
		{"Pop Stash", "pop", false},
		{"Pop and Restore Staged Changes", "pop", true},
		{"Drop Stash…", "drop", false},
	} {
		m.palette = append(m.palette, action{label: m.tr(a.label), run: func() tea.Cmd {
			return m.useStash(target, stash, a.operation, a.index)
		}})
	}
	m.palette = append(m.palette, action{label: m.tr("Back to Stashed Files"), run: func() tea.Cmd {
		return m.loadStashPicker(target, stash)
	}})
	return m.prompt("stashPicker", stash.Ref()+" · "+m.repositoryName(target)+" · "+stash.Subject, "")
}

func (m *Model) useStash(target string, stash gitservice.Stash, action string, index bool) tea.Cmd {
	title, label, question := "Stash apply", "Apply and Keep Stash", ""
	switch action {
	case "pop":
		title, label, question = "Stash pop", "Pop Stash", "Apply and remove %s in %s?"
		if index {
			question = "Apply %s in %s, restore staged changes, and remove the stash?"
		}
	case "drop":
		title, label, question = "Stash drop", "Drop Stash", "Delete %s in %s?"
	}
	run := func() tea.Cmd {
		if action != "drop" && m.hasDirtyTabsForCheckout(target) {
			m.notice = m.tr("Save or close edited files before this operation")
			return nil
		}
		return m.operation(title, func() (string, error) {
			repo, err := m.repo.RepositoryAt(m.ctx, target)
			if err != nil {
				return "", err
			}
			return repo.UseStash(m.ctx, stash, action, index)
		})
	}
	if question != "" {
		m.openConfirmation(m.tr(question, stash.Ref(), m.repositoryName(target)), stash.Subject, func() tea.Cmd {
			return m.openStashActions(target, stash)
		}, confirmationAction{label: m.tr(label), run: func() tea.Cmd {
			m.dismissDialog()
			return run()
		}})
		if action == "drop" {
			m.confirmationButton = 1 // Require moving focus to the destructive action.
		}
		return nil
	}
	return run()
}
