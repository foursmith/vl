package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

var invalidGitHubNameCharacter = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

type githubPublishPicker struct {
	target, branch, owner string
}

type githubPublishOwnerMsg struct {
	picker *githubPublishPicker
	owner  string
	err    error
}

func (m *Model) openGitHubPublish(target, branch string) tea.Cmd {
	picker := &githubPublishPicker{target: target, branch: branch}
	m.githubPublish = picker
	m.palette = nil
	name := invalidGitHubNameCharacter.ReplaceAllString(filepath.Base(m.repositoryName(target)), "-")
	m.prompt("githubPublish", m.tr("GitHub repository name"), name)
	repo, ctx := m.repo, m.ctx
	return func() tea.Msg {
		owner, err := repo.GitHubPublishOwner(ctx)
		return githubPublishOwnerMsg{picker, owner, err}
	}
}

func (m *Model) finishGitHubPublishOwner(msg githubPublishOwnerMsg) {
	if m.modal != "githubPublish" || m.githubPublish != msg.picker {
		return
	}
	if msg.err != nil {
		m.dismissDialog()
		if errors.Is(msg.err, exec.ErrNotFound) {
			m.notice = m.tr("Install GitHub CLI (gh)")
		} else {
			m.notice = m.tr("Could not read GitHub account: %s", msg.err.Error())
		}
		return
	}
	msg.picker.owner = msg.owner
	m.pickerFilter = pickerFilterCache{}
	m.syncPicker()
}

func (m *Model) githubPublishActions() []action {
	picker := m.githubPublish
	if picker.owner == "" {
		return []action{{label: m.tr("Checking GitHub account…")}}
	}
	name := strings.TrimSpace(m.input.Value())
	if !gitservice.ValidGitHubRepositoryName(name) {
		return []action{{label: m.tr("Enter a valid GitHub repository name")}}
	}
	fullName := picker.owner + "/" + name
	publish := func(private bool) func() tea.Cmd {
		return func() tea.Cmd {
			return m.publishInRepository(picker.target, func(repo *gitservice.Repository) (string, error) {
				return repo.PublishToGitHub(m.ctx, fullName, picker.branch, private)
			})
		}
	}
	return []action{
		{label: m.tr("Publish private · %s", fullName), run: publish(true)},
		{label: m.tr("Publish public · %s", fullName), run: publish(false)},
	}
}
