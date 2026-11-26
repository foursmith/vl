package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func writePreviewPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: 220, A: 255})
		}
	}
	err = png.Encode(f, pixels)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestImageTabsPreviewReadOnlyAndLayout(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.kittySupported, m.kittyProbeDone = true, true
	path := filepath.Join(m.repo.Root, "picture.PNG")
	writePreviewPNG(t, path, 9, 5)
	m.openPreviewFile("picture.PNG")
	if m.currentImage() == nil || m.editing() {
		t.Fatal("image did not open read-only")
	}
	m.openPreviewFile("file0.go")
	if len(m.tabs) != 1 || !m.editing() {
		t.Fatal("image preview was not replaced")
	}
	m.openFile("picture.PNG")
	original := m.currentImage()
	press(m, "x")
	if m.currentImage() != original {
		t.Fatal("text input changed preview")
	}
	for _, size := range [][2]int{{120, 35}, {60, 20}, {30, 8}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.sidebar = false
		frame := ansi.Strip(m.View().Content)
		lines := strings.Split(frame, "\n")
		if len(lines) != size[1] {
			t.Fatalf("height %v: %d", size, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != size[0] {
				t.Fatalf("width %v: %d", size, ansi.StringWidth(line))
			}
		}
	}
	m.openFile("picture.PNG")
	if len(m.tabs) != 2 {
		t.Fatal("reopened image duplicated tab")
	}
	m.closeAllTabs()
	if len(m.tabs) != 0 {
		t.Fatal("image did not close")
	}
}

func TestClickImageTabKeepsPreview(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "picture.png"), 9, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m.openPreviewFile("picture.png")
	m.sidebar = true

	slot := m.tabLayout()[0]
	m.Update(tea.MouseClickMsg{X: m.mainX() + slot.x + 1, Y: tabRow, Button: tea.MouseLeft})
	if m.sidebar || m.tabs[0].preview {
		t.Fatal("click did not focus and keep the image tab")
	}

	m.openPreviewFile("file0.go")
	if len(m.tabs) != 2 || m.tabs[0].image == nil {
		t.Fatal("opening another preview replaced the kept image tab")
	}
}

func TestImageReloadOwnershipAndQuickPosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.kittySupported, m.kittyProbeDone = true, true
	path := filepath.Join(m.repo.Root, "picture.png")
	writePreviewPNG(t, path, 9, 5)
	m.openFile("picture.png")
	original := m.currentImage()
	path = original.Path
	m.imageLines()
	writePreviewPNG(t, path, 13, 5)
	reload := m.reloadWatchedFiles([]string{path})
	m.openFile("file0.go")
	drain(t, m, reload)
	if m.currentImage() != nil || m.tabs[0].image.Bounds().Dx() != 13 || m.tabs[0].image.rows != nil {
		t.Fatal("reload changed active tab or retained old pixels")
	}
	m.activateTab(0)
	stale := m.reloadWatchedFiles([]string{path})
	m.closeTab()
	drain(t, m, stale)
	if len(m.tabs) != 1 || m.currentImage() != nil {
		t.Fatal("late reload reopened image")
	}
	m.palette = []action{{id: "picture.png"}}
	actions, _ := m.quickPositionActions("picture.png:2")
	if len(actions) != 1 {
		t.Fatal("missing quick action")
	}
	actions[0].run()
	if m.currentImage() == nil || m.currentImage() == original {
		t.Fatal("quick open image failed")
	}
}

func TestImagePreviewPreservesDirtyAndFailedOpen(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.kittySupported, m.kittyProbeDone = true, true
	m.openPreviewFile("file0.go")
	doc := m.document()
	doc.Insert("// dirty")
	path := filepath.Join(m.repo.Root, "image.png")
	writePreviewPNG(t, path, 8, 5)
	m.openPreviewFile("image.png")
	if len(m.tabs) != 2 || !doc.Dirty() || m.tabs[0].preview {
		t.Fatal("image replaced dirty text preview")
	}
	im := m.currentImage()
	bad := filepath.Join(m.repo.Root, "broken.png")
	if err := os.WriteFile(bad, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	m.openPreviewFile("broken.png")
	if m.currentImage() != im || len(m.tabs) != 2 {
		t.Fatal("failed open replaced existing preview")
	}
	if err := os.Remove(im.Path); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.reloadWatchedFiles([]string{im.Path}))
	if m.currentImage() != im {
		t.Fatal("failed reload discarded visible preview")
	}
	m.closeAllTabs()
	if len(m.tabs) != 1 || m.document() != doc || !hasSaveConfirmation(m) {
		t.Fatal("close did not retain dirty save guard")
	}
}

func TestSVGFilePreviewWithoutResvgUsesTextTab(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	t.Setenv("PATH", "")
	path := filepath.Join(m.repo.Root, "icon.svg")
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="red"/></svg>`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m.openPreviewFile("icon.svg")
	if !m.editing() || m.currentImage() != nil || m.document().Text != source {
		t.Fatal("SVG did not open as source text")
	}
}
