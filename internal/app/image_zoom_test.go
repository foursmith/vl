package app

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func zoomWorkspace(t *testing.T) *Model {
	t.Helper()
	m := tabWorkspace(t, 1)
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 1200, 800)
	m.openFile("image.png")
	m.imageRegion()
	return m
}

func TestImageWheelZoomAnchorsPointerAndPreservesTabs(t *testing.T) {
	m := zoomWorkspace(t)
	x, y := m.mainX()+m.mainWidth()/2+3, editorContentRow+m.contentHeight()/2
	before := m.imageRegion()
	px, py := m.imagePointer(before, x, y)
	anchorX, anchorY := before.X+px/before.Scale, before.Y+py/before.Scale
	m.imageLines()
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
	after := m.imageRegion()
	px, py = m.imagePointer(after, x, y)
	if math.Abs(after.X+px/after.Scale-anchorX) > .0001 || math.Abs(after.Y+py/after.Scale-anchorY) > .0001 {
		t.Fatal("zoom moved the source point under the mouse")
	}
	if after.Scale <= before.Scale || m.tabs[0].image.rows != nil {
		t.Fatal("wheel did not invalidate and enlarge image")
	}
	zoom := m.tabs[0].image.zoom
	m.openFile("file0.go")
	m.activateTab(0)
	if m.tabs[0].image.zoom != zoom {
		t.Fatal("tab switch lost zoom")
	}
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	if math.Abs(m.tabs[0].image.zoom-1) > .0001 {
		t.Fatal("wheel down did not reverse zoom")
	}
}

func TestImageZoomRoutingLimitsPanAndReset(t *testing.T) {
	m := zoomWorkspace(t)
	x, y := m.mainX()+m.mainWidth()/2, editorContentRow+m.contentHeight()/2
	m.Update(tea.MouseWheelMsg{X: m.sideX() + 1, Y: y, Button: tea.MouseWheelUp})
	if m.tabs[0].image.zoom != 1 {
		t.Fatal("sidebar wheel zoomed image")
	}
	m.modal = "find"
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
	if m.tabs[0].image.zoom != 1 {
		t.Fatal("modal wheel zoomed image")
	}
	m.modal = ""
	m.zoomImage(1e9, x, y)
	if m.tabs[0].image.zoom != 32 {
		t.Fatal("missing upper zoom bound")
	}
	before := m.imageRegion()
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x - 3, Y: y - 2, Button: tea.MouseLeft})
	after := m.imageRegion()
	if after.X <= before.X || after.Y <= before.Y {
		t.Fatal("drag did not pan the zoomed image")
	}
	m.Update(tea.MouseReleaseMsg{X: x - 3, Y: y - 2, Button: tea.MouseLeft})
	if m.imagePan != nil {
		t.Fatal("release retained drag")
	}
	m.beginImagePan(x, y)
	m.dragImage(-100000, -100000)
	edge := m.imageRegion()
	if edge.X+edge.Width > 1200.0001 || edge.Y+edge.Height > 800.0001 {
		t.Fatal("pan escaped image bounds")
	}
	m.Update(tea.BlurMsg{})
	if m.imagePan != nil {
		t.Fatal("blur retained drag")
	}
	press(m, "0")
	if m.tabs[0].image.zoom != 1 || m.tabs[0].image.centerX != .5 || m.tabs[0].image.centerY != .5 {
		t.Fatal("0 did not restore fit")
	}
	m.zoomImage(1e-9, x, y)
	if m.tabs[0].image.zoom != .1 {
		t.Fatal("missing lower zoom bound")
	}
}

func TestImageZoomFrameBounds(t *testing.T) {
	m := zoomWorkspace(t)
	for _, size := range [][2]int{{120, 35}, {60, 20}, {30, 8}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.sidebar = false
		m.zoomImage(32, m.mainX()+m.mainWidth()/2, editorContentRow+m.contentHeight()/2)
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != size[1] {
			t.Fatal("zoom changed frame height")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != size[0] {
				t.Fatal("zoom escaped frame width")
			}
		}
	}
}
