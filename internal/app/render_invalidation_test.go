package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/ourongxing/vl/internal/imagepreview"
)

func pendingImageUpload(t testing.TB, m *Model) imageUploadMsg {
	t.Helper()
	cmd := m.prepareImageUploads()
	if cmd == nil {
		t.Fatal("missing image upload")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) != 1 {
			t.Fatalf("expected one upload, got %d", len(batch))
		}
		msg = batch[0]()
	}
	upload, ok := msg.(imageUploadMsg)
	if !ok {
		t.Fatalf("unexpected upload command %T", msg)
	}
	return upload
}

func TestImageUploadRejectsObsoleteHiddenAndClosedTargets(t *testing.T) {
	m := zoomWorkspace(t)
	defer m.Close()
	r := NewRuntime(m, nil)
	first := pendingImageUpload(t, m)
	m.zoomImage(2, m.mainX()+10, m.contentRow()+5)
	next := pendingImageUpload(t, m)
	if r.Filter(r, first) != nil {
		t.Fatal("obsolete crop reached terminal")
	}
	if _, ok := r.Filter(r, next).(tea.RawMsg); !ok {
		t.Fatal("current crop was dropped")
	}
	if r.Filter(r, next) != nil || next.upload.payload != "" {
		t.Fatal("upload replayed or retained sent pixels")
	}
	m.zoomImage(2, m.mainX()+10, m.contentRow()+5)
	hidden := pendingImageUpload(t, m)
	m.openFile("file0.go")
	if r.Filter(r, hidden) != nil {
		t.Fatal("hidden image uploaded")
	}
	m.activateTab(0)
	retried := pendingImageUpload(t, m)
	if _, ok := r.Filter(r, retried).(tea.RawMsg); !ok {
		t.Fatal("hidden upload was not retried on return")
	}
	m.zoomImage(2, m.mainX()+10, m.contentRow()+5)
	closed := pendingImageUpload(t, m)
	m.closeTab()
	if r.Filter(r, closed) != nil {
		t.Fatal("closed image was uploaded after retirement")
	}
}

func TestImageReloadAndDuplicateCellReportRetainPixels(t *testing.T) {
	m := zoomWorkspace(t)
	defer m.Close()
	imageUploadOutput(t, m)
	im := m.tabs[0].image
	original, upload := im.Image, im.upload
	rows := im.rows
	fresh, err := imagepreview.Open(t.Context(), original.Path)
	if err != nil {
		t.Fatal(err)
	}
	m.tabs[0].diskNotice = "previous failure"
	m.finishImageReload(imageReloadMsg{original: original, fresh: fresh, seq: m.tabs[0].reloadSeq})
	if im.Image != original || &im.rows[0] != &rows[0] || m.tabs[0].diskNotice != "" {
		t.Fatal("identical reload invalidated placement or retained notice")
	}
	m.Update(uv.CellSizeEvent{Width: m.cellWidth, Height: m.cellHeight})
	if &im.rows[0] != &rows[0] {
		t.Fatal("identical cell report invalidated placement")
	}
	m.resetImageZoom()
	m.panImage(10000, 10000)
	imageUploadOutput(t, m)
	if im.upload != upload {
		t.Fatal("unchanged visible crop was encoded again")
	}
	writePreviewPNG(t, filepath.Join(m.repo.Root, "image.png"), 240, 160)
	fresh, err = imagepreview.Open(t.Context(), original.Path)
	if err != nil {
		t.Fatal(err)
	}
	m.finishImageReload(imageReloadMsg{original: original, fresh: fresh, seq: m.tabs[0].reloadSeq})
	if im.Image == original || im.content != nil || !strings.Contains(imageUploadOutput(t, m), "\x1b_G") {
		t.Fatal("changed pixels retained the old crop")
	}
}

func TestImageLayoutOnlyResizeDoesNotReupload(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	m.kittySupported = true
	writePreviewPNG(t, filepath.Join(m.repo.Root, "tiny.png"), 9, 5)
	m.openFile("tiny.png")
	imageUploadOutput(t, m)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if got := imageUploadOutput(t, m); got != "" {
		t.Fatal("margin-only resize retransmitted image")
	}
	if strings.Contains(m.View().Content, "\x1b_G") {
		t.Fatal("pixel payload leaked into frame")
	}
}

func TestPickerFilterInvalidation(t *testing.T) {
	m := newHoverRenderModel(t)
	m.modal = "quick"
	m.palette = []action{{id: "alpha.go", label: "alpha.go"}, {id: "beta.go", label: "beta.go"}}
	m.input.SetValue("alpha")
	first := m.filteredActions()
	if len(first) != 1 || first[0].id != "alpha.go" {
		t.Fatal("incorrect initial filter")
	}
	m.Update(tea.MouseMotionMsg{X: 1, Y: 1})
	if rows := m.filteredActions(); &rows[0] != &first[0] {
		t.Fatal("passive motion repeated filtering")
	}
	m.input.SetValue("beta")
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "beta.go" {
		t.Fatal("query retained stale rows")
	}
	m.palette = []action{{id: "beta-new.go", label: "beta-new.go"}}
	if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "beta-new.go" {
		t.Fatal("replacement palette retained stale rows")
	}
	m.palette[0].label = "Updated beta"
	m.Update(operationMsg{title: "Refresh"})
	if rows := m.filteredActions(); rows[0].label != "Updated beta" {
		t.Fatal("background update retained stale labels")
	}
}

func TestImageRenderErrorDropsPendingUpload(t *testing.T) {
	m := zoomWorkspace(t)
	defer m.Close()
	pending := pendingImageUpload(t, m)
	im := m.tabs[0].image
	id := im.id
	m.zoomImage(2, m.mainX()+10, m.contentRow()+5)
	im.id = 0 // Force the protocol encoder's validation error.
	if got := imageUploadOutput(t, m); got != "" {
		t.Fatal("failed crop uploaded stale pixels")
	}
	if NewRuntime(m, nil).Filter(nil, pending) != nil {
		t.Fatal("failed crop retained an obsolete upload")
	}
	rows := im.rows
	m.imageLines()
	if &im.rows[0] != &rows[0] {
		t.Fatal("unchanged render failure was recomputed")
	}
	im.id, im.rows = id, nil
	if got := imageUploadOutput(t, m); got == "" {
		t.Fatal("valid retry did not restore image upload")
	}
}
