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
	if len(m.tabs) != 1 || m.document() != doc || m.modal != "close" {
		t.Fatal("close did not retain dirty save guard")
	}
}

func TestImageTabPixelBudgetReloadsEvictedTab(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.kittySupported = true
	source, err := os.ReadFile(filepath.Join("..", "..", "assets", "screenshots.webp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.webp", "two.webp", "three.webp"} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, name), source, 0600); err != nil {
			t.Fatal(err)
		}
		m.openFile(name)
		m.zoomImage(2, m.mainX()+10, m.contentRow()+5)
		imageUploadOutput(t, m)
	}
	first := m.tabs[0].image
	if len(m.tabs) != 3 || first.DecodedBytes() != 0 || first.Bounds().Dx() != 3200 || first.id != 0 {
		t.Fatal("least recently used image did not release its pixels and placement")
	}
	if m.tabs[1].image.DecodedBytes() == 0 || m.tabs[2].image.DecodedBytes() == 0 {
		t.Fatal("image budget discarded more tabs than necessary")
	}
	m.activateTab(0)
	if first.zoom != 2 || !strings.Contains(strings.Join(m.imageLines(), ""), "Loading image") {
		t.Fatal("evicted tab lost its zoom or loading state")
	}
	if output := imageUploadOutput(t, m); output == "" || first.DecodedBytes() == 0 || m.tabs[1].image.DecodedBytes() != 0 {
		t.Fatal("reselected tab did not restore its pixels within the budget")
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
