package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestMarkdownImageRowsAndUpload(t *testing.T) {
	dir := t.TempDir()
	writePreviewPNG(t, filepath.Join(dir, "local image.png"), 160, 128)
	writePreviewPNG(t, filepath.Join(dir, "fenced.png"), 80, 64)
	const heading = "# Notes\n"
	const reference = "![local](local%20image.png)\n"
	source := heading + reference + "After\n" +
		"```md\n![fenced](fenced.png)\n```\n" + strings.Repeat("Tail\n", 25)
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if cmd := m.scheduleMarkdownImages(); cmd != nil {
		t.Fatal("loaded inline images without Kitty support")
	}
	m.kittySupported = true
	load := m.scheduleMarkdownImages()
	if load == nil {
		t.Fatal("local image was not scheduled")
	}
	loaded, ok := load().(markdownImageLoadedMsg)
	if !ok {
		t.Fatal("expected only the local raster image to be scheduled")
	}
	if loaded.image == nil || loaded.path != filepath.Join(filepath.Dir(m.document().Path), "local image.png") {
		t.Fatalf("expected just the local file, got %q", loaded.path)
	}
	m.Update(loaded)

	tab := &m.tabs[m.activeTab]
	if len(tab.markdownImages.blocks) != 1 {
		t.Fatalf("expected one image block, got %+v", tab.markdownImages.blocks)
	}
	b := tab.markdownImages.blocks[0]
	if b.line != 1 || b.first != 2 || b.region.Rows < 2 {
		t.Fatalf("unexpected image placement: %+v", b)
	}
	placeholder := string(kitty.Placeholder)
	lines := m.editorLines()
	if !strings.Contains(lines[1], "local%20image") || !strings.Contains(lines[b.first], placeholder) ||
		!strings.Contains(lines[b.first+b.region.Rows], "After") {
		t.Fatalf("expected the source, inline image, then text: %q", lines)
	}
	if upload := imageUploadOutput(t, m); !strings.Contains(upload, "\x1b_G") || imageUploadOutput(t, m) != "" {
		t.Fatal("expected exactly one pixel upload for the unchanged image")
	}

	m.positionEditor(8, b.first+b.region.Rows, false)
	if line, _ := tab.doc.Position(); line != 2 {
		t.Fatalf("click after image went to source row %d", line)
	}
	if cursor := m.editorCursor(); cursor == nil || cursor.Y != b.first+b.region.Rows {
		t.Fatalf("cursor not drawn after image: %v", cursor)
	}
	tab.setEditorDisplayTop(b.first + 1)
	if tab.editorDisplayTop() != b.first+1 || !strings.Contains(m.editorLines()[0], placeholder) || m.textRowAt(0) != -1 {
		t.Fatal("could not scroll partway into the image")
	}
	if tab.doc.Text != source || tab.doc.Dirty() {
		t.Fatal("displaying the image changed the source document")
	}

	tab.doc.Edit(strings.Replace(source, reference, "", 1))
	m.scheduleMarkdownImages()
	m.ensureEditor()
	if len(tab.markdownImages.blocks) != 0 || m.flushRetiredImages() == nil {
		t.Fatal("removing the reference left a placement or Kitty image behind")
	}
}

func TestMarkdownRemoteImageClickRetryAndOfflineCache(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl unavailable")
	}
	t.Setenv("NO_PROXY", "127.0.0.1")
	t.Setenv("no_proxy", "127.0.0.1")
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	pngPath := filepath.Join(dir, "image.png")
	writePreviewPNG(t, pngPath, 160, 128)
	png, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(png)
	}))
	defer server.Close()

	path := filepath.Join(dir, "notes.md")
	source := fmt.Sprintf("![remote](%s/image?q=1) ![same](%s/image?q=1#fragment)\nAfter\n", server.URL, server.URL)
	source += "![scheme-relative](//example.test/image.png)\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	open := func(kitty bool) *Model {
		m, err := NewEditor(t.Context(), path, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(m.Close)
		m.kittySupported = kitty
		_, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 32})
		drain(t, m, cmd)
		return m
	}
	m := open(false)
	if tab := &m.tabs[m.activeTab]; tab.markdownImages != nil || !strings.Contains(m.editorLines()[1], "After") {
		t.Fatal("inserted a placeholder without graphics support")
	}

	m = open(true)
	tab := &m.tabs[m.activeTab]
	if len(tab.markdownImages.blocks) != 2 {
		t.Fatal("expected two placeholders")
	}
	first, second := tab.markdownImages.blocks[0], tab.markdownImages.blocks[1]
	pixelWidth, pixelHeight := first.region.Columns*m.cellWidth, first.region.Rows*m.cellHeight
	if delta := 9*pixelWidth - 16*pixelHeight; delta < -9*m.cellWidth/2 || delta > 9*m.cellWidth/2 {
		t.Fatalf("placeholder is %d×%d pixels, want 16:9 within half a column", pixelWidth, pixelHeight)
	}
	labelRow := second.first + (second.region.Rows-1)/2
	bottomRow := second.first + second.region.Rows - 1
	lines := m.editorLines()
	if first.region.Rows < 5 || !strings.Contains(lines[first.first], "╭") ||
		!strings.Contains(lines[labelRow], "Load image") || !strings.Contains(lines[bottomRow], "╯") ||
		!strings.Contains(lines[bottomRow+1], "After") || requests.Load() != 0 {
		t.Fatal("expected two framed placeholders and no automatic download")
	}
	outside := tea.MouseClickMsg{X: m.mainX() + editorGutterWidth + second.region.Columns, Y: m.contentRow() + bottomRow, Button: tea.MouseLeft}
	_, cmd := m.Update(outside)
	drain(t, m, cmd)
	if requests.Load() != 0 {
		t.Fatal("click outside the frame triggered a download")
	}
	click := outside
	click.X--
	_, cmd = m.Update(click)
	if !strings.Contains(m.editorLines()[labelRow], "Loading image") {
		t.Fatal("click did not show loading state")
	}
	drain(t, m, cmd)
	if !strings.Contains(m.editorLines()[labelRow], "Retry image") || requests.Load() != 1 {
		t.Fatal("failed download did not offer retry")
	}
	_, cmd = m.Update(click)
	drain(t, m, cmd)
	blocks := tab.markdownImages.blocks
	if len(blocks) != 2 || blocks[0].image == nil || blocks[0].image != blocks[1].image || requests.Load() != 2 {
		t.Fatal("expected both placements to reuse the downloaded image")
	}
	if upload := imageUploadOutput(t, m); !strings.Contains(upload, "\x1b_G") || imageUploadOutput(t, m) != "" {
		t.Fatal("expected one shared Kitty upload")
	}
	tab.setEditorDisplayTop(blocks[1].first)
	if tab.editorDisplayTop() != blocks[1].first {
		t.Fatal("could not scroll to the second placement on the same source line")
	}

	server.Close()
	m = open(true)
	if blocks := m.tabs[m.activeTab].markdownImages.blocks; len(blocks) != 2 || blocks[0].image == nil ||
		blocks[0].image != blocks[1].image || requests.Load() != 2 {
		t.Fatal("reopening offline did not automatically render the cached image")
	}
}
