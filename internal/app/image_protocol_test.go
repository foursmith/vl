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
	"github.com/ourongxing/elegit/internal/imagepreview"
)

func TestImageProtocolDetectionAndCellSize(t *testing.T) {
	m := tabWorkspace(t, 1)
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 32, 5)
	m.openFile("image.png")
	if strings.Contains(strings.Join(m.imageLines(), ""), "\x1b_G") {
		t.Fatal("transmitted before detection")
	}
	m.Update(imageProbeTimeoutMsg{})
	if !strings.Contains(strings.Join(m.imageLines(), ""), "未响应") {
		t.Fatal("missing unsupported-terminal explanation")
	}
	m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: kittyProbeID}, Payload: []byte("OK")})
	if !m.kittySupported || !strings.Contains(strings.Join(m.imageLines(), ""), "\x1b_G") {
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
	m.modal = "palette"
	m.input.SetValue(">")
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
