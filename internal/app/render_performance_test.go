package app

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/imagepreview"
)

func performanceImageModel(t testing.TB) *Model {
	t.Helper()
	m := newHoverRenderModel(t)
	pixels := image.NewNRGBA(image.Rect(0, 0, 1000, 700))
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range pixels.Pix {
		pixels.Pix[i] = byte(rng.Uint32())
	}
	for i := 3; i < len(pixels.Pix); i += 4 {
		pixels.Pix[i] = 255
	}
	var data bytes.Buffer
	if err := png.Encode(&data, pixels); err != nil {
		t.Fatal(err)
	}
	im, err := imagepreview.Decode("image.png", data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	m.tabs = []tab{{image: newImageState(im, 42)}}
	m.activeTab = 0
	m.diffOpen = false
	m.kittySupported = true
	m.sidebar = false
	m.View()
	return m
}

func BenchmarkImageInteractions(b *testing.B) {
	for _, kind := range []string{"sidebarHover", "pickerHover", "menuHover", "stationaryDrag", "edgePan"} {
		b.Run(kind, func(b *testing.B) {
			m := performanceImageModel(b)
			x, y := m.mainX()+8, m.contentRow()+5
			switch kind {
			case "pickerHover":
				m.openPalette()
			case "menuHover":
				m.openContextMenu(x, y)
			case "stationaryDrag":
				m.beginImagePan(x, y)
			case "edgePan":
				m.panImage(10000, 10000)
			}
			m.View()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				switch kind {
				case "sidebarHover":
					m.Update(tea.MouseMotionMsg{X: m.sideX() + 8, Y: 3 + i%2})
				case "pickerHover":
					l := m.pickerLayout(m.filteredActions())
					m.Update(tea.MouseMotionMsg{X: l.x + 8, Y: l.y + 3 + i%2})
				case "menuHover":
					m.Update(tea.MouseMotionMsg{X: x + 2, Y: y + 1 + i%2})
				case "stationaryDrag":
					m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
				case "edgePan":
					m.panImage(1, 1)
				}
				m.View()
			}
		})
	}
}

func BenchmarkFilteredPickerHover(b *testing.B) {
	m := newHoverRenderModel(b)
	m.modal = "quick"
	m.palette = nil
	for i := 0; i < 10000; i++ {
		p := fmt.Sprintf("src/package%03d/file%05d.go", i%100, i)
		m.palette = append(m.palette, action{id: p, label: p})
	}
	m.input.SetValue("file09")
	m.syncPicker()
	m.View()
	l := m.pickerLayout(m.filteredActions())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(tea.MouseMotionMsg{X: l.x + 8 + i%2, Y: l.y + 4})
		m.View()
	}
}
