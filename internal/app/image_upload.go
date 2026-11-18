package app

import tea "charm.land/bubbletea/v2"

// Uploads belong to terminal I/O rather than cells. Keeping PNGs in cell content
// makes every redraw parse them and every uncovered cell retransmit them.
type imageUpload struct {
	target       *imageState
	payload      string
	queued, sent bool
}
type imageUploadMsg struct{ upload *imageUpload }

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
	m.visibleImageSurfaces(func(im *imageState, w, h int) {
		m.imagePreviewLines(im, w, h)
		upload := im.upload
		if upload == nil || upload.sent || upload.queued {
			return
		}
		upload.queued = true
		commands = append(commands, func() tea.Msg { return imageUploadMsg{upload} })
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
	if !visible {
		return nil
	}
	payload := upload.payload
	upload.sent, upload.payload = true, ""
	return tea.RawMsg{Msg: payload}
}
