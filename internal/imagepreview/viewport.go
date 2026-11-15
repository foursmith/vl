package imagepreview

import (
	"fmt"
	"image"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
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
	if im.vector == nil {
		scale = min(scale, 1)
	}
	scale *= zoom
	visibleWidth, visibleHeight := min(width, float64(columns*cellWidth)/scale), min(height, float64(rows*cellHeight)/scale)
	x := min(max(0, centerX*width-visibleWidth/2), width-visibleWidth)
	y := min(max(0, centerY*height-visibleHeight/2), height-visibleHeight)
	return Region{X: x, Y: y, Width: visibleWidth, Height: visibleHeight, Scale: scale,
		Columns: min(columns, max(1, int(math.Ceil(visibleWidth*scale/float64(cellWidth))))),
		Rows:    min(rows, max(1, int(math.Ceil(visibleHeight*scale/float64(cellHeight)))))}
}

// KittyRegionRows sends a bounded PNG crop using a virtual Kitty placement.
func (im *Image) KittyRegionRows(id int, region Region) ([]string, error) {
	if id <= 0 || uint64(id) > 0xffffffff || id&0x00ffff00 != 0 ||
		region.Columns <= 0 || region.Rows <= 0 || region.Columns > 255 || region.Rows > 255 ||
		region.Width <= 0 || region.Height <= 0 || region.Scale <= 0 {
		return nil, fmt.Errorf("invalid Kitty image region")
	}
	width, height := max(1, int(math.Round(region.Width*region.Scale))), max(1, int(math.Round(region.Height*region.Scale)))
	transferScale := min(1, 4096/float64(max(width, height)))
	width, height = max(1, int(float64(width)*transferScale)), max(1, int(float64(height)*transferScale))
	var pixels image.Image
	if im.vector != nil {
		pixmap, err := im.rasterSVGRegion(width, height, region)
		if err != nil {
			return nil, err
		}
		pixels = pixmap.AsRGBA()
	} else {
		target := image.NewRGBA(image.Rect(0, 0, width, height))
		sx, sy := float64(width)/region.Width, float64(height)/region.Height
		b := im.Bounds()
		transform := f64.Aff3{sx, 0, -(region.X + float64(b.Min.X)) * sx, 0, sy, -(region.Y + float64(b.Min.Y)) * sy}
		draw.CatmullRom.Transform(target, transform, im.pixels, b, draw.Src, nil)
		pixels = target
	}
	return kittyPixelRows(id, region.Columns, region.Rows, pixels)
}
