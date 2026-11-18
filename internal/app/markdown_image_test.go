package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	loaded := load().(markdownImageLoadedMsg)
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

func TestMarkdownRemoteImageAndSharePage(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "image.png")
	writePreviewPNG(t, local, 96, 80)
	pixels, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" && r.URL.Query().Get("format") == "png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pixels)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><img src='/asset?format=png'></html>"))
	}))
	defer server.Close()
	remote := server.URL + "/asset?format=png"
	share := server.URL + "/share.png"
	source := "![local](./image.png)\n\n![remote](" + remote + ")\n\n![share page](" + share + ")\n"
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.kittySupported = true
	pending := m.scheduleMarkdownImages()()
	cmds, ok := pending.(tea.BatchMsg)
	if !ok || len(cmds) != 3 {
		t.Fatalf("expected independent local and HTTPS/HTTP commands: %T, %v", pending, pending)
	}
	m.Update(cmds[0]())
	tab := &m.tabs[m.activeTab]
	if len(tab.markdownImages.blocks) != 1 || tab.markdownImages.blocks[0].line != 0 {
		t.Fatal("local image did not appear independently from remote requests")
	}
	for _, cmd := range cmds[1:] {
		m.Update(cmd())
	}
	if im := tab.markdownImages.files[remote]; im == nil || im.Bounds().Dx() != 96 {
		t.Fatal("image URL with a query and no filename extension was not loaded")
	}
	if tab.markdownImages.files[share] != nil || len(tab.markdownImages.blocks) != 2 || tab.markdownImages.blocks[1].line != 2 {
		t.Fatal("HTML share page was treated as an image")
	}
	if m.document().Text != source {
		t.Fatal("remote image loading changed the source")
	}
}
