package imagepreview

import (
	"fmt"
	"image"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

const (
	imageDownsampleThreshold = 500_000
	maxUploadSide            = 4096
)

// Region describes the visible part of an image in source pixel coordinates.
// Only this part is rasterized, so zooming never allocates a full enlarged image.
type Region struct {
	X, Y, Width, Height float64
	Scale               float64
	Columns, Rows       int
}

func (im *Image) ViewRegion(columns, rows, cellWidth, cellHeight int, zoom, centerX, centerY float64) Region {
	if columns <= 0 || rows <= 0 {
		return Region{}
	}
	columns, rows = min(255, columns), min(255, rows)
	b := im.Bounds()
	width, height := float64(b.Dx()), float64(b.Dy())
	scale := min(float64(columns*cellWidth)/width, float64(rows*cellHeight)/height)
	scale = min(scale, 1) * zoom
	visibleWidth, visibleHeight := min(width, float64(columns*cellWidth)/scale), min(height, float64(rows*cellHeight)/scale)
	x := min(max(0, centerX*width-visibleWidth/2), width-visibleWidth)
	y := min(max(0, centerY*height-visibleHeight/2), height-visibleHeight)
	return Region{X: x, Y: y, Width: visibleWidth, Height: visibleHeight, Scale: scale,
		Columns: min(columns, max(1, int(math.Ceil(visibleWidth*scale/float64(cellWidth))))),
		Rows:    min(rows, max(1, int(math.Ceil(visibleHeight*scale/float64(cellHeight)))))}
}

// KittyRegion separates the pixel transfer from the virtual placement's text.
// Upload once via terminal I/O; redraw placeholders without retransmitting PNGs.
func (im *Image) KittyRegion(id int, region Region) (string, []string, error) {
	if im.pixels == nil || id <= 0 || uint64(id) > 0xffffffff || id&0x00ffff00 != 0 ||
		region.Columns <= 0 || region.Rows <= 0 || region.Columns > 255 || region.Rows > 255 ||
		region.Width <= 0 || region.Height <= 0 || region.Scale <= 0 {
		return "", nil, fmt.Errorf("invalid Kitty image region")
	}
	scale := region.Scale
	if im.sourceBytes > imageDownsampleThreshold {
		// Reduce the uploaded pixel count in proportion to the byte ratio.
		// The terminal scales the raster into the original cell placement.
		scale *= math.Sqrt(float64(imageDownsampleThreshold) / float64(im.sourceBytes))
	}
	// The terminal scales the raster to the display cells. Enlarging it here
	// adds encode and transfer work without adding source detail.
	scale = min(scale, 1, maxUploadSide/max(region.Width, region.Height))
	width := max(1, int(math.Round(region.Width*scale)))
	height := max(1, int(math.Round(region.Height*scale)))
	b := im.Bounds()
	whole := region.X == 0 && region.Y == 0 && region.Width == float64(b.Dx()) && region.Height == float64(b.Dy())
	if whole && width == b.Dx() && height == b.Dy() {
		return kittyPixelRows(id, region.Columns, region.Rows, im.pixels)
	}
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	sx, sy := float64(width)/region.Width, float64(height)/region.Height
	transform := f64.Aff3{sx, 0, -(region.X + float64(b.Min.X)) * sx, 0, sy, -(region.Y + float64(b.Min.Y)) * sy}
	if max(sx, sy) < .25 {
		// Take four bilinear samples per output pixel at thumbnail sizes,
		// where a single sample can miss fine patterns in the source.
		samples := image.NewRGBA(image.Rect(0, 0, width*2, height*2))
		for i := range transform {
			transform[i] *= 2
		}
		draw.ApproxBiLinear.Transform(samples, transform, im.pixels, b, draw.Src, nil)
		draw.ApproxBiLinear.Scale(target, target.Bounds(), samples, samples.Bounds(), draw.Src, nil)
	} else {
		draw.ApproxBiLinear.Transform(target, transform, im.pixels, b, draw.Src, nil)
	}
	return kittyPixelRows(id, region.Columns, region.Rows, target)
}
