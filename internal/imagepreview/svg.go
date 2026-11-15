package imagepreview

import (
	"fmt"
	"image"
	"io"
	"log/slog"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
)

var svgLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
var svgRenderer = svg.New(
	svg.WithMaxDepth(128),
	svg.WithLogger(svgLogger),
	svg.WithImageHrefResolver(svg.ImageHrefResolver{
		// External resources are not part of a self-contained file preview.
		ResolveString: func(string) (svg.ImageKind, bool) { return nil, false },
	}),
)

func openSVG(path string, data []byte) (*Image, error) {
	doc, err := svgRenderer.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("SVG preview: %w", err)
	}
	config, err := doc.Config()
	if err != nil {
		return nil, fmt.Errorf("SVG preview: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxPixels/config.Height {
		return nil, fmt.Errorf("image exceeds 16 megapixel preview limit")
	}
	return &Image{Path: path, Format: "svg", bounds: image.Rect(0, 0, config.Width, config.Height), vector: doc}, nil
}

func (im *Image) rasterSVGRegion(width, height int, region Region) (*okage.Pixmap, error) {
	pixmap, ok := okage.NewPixmap(uint32(width), uint32(height))
	if !ok {
		return nil, fmt.Errorf("invalid SVG preview size")
	}
	sx, sy := float32(float64(width)/region.Width), float32(float64(height)/region.Height)
	transform := okage.TransformFromRow(sx, 0, 0, sy, -float32(region.X)*sx, -float32(region.Y)*sy)
	target := pixmap.AsMut()
	svg.Render(im.vector.Tree(), transform, &target, svg.WithLogger(svgLogger))
	return pixmap, nil
}
