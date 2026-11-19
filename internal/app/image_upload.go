package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/foursmith/vl/internal/imagepreview"
)

// Uploads belong to terminal I/O rather than cells. Keeping PNGs in cell content
// makes every redraw parse them and every uncovered cell retransmit them.
type imageUpload struct {
	target       *imageState
	payload      string
	queued, sent bool
}
type imageUploadMsg struct{ upload *imageUpload }

type imageRasterRequest struct {
	target *imageState
	source *imagepreview.Image
	id     int
	region imagepreview.Region
}

type imageRasterResult struct {
	*imageRasterRequest
	decoded *imagepreview.Image
	payload string
	rows    []string
	err     error
}

func (m *Model) prepareImageRaster(im *imageState) tea.Cmd {
	if m.imageRaster != nil {
		return nil
	}
	if im.id == 0 {
		im.id = m.allocateImageID()
	}
	region := m.imageRegion()
	if im.hasRegion(region) || im.renderErr != nil && !im.HasPixels() {
		return nil
	}
	request := imageRasterRequest{target: im, source: im.Image, id: im.id, region: region}
	m.imageRaster = &request
	ctx := m.ctx
	return func() tea.Msg {
		if !request.source.HasPixels() {
			decoded, err := imagepreview.Open(ctx, request.source.Path)
			return imageRasterResult{imageRasterRequest: &request, decoded: decoded, err: err}
		}
		payload, rows, err := request.source.KittyRegion(request.id, request.region)
		return imageRasterResult{imageRasterRequest: &request, payload: payload, rows: rows, err: err}
	}
}

func (m *Model) finishImageRaster(result imageRasterResult) {
	if m.imageRaster != result.imageRasterRequest {
		return
	}
	m.imageRaster = nil
	im := result.target
	if m.currentImage() != result.source || m.tabs[m.activeTab].image != im || im.id != result.id {
		return
	}
	if result.decoded != nil {
		im.replacePixels(result.decoded)
		m.trimImageTabs()
		return
	}
	if !result.source.HasPixels() {
		im.setRegion(result.region, "", nil, result.err)
		return
	}
	w, h := m.mainWidth(), m.contentHeight()
	if result.region.Columns > w || result.region.Rows > h {
		return
	}
	// Present completed frames during a gesture. The next job will pick up
	// the latest viewport, skipping any intermediate input events.
	current := m.imageRegion()
	if current != result.region && (result.err != nil || im.content != nil && im.region == current) {
		return
	}
	im.setRegion(result.region, result.payload, result.rows, result.err)
}

func (m *Model) visibleImageSurfaces(visit func(*imageState, int, int)) {
	if !m.kittySupported || m.width < 30 || m.height < 8 || m.width < 70 && m.sidebar && m.sidebarVisible {
		return
	}
	w, h := m.mainWidth(), m.contentHeight()
	if m.currentImage() != nil {
		visit(m.tabs[m.activeTab].image, w, h)
	} else if m.images != nil && !m.editing() && m.patch.Raw != "" && w >= 3 && h >= 2 {
		for side, im := range m.images.sides {
			if im != nil {
				width := (w - 1) / 2
				if side == 1 {
					width = w - 1 - width
				}
				visit(im, width, h-1)
			}
		}
	}
}

// Runtime prepares visible placements after Update, before View. Commands carry
// immutable upload identities; Filter checks them again immediately before I/O.
func (m *Model) prepareImageUploads() tea.Cmd {
	var commands []tea.Cmd
	queue := func(im *imageState) {
		upload := im.upload
		if upload == nil || upload.sent || upload.queued {
			return
		}
		upload.queued = true
		commands = append(commands, func() tea.Msg { return imageUploadMsg{upload} })
	}
	m.visibleImageSurfaces(func(im *imageState, w, h int) {
		if m.currentImage() != nil {
			if cmd := m.prepareImageRaster(im); cmd != nil {
				commands = append(commands, cmd)
			}
		} else {
			m.diffImagePreviewLines(im, w, h)
		}
		queue(im)
	})
	m.visibleMarkdownImages(func(block *markdownImageBlock) {
		block.image.regionRows(block.region)
		queue(block.image)
	})
	return tea.Batch(commands...)
}

func (m *Model) imageUploadMessage(msg imageUploadMsg) tea.Msg {
	upload := msg.upload
	upload.queued = false
	if upload.sent || upload.target.upload != upload {
		return nil
	}
	visible := false
	m.visibleImageSurfaces(func(im *imageState, _, _ int) { visible = visible || im == upload.target })
	m.visibleMarkdownImages(func(block *markdownImageBlock) { visible = visible || block.image == upload.target })
	if !visible {
		return nil
	}
	payload := upload.payload
	upload.sent, upload.payload = true, ""
	return tea.RawMsg{Msg: payload}
}
