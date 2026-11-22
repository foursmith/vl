package app

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/foursmith/vl/internal/imagepreview"
)

type imageState struct {
	*imagepreview.Image
	id                     int
	rows                   []string
	content                []string
	region                 imagepreview.Region
	upload                 *imageUpload
	renderErr              error
	lastUsed               uint64
	width, height          int
	zoom, centerX, centerY float64
}

func newImageState(im *imagepreview.Image, id int) *imageState {
	return &imageState{Image: im, id: id, zoom: 1, centerX: .5, centerY: .5}
}

func (m *Model) currentImage() *imagepreview.Image {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return nil
	}
	if im := m.tabs[m.activeTab].image; im != nil {
		return im.Image
	}
	return nil
}

func (m *Model) imageLines() []string {
	return m.imagePreviewLines(m.tabs[m.activeTab].image, m.mainWidth(), m.contentHeight())
}

func (m *Model) imagePreviewLines(im *imageState, w, h int) []string {
	if h <= 0 || w <= 0 {
		return nil
	}
	if !m.kittySupported {
		text := m.tr("Detecting terminal graphics protocol…")
		if m.kittyProbeDone {
			text = m.tr("No response to the Kitty graphics protocol; a terminal with Unicode placeholder support is required")
		}
		lines := make([]string, h)
		lines[h/2] = centerImageLabel(text, w)
		return lines
	}
	if im.width == w && im.height == h && im.rows != nil {
		return im.rows
	}
	lines := make([]string, h)
	switch {
	case im.renderErr != nil:
		lines[h/2] = "  " + safe(m.tr(im.renderErr.Error()))
	case im.content == nil:
		lines[h/2] = centerImageLabel(m.tr("Loading image…"), w)
	default:
		placeImageRows(lines, w, im.content, im.region)
	}
	im.width, im.height, im.rows = w, h, lines
	return lines
}

func centerImageLabel(text string, width int) string {
	text = ansi.Truncate(text, width, "")
	return strings.Repeat(" ", (width-ansi.StringWidth(text))/2) + text
}

func (m *Model) diffImagePreviewLines(im *imageState, w, h int) []string {
	if m.kittySupported && w > 0 && h > 0 && (im.rows == nil || im.width != w || im.height != h) {
		im.regionRows(im.viewRegion(w, h, m.cellWidth, m.cellHeight))
	}
	return m.imagePreviewLines(im, w, h)
}

func placeImageRows(lines []string, width int, content []string, region imagepreview.Region) {
	// A previous frame may be too large after a window resize.
	if region.Columns > width || region.Rows > len(lines) {
		return
	}
	padding := strings.Repeat(" ", (width-region.Columns)/2)
	top := (len(lines) - region.Rows) / 2
	for r, line := range content {
		lines[top+r] = padding + line
	}
}

func (im *imageState) hasRegion(region imagepreview.Region) bool {
	return im.region == region && (im.content != nil || im.renderErr != nil)
}

func (im *imageState) regionRows(region imagepreview.Region) []string {
	if !im.hasRegion(region) {
		upload, content, err := im.KittyRegion(im.id, region)
		im.setRegion(region, upload, content, err)
	}
	return im.content
}

func (im *imageState) setRegion(region imagepreview.Region, payload string, content []string, err error) {
	im.region, im.content, im.renderErr = region, content, err
	im.rows, im.upload = nil, nil
	if err == nil {
		im.upload = &imageUpload{target: im, payload: payload}
	}
}

type imageReloadMsg struct {
	original, fresh *imagepreview.Image
	seq             uint64
	err             error
}

func (m *Model) finishImageReload(msg imageReloadMsg) {
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.image == nil || t.image.Image != msg.original || t.reloadSeq != msg.seq {
			continue
		}
		if msg.err != nil {
			notice := m.tr("Image unavailable on disk: %s", m.tr(msg.err.Error()))
			t.diskNotice = notice
			return
		}
		t.diskNotice = ""
		if t.image.HasPixels() && msg.fresh.SameContent(t.image.Image) {
			return
		}
		t.image.replacePixels(msg.fresh)
		m.trimImageTabs()
		return
	}
}

func (im *imageState) replacePixels(fresh *imagepreview.Image) {
	im.Image, im.rows, im.content, im.upload, im.renderErr = fresh, nil, nil, nil, nil
}

const imageTabPixelBudget = 64 << 20

func (m *Model) trimImageTabs() {
	total := 0
	for i := range m.tabs {
		if im := m.tabs[i].image; im != nil {
			total += im.DecodedBytes()
		}
	}
	for total > imageTabPixelBudget {
		var oldest *imageState
		for i := range m.tabs {
			if im := m.tabs[i].image; i != m.activeTab && im != nil && im.HasPixels() &&
				(oldest == nil || im.lastUsed < oldest.lastUsed) {
				oldest = im
			}
		}
		if oldest == nil {
			return
		}
		total -= oldest.DecodedBytes()
		m.retireImageID(oldest.id)
		oldest.id = 0
		oldest.replacePixels(oldest.WithoutPixels())
	}
}

const kittyProbeID = 2147483647

type imageProbeTimeoutMsg struct{}

func (m *Model) probeImages() tea.Cmd {
	return tea.Batch(
		tea.Raw(ansi.KittyGraphics([]byte("AAAA"), "a=q", fmt.Sprintf("i=%d", kittyProbeID), "f=24", "s=1", "v=1", "t=d")),
		delayedMessage(m.ctx, 2*time.Second, func(time.Time) tea.Msg { return imageProbeTimeoutMsg{} }),
	)
}

func (m *Model) imageCapability(msg uv.KittyGraphicsEvent) {
	if msg.Options.ID != kittyProbeID {
		return
	}
	m.kittyProbeDone = true
	m.kittySupported = string(msg.Payload) == "OK"
}

func (m *Model) allocateImageID() int {
	for {
		id := int(rand.Uint32N(256))<<24 | (int(rand.Uint32N(255)) + 1)
		if !m.imageIDs[id] {
			m.imageIDs[id] = true
			return id
		}
	}
}

func (m *Model) retireImage(t *tab) {
	if t.image != nil {
		m.retireImageID(t.image.id)
	}
	if t.markdownImages != nil {
		for _, im := range t.markdownImages.files {
			if im != nil {
				m.retireImageID(im.id)
			}
		}
		for _, state := range t.markdownImages.remote {
			if state.image != nil {
				m.retireImageID(state.image.id)
			}
		}
	}
	if t.diff != nil {
		m.setDiffImages(t.diff, nil)
	}
}

func (m *Model) retireImageID(id int) {
	if id != 0 {
		m.imageRetired = append(m.imageRetired, id)
	}
}

type imageRetirementMsg []int

func (m *Model) flushRetiredImages() tea.Cmd {
	if len(m.imageRetired) == 0 {
		return nil
	}
	retired := imageRetirementMsg(m.imageRetired)
	m.imageRetired = nil
	return func() tea.Msg { return retired }
}

func (m *Model) imageRetirementMessage(retired imageRetirementMsg) tea.Msg {
	var out strings.Builder
	for _, id := range retired {
		if !m.imageIDs[id] {
			continue
		}
		out.WriteString(imagepreview.DeleteKitty(id))
		delete(m.imageIDs, id)
	}
	if out.Len() == 0 {
		return nil
	}
	return tea.RawMsg{Msg: out.String()}
}

// ImageCleanupSequence releases only this workbench's images, including IDs
// whose queued deletion may not have run before shutdown.
func (m *Model) ImageCleanupSequence() string {
	var out strings.Builder
	for id := range m.imageIDs {
		out.WriteString(imagepreview.DeleteKitty(id))
	}
	return out.String()
}

// Zoom is relative to the initial fit, and the center is normalized source space.
func (m *Model) imageRegion() imagepreview.Region {
	return m.tabs[m.activeTab].image.viewRegion(m.mainWidth(), m.contentHeight(), m.cellWidth, m.cellHeight)
}

func (im *imageState) viewRegion(w, h, cellWidth, cellHeight int) imagepreview.Region {
	return im.ViewRegion(max(1, w-4), max(1, h-2), cellWidth, cellHeight, im.zoom, im.centerX, im.centerY)
}

func (m *Model) imageContentAt(x, y int) bool {
	return !m.sidebarAt(x) &&
		x >= m.mainX() && x < m.mainX()+m.mainWidth() && y >= m.contentRow() && y < m.contentRow()+m.contentHeight()
}

func (m *Model) imagePointer(region imagepreview.Region, x, y int) (float64, float64) {
	cw, ch := m.cellWidth, m.cellHeight
	left, top := (m.mainWidth()-region.Columns)/2, (m.contentHeight()-region.Rows)/2
	px := (float64(x-m.mainX()-left) + .5) * float64(cw)
	py := (float64(y-m.contentRow()-top) + .5) * float64(ch)
	return min(max(0, px), region.Width*region.Scale), min(max(0, py), region.Height*region.Scale)
}

func (m *Model) zoomImage(factor float64, x, y int) {
	m.imagePan = nil
	if !m.kittySupported {
		return
	}
	t := &m.tabs[m.activeTab]
	before := m.imageRegion()
	next := min(32., max(.1, t.image.zoom*factor))
	if math.Abs(next-1) < .000001 {
		next = 1
	}
	if next == t.image.zoom {
		return
	}
	px, py := m.imagePointer(before, x, y)
	anchorX, anchorY := before.X+px/before.Scale, before.Y+py/before.Scale
	t.image.zoom = next
	after := m.imageRegion()
	px, py = m.imagePointer(after, x, y)
	bounds := t.image.Bounds()
	t.image.centerX = (anchorX - px/after.Scale + after.Width/2) / float64(bounds.Dx())
	t.image.centerY = (anchorY - py/after.Scale + after.Height/2) / float64(bounds.Dy())
	m.clampImageCenter()
	t.image.rows = nil
}

func (m *Model) resetImageZoom() {
	t := &m.tabs[m.activeTab]
	t.image.zoom, t.image.centerX, t.image.centerY, t.image.rows = 1, .5, .5, nil
	m.imagePan = nil
}

// Move the viewport by terminal cells, independent of image scale and size.
func (m *Model) panImage(dx, dy int) {
	m.imagePan = nil
	if !m.kittySupported {
		return
	}
	t := &m.tabs[m.activeTab]
	m.clampImageCenter()
	region := m.imageRegion()
	bounds := t.image.Bounds()
	t.image.centerX += float64(dx*m.cellWidth) / region.Scale / float64(bounds.Dx())
	t.image.centerY += float64(dy*m.cellHeight) / region.Scale / float64(bounds.Dy())
	m.clampImageCenter()
	t.image.rows = nil
}

func (m *Model) clampImageCenter() {
	t := &m.tabs[m.activeTab]
	region := m.imageRegion()
	bounds := t.image.Bounds()
	t.image.centerX = (region.X + region.Width/2) / float64(bounds.Dx())
	t.image.centerY = (region.Y + region.Height/2) / float64(bounds.Dy())
}

type imagePan struct {
	image                   *imagepreview.Image
	x, y                    int
	centerX, centerY, scale float64
}

func (m *Model) beginImagePan(x, y int) {
	m.focusEditor()
	if !m.kittySupported {
		return
	}
	region := m.imageRegion()
	m.clampImageCenter()
	t := &m.tabs[m.activeTab]
	m.imagePan = &imagePan{t.image.Image, x, y, t.image.centerX, t.image.centerY, region.Scale}
}

func (m *Model) dragImage(x, y int) {
	pan := m.imagePan
	if pan == nil || pan.image != m.currentImage() || m.modal != "" {
		m.imagePan = nil
		return
	}
	cw, ch := m.cellWidth, m.cellHeight
	t := &m.tabs[m.activeTab]
	b := t.image.Bounds()
	t.image.centerX = pan.centerX - float64((x-pan.x)*cw)/pan.scale/float64(b.Dx())
	t.image.centerY = pan.centerY - float64((y-pan.y)*ch)/pan.scale/float64(b.Dy())
	m.clampImageCenter()
	t.image.rows = nil
}
