package app

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/imagepreview"
)

func TestImageProtocolDetectionAndCellSize(t *testing.T) {
	m := tabWorkspace(t, 1)
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 32, 5)
	m.openFile("image.png")
	if strings.Contains(strings.Join(m.imageLines(), ""), "\x1b_G") {
		t.Fatal("transmitted before detection")
	}
	m.Update(imageProbeTimeoutMsg{})
	if !strings.Contains(strings.Join(m.imageLines(), ""), "No response") {
		t.Fatal("missing unsupported-terminal explanation")
	}
	m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: kittyProbeID}, Payload: []byte("OK")})
	if !m.kittySupported || !strings.Contains(imageUploadOutput(t, m), "\x1b_G") {
		t.Fatal("did not enable image protocol")
	}
	m.Update(uv.CellSizeEvent{Width: 10, Height: 22})
	if m.cellWidth != 10 || m.cellHeight != 22 || m.tabs[0].image.rows != nil {
		t.Fatal("cell geometry did not invalidate placement")
	}
	m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: 3}, Payload: []byte("ERROR")})
	if !m.kittySupported {
		t.Fatal("unrelated reply disabled image rendering")
	}
}

func TestDiffImagesSurviveTerminalRenderer(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 80, 40)
	imageGit(t, m, "add", "image.png")
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 120, 60)
	drain(t, m, m.requestDiff(diffTarget{path: "image.png"}, readImageDiff(m, gitservice.Change{Path: "image.png"}, false, "")))
	var output bytes.Buffer
	renderer := uv.NewTerminalRenderer(&output, []string{"TERM=xterm-kitty", "COLORTERM=truecolor"})
	renderer.SetFullscreen(true)
	renderer.SetColorProfile(colorprofile.ANSI256)
	frame := func() {
		output.WriteString(imageUploadOutput(t, m))
		scr := uv.NewScreenBuffer(m.width, m.height)
		uv.NewStyledString(m.View().Content).Draw(scr, uv.Rect(0, 0, m.width, m.height))
		renderer.Render(scr.RenderBuffer)
		if err := renderer.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	for _, im := range m.images.sides {
		if im == nil || !strings.Contains(output.String(), fmt.Sprintf("i=%d", im.id)) {
			t.Fatal("diff image upload did not reach terminal output")
		}
	}
	output.Reset()
	frame()
	if strings.Contains(output.String(), "\x1b_G") {
		t.Fatal("unchanged image diff retransmitted pixels")
	}
	left, right := m.images.sides[0], m.images.sides[1]
	drain(t, m, m.refreshDiffs())
	if m.images.sides[0] != left || m.images.sides[1] != right {
		t.Fatal("unchanged refresh replaced image placements")
	}
	output.Reset()
	frame()
	if strings.Contains(output.String(), "\x1b_G") {
		t.Fatal("unchanged refresh retransmitted pixels")
	}
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 140, 60)
	drain(t, m, m.refreshDiffs())
	if m.images.sides[0] != left || m.images.sides[1] == right || m.images.sides[1].Bounds().Dx() != 140 {
		t.Fatal("changed refresh replaced the unchanged side or retained old pixels")
	}
}

func TestImageProtocolSurvivesRendererAndCleansOwnedIDs(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 800, 600)
	m.openPreviewFile("image.png")
	id := m.tabs[0].image.id
	var output bytes.Buffer
	renderer := uv.NewTerminalRenderer(&output, []string{"TERM=xterm-kitty", "COLORTERM=truecolor"})
	renderer.SetFullscreen(true)
	renderer.SetColorProfile(colorprofile.ANSI256)
	frame := func() {
		output.WriteString(imageUploadOutput(t, m))
		scr := uv.NewScreenBuffer(m.width, m.height)
		uv.NewStyledString(m.View().Content).Draw(scr, uv.Rect(0, 0, m.width, m.height))
		renderer.Render(scr.RenderBuffer)
		if err := renderer.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	if !strings.Contains(output.String(), "\x1b_G") || !strings.Contains(output.String(), fmt.Sprintf("i=%d", id)) {
		t.Fatal("image protocol did not reach terminal output")
	}
	if !strings.Contains(output.String(), fmt.Sprintf("[38;5;%dm", id&255)) || !strings.Contains(output.String(), string(kitty.Diacritic(id>>24))) {
		t.Fatal("renderer changed placeholder identity in 256-color mode")
	}
	placeholderCount := strings.Count(m.View().Content, string(kitty.Placeholder))
	output.Reset()
	frame()
	if strings.Contains(output.String(), "\x1b_G") {
		t.Fatal("unchanged frame retransmitted image")
	}
	// A modal must replace the same cells that carry image placeholders.
	m.openPalette()
	modal := m.View().Content
	if count := strings.Count(modal, string(kitty.Placeholder)); count == 0 || count >= placeholderCount {
		t.Fatal("modal did not clip image placeholders")
	}
	if ansi.StringWidth(strings.Split(modal, "\n")[0]) != m.width {
		t.Fatal("modal damaged image frame width")
	}
	m.modal = ""
	m.openPreviewFile("file0.go")
	deletion := imagepreview.DeleteKitty(id)
	cmd := m.flushRetiredImages()
	if cmd == nil {
		t.Fatal("preview replacement did not release terminal image")
	}
	raw, ok := cmd().(tea.RawMsg)
	if !ok || !strings.Contains(fmt.Sprint(raw), "i=") {
		t.Fatal("missing image deletion")
	}
	if cleanup := m.ImageCleanupSequence(); cleanup != deletion {
		t.Fatalf("cleanup escaped image ownership: %q", cleanup)
	}
}

func TestImageContextMenuHoverDoesNotRetransmitPixels(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 800, 600)
	m.openPreviewFile("image.png")
	var output bytes.Buffer
	renderer := uv.NewTerminalRenderer(&output, []string{"TERM=xterm-kitty", "COLORTERM=truecolor"})
	renderer.SetFullscreen(true)
	frame := func() {
		output.WriteString(imageUploadOutput(t, m))
		scr := uv.NewScreenBuffer(m.width, m.height)
		uv.NewStyledString(m.View().Content).Draw(scr, uv.Rect(0, 0, m.width, m.height))
		renderer.Render(scr.RenderBuffer)
		if err := renderer.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	for _, x := range []int{m.mainX() + 1, m.mainX() + 10} {
		m.openContextMenu(x, m.contentRow()+1)
		for i := 0; i < 3; i++ {
			output.Reset()
			m.Update(tea.MouseMotionMsg{X: x + 2, Y: m.contentRow() + 3 + i%2})
			frame()
			if strings.Contains(output.String(), "\x1b_G") {
				t.Fatalf("menu hover retransmitted pixels at x=%d, step=%d", x, i)
			}
		}
		m.contextMenu = nil
		m.hoverOnly = false
		output.Reset()
		frame()
		if strings.Contains(output.String(), "\x1b_G") {
			t.Fatal("closing menu retransmitted pixels")
		}
	}
}

// Run the same transfer preparation and final filtering used by Runtime.
func imageUploadOutput(t testing.TB, m *Model) string {
	t.Helper()
	runtime := NewRuntime(m, nil)
	var output strings.Builder
	queue := []tea.Cmd{m.prepareImageUploads()}
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if raw, ok := runtime.Filter(runtime, msg).(tea.RawMsg); ok {
			output.WriteString(raw.Msg.(string))
		}
	}
	return output.String()
}
