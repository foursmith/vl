package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/imagepreview"
)

type diffImages struct {
	sides  [2]*imageState
	errors [2]error
}

// Capture the source before starting asynchronous work; sidebar and tab changes
// must never change which revision a pending image belongs to.
func (m *Model) diffImageReader(d *diffState) func(context.Context) *diffImages {
	if !imagepreview.Supported(d.diffPath) {
		return nil
	}
	workspace := m.repo
	c := gitservice.Change{Path: d.diffPath, Original: d.diffOriginal, Repository: d.diffRepository}
	if d.diffChange != nil {
		c = *d.diffChange
	}
	commit, staged := d.diffCommit, d.diffStaged
	return func(ctx context.Context) *diffImages {
		result := &diffImages{}
		oldRev, newRev := "", ""
		if commit != "" {
			oldRev, newRev = commit+"^", commit
		} else if staged {
			oldRev = "HEAD"
		}
		repo, local, err := workspace.ForChange(ctx, c)
		if err != nil {
			result.errors = [2]error{err, err}
			return result
		}
		oldPath := local.Path
		if local.Original != "" && (commit != "" || staged || local.Worktree == 'R' || local.Worktree == 'C') {
			oldPath = local.Original
		}
		for side, path := range []string{oldPath, local.Path} {
			var im *imagepreview.Image
			var err error
			if side == 1 && commit == "" && !staged {
				im, err = imagepreview.Open(filepath.Join(repo.Root, filepath.FromSlash(path)))
				if os.IsNotExist(err) {
					err = nil
				}
			} else {
				rev := oldRev
				if side == 1 {
					rev = newRev
				}
				var data []byte
				data, err = repo.Blob(ctx, rev+":"+path, imagepreview.MaxBytes)
				if err == nil && data != nil {
					im, err = imagepreview.Decode(path, data)
				}
			}
			result.errors[side] = err
			if im != nil {
				result.sides[side] = newImageState(im, 0)
			}
		}
		return result
	}
}

// Retain unchanged placements across refreshes and retire only replaced sides.
func (m *Model) setDiffImages(d *diffState, images *diffImages) {
	for side := range 2 {
		var old, next *imageState
		if d.images != nil {
			old = d.images.sides[side]
		}
		if images != nil {
			next = images.sides[side]
		}
		if old != nil && next != nil && next.SameContent(old.Image) {
			images.sides[side] = old
			continue
		}
		if old != nil {
			m.imageRetired = append(m.imageRetired, old.id)
		}
		if next != nil && next.id == 0 {
			next.id = m.allocateImageID()
		}
	}
	d.images = images
}

func (m *Model) diffImageLines() []string {
	w, h := m.mainWidth(), m.contentHeight()
	if w < 3 || h < 2 {
		return nil
	}
	widths := [2]int{(w - 1) / 2, w - 1 - (w-1)/2}
	labels := [2]string{"Index", "Working tree"}
	if m.diffCommit != "" {
		labels = [2]string{"Parent", "Commit"}
	} else if m.diffStaged {
		labels = [2]string{"HEAD", "Index"}
	}
	var columns [2][]string
	for side, width := range widths {
		columns[side] = make([]string, h)
		im := m.images.sides[side]
		label := labels[side]
		if im != nil {
			b := im.Bounds()
			label += fmt.Sprintf(" · %d × %d", b.Dx(), b.Dy())
			copy(columns[side][1:], m.imagePreviewLines(im, width, h-1))
		} else {
			text := "不存在"
			if err := m.images.errors[side]; err != nil {
				text = safe(err.Error())
			}
			columns[side][1+(h-1)/2] = "  " + text
		}
		columns[side][0] = "  " + label
	}
	lines := make([]string, h)
	separator := row("│", 1, m.colors.Border, m.colors.Background)
	for y := range lines {
		lines[y] = row(columns[0][y], widths[0], m.colors.Foreground, m.colors.Background) + separator + row(columns[1][y], widths[1], m.colors.Foreground, m.colors.Background)
	}
	return lines
}
