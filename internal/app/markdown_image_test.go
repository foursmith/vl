package app

import (
	"context"
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
		"![http](http://example.test/image.png)\n" +
		"![https](https://example.test/image.png)\n" +
		"![host-relative](//example.test/image.png)\n" +
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
