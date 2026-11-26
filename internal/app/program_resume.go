package app

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func (r *Runtime) SetResumeToken(token string) { r.resumeToken = token }

func (r *Runtime) reportResume() tea.Cmd {
	if r.resumeToken == "" {
		return nil
	}
	root := r.current.repo.Root
	if validResumePath(root) {
		fields := "argv=" + base64.StdEncoding.EncodeToString([]byte("vl")) + "," +
			base64.StdEncoding.EncodeToString([]byte(root))
		if cwd, err := os.Getwd(); err == nil && validResumePath(cwd) {
			fields += ":cwd=" + base64.StdEncoding.EncodeToString([]byte(cwd))
		}
		if sequence := r.resumeSequence(fields); len(sequence) <= 2048 {
			return tea.Raw(sequence)
		}
	}
	// An unreportable workspace must not leave the previous one registered.
	return tea.Raw(r.resumeSequence("clear=1"))
}

func (r *Runtime) ResumeClearSequence() string {
	if r.resumeToken == "" || !r.current.quitRequested {
		return ""
	}
	return r.resumeSequence("clear=1")
}

func (r *Runtime) resumeSequence(fields string) string {
	body := fmt.Sprintf("pid=%d:time=%d:%s", os.Getpid(), time.Now().Unix(), fields)
	mac := sha256.Sum256([]byte(body + ":" + r.resumeToken))
	return fmt.Sprintf("\x1b]7577;%s:mac=%x\x1b\\", body, mac)
}

func validResumePath(path string) bool {
	return filepath.IsAbs(path) && utf8.ValidString(path) && strings.IndexFunc(path, unicode.IsControl) == -1
}
