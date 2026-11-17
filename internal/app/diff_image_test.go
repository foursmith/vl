package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func readImageDiff(m *Model, c gitservice.Change, staged bool, commit string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if commit != "" {
			return m.repo.CommitFileDiff(ctx, commit, c)
		}
		return m.repo.Diff(ctx, c, staged)
	}
}

func imageGit(t *testing.T, m *Model, args ...string) string {
	t.Helper()
	out, err := m.repo.Run(m.ctx, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestDiffImagesRenameDeleteAndReplyOwnership(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	writePreviewPNG(t, filepath.Join(m.repo.Root, "old.png"), 8, 5)
	imageGit(t, m, "add", ".")
	imageGit(t, m, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "image")
	imageGit(t, m, "mv", "old.png", "new.png")
	c := gitservice.Change{Path: "new.png", Original: "old.png", Index: 'R', Worktree: ' '}
	cmd := m.requestDiff(diffTarget{path: c.Path, staged: true, change: &c}, readImageDiff(m, c, true, ""))
	d := m.diffState
	msg := cmd().(diffMsg)
	m.openFile("new.png")
	m.Update(msg)
	if m.currentImage() == nil || d.images == nil || d.images.sides[0] == nil || d.images.sides[1] == nil {
		t.Fatal("renamed image sides were missing or reply stole focus")
	}
	m.activateTab(0)
	stale := m.loadDiff(d, false)().(diffMsg)
	m.closeTab()
	ids := len(m.imageIDs)
	m.Update(stale)
	if len(m.imageIDs) != ids || d.images != nil {
		t.Fatal("closed-tab reply allocated images")
	}
	// The unstaged side of an index rename reads the new index path.
	writePreviewPNG(t, filepath.Join(m.repo.Root, "new.png"), 12, 5)
	c.Worktree = 'M'
	drain(t, m, m.requestDiff(diffTarget{path: c.Path, change: &c}, readImageDiff(m, c, false, "")))
	if m.images.sides[0] == nil || m.images.sides[0].Bounds().Dx() != 8 || m.images.sides[1].Bounds().Dx() != 12 {
		t.Fatal("unstaged rename compared the wrong index path")
	}
	if err := os.Remove(filepath.Join(m.repo.Root, "new.png")); err != nil {
		t.Fatal(err)
	}
	c.Worktree = 'D'
	drain(t, m, m.requestDiff(diffTarget{path: c.Path, change: &c}, readImageDiff(m, c, false, "")))
	if m.images.sides[0] == nil || m.images.sides[1] != nil || m.images.errors[1] != nil {
		t.Fatal("deleted image did not leave an absent right side")
	}
}

func TestDiffImagesReadActualReviewVersions(t *testing.T) {
	for _, ext := range []string{"png", "svg"} {
		t.Run(ext, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			defer m.Close()
			m.kittySupported = true
			path := "image." + ext
			write := func(width int) {
				if ext == "png" {
					writePreviewPNG(t, filepath.Join(m.repo.Root, path), width, 5)
				} else if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="5"><rect width="100%%" height="5" fill="red"/></svg>`, width)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			commit := func() string {
				imageGit(t, m, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "image")
				return imageGit(t, m, "rev-parse", "HEAD")
			}
			write(8)
			imageGit(t, m, "add", "--", path)
			root := commit()
			write(12)
			imageGit(t, m, "add", "--", path)
			head := commit()
			write(20)
			imageGit(t, m, "add", "--", path)
			write(28)
			c := gitservice.Change{Path: path, Index: 'M', Worktree: 'M'}
			for _, tc := range []struct {
				commit string
				staged bool
				widths [2]int
			}{{"", false, [2]int{20, 28}}, {"", true, [2]int{12, 20}}, {head, false, [2]int{8, 12}}, {root, false, [2]int{0, 8}}} {
				target := diffTarget{path: path, staged: tc.staged, commit: tc.commit, change: &c}
				drain(t, m, m.requestDiff(target, readImageDiff(m, c, tc.staged, tc.commit)))
				if m.images == nil {
					t.Fatal("image diff was not loaded")
				}
				for side, width := range tc.widths {
					im := m.images.sides[side]
					if err := m.images.errors[side]; err != nil {
						t.Fatal(err)
					}
					if width == 0 {
						if im != nil {
							t.Fatal("initial commit has a parent image")
						}
					} else if im == nil || im.Bounds().Dx() != width {
						t.Fatalf("%+v side %d: wrong version", tc, side)
					}
				}
				if !strings.Contains(m.View().Content, "\x1b_G") {
					t.Fatal("image protocol did not reach the view")
				}
			}
			for _, size := range [][2]int{{120, 35}, {70, 20}, {30, 8}} {
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m.sidebar = false
				for _, line := range strings.Split(m.View().Content, "\n") {
					if ansi.StringWidth(line) != size[0] {
						t.Fatal("image diff exceeded frame width")
					}
				}
			}
			m.Update(uv.CellSizeEvent{Width: 10, Height: 20})
			if m.images.sides[1].rows != nil {
				t.Fatal("cell size change retained stale placements")
			}
			m.kittySupported, m.kittyProbeDone = false, true
			m.Update(tea.WindowSizeMsg{Width: 160, Height: 35})
			if view := m.View().Content; strings.Contains(view, "\x1b_G") || !strings.Contains(view, "No response") {
				t.Fatal("unsupported terminal did not degrade gracefully")
			}
			m.closeAllTabs()
			if len(m.imageRetired) == 0 {
				t.Fatal("closed image diffs leaked terminal images")
			}
		})
	}
}
