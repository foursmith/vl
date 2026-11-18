package config

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// Session stores the open editor tabs and selected sidebar tab for a workspace.
type Session struct {
	Tabs       []SessionTab `json:"tabs"`
	Active     int          `json:"active"`
	SidebarTab string       `json:"sidebarTab,omitempty"`
}

type SessionTab struct {
	Kind        string              `json:"kind"`
	Path        string              `json:"path"`
	Preview     bool                `json:"preview,omitempty"`
	Repository  string              `json:"repository,omitempty"`
	Commit      string              `json:"commit,omitempty"`
	Staged      bool                `json:"staged,omitempty"`
	Original    string              `json:"original,omitempty"`
	PullRequest *SessionPullRequest `json:"pullRequest,omitempty"`
}

type SessionPullRequest struct {
	Number  int    `json:"number"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

func SessionPath(root string) (string, error) {
	dir, err := statePath("workspaces")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%x.json", sha256.Sum256([]byte(filepath.Clean(root))))), nil
}

func LoadSession(path string) (Session, error) {
	var session Session
	err := loadJSON(path, &session)
	return session, err
}

func SaveSession(path string, session Session) error { return saveJSON(path, session) }
