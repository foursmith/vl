// Package imagepreview decodes bounded, read-only image previews.
package imagepreview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const MaxBytes = 32 << 20
const MaxPixels = 16 << 20

type Image struct {
	Path, Format string
	sourceBytes  int
	pixels       image.Image
	bounds       image.Rectangle
	digest       [sha256.Size]byte
}

func Supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	case ".svg":
		_, err := exec.LookPath("resvg")
		return err == nil
	}
	return false
}

func Open(ctx context.Context, path string) (*Image, error) {
	data, err := readImageFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(ctx, path, data)
}

func readImageFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("image preview requires a regular file")
	}
	if info.Size() > MaxBytes {
		return nil, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	return io.ReadAll(io.LimitReader(f, MaxBytes+1))
}

// Decode applies the same preview limits to Git blobs and disk files.
func Decode(ctx context.Context, path string, source []byte) (*Image, error) {
	if len(source) > MaxBytes {
		return nil, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	data := source
	svg := strings.EqualFold(filepath.Ext(path), ".svg")
	if svg {
		var err error
		data, err = renderSVG(ctx, data)
		if err != nil {
			return nil, err
		}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image preview: %w", err)
	}
	if svg && format != "png" {
		return nil, fmt.Errorf("resvg did not return a PNG")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxPixels/config.Height {
		return nil, fmt.Errorf("image exceeds 16 megapixel preview limit")
	}
	pixels, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image preview: %w", err)
	}
	// Alpha WebP decodes to NYCbCrA. Convert once so the resampler can use
	// packed pixels without converting every tap of every viewport.
	if webp, ok := pixels.(*image.NYCbCrA); ok {
		bounds := webp.Bounds()
		rgba := image.NewRGBA(bounds)
		// Let the standard draw fast path convert YCbCr, then apply alpha
		// in place so the packed RGBA is premultiplied.
		draw.Copy(rgba, bounds.Min, &webp.YCbCr, bounds, draw.Src, nil)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := rgba.Pix[rgba.PixOffset(bounds.Min.X, y):]
			start := webp.AOffset(bounds.Min.X, y)
			for x, a := range webp.A[start : start+bounds.Dx()] {
				if a == 255 {
					continue
				}
				i := 4 * x
				for c := range 3 {
					row[i+c] = byte((uint16(row[i+c])*uint16(a) + 127) / 255)
				}
				row[i+3] = a
			}
		}
		pixels = rgba
	}
	if svg {
		format = "svg"
	}
	return &Image{
		Path: path, Format: format, sourceBytes: len(source),
		pixels: pixels, bounds: pixels.Bounds(), digest: sha256.Sum256(source),
	}, nil
}

// SameContent identifies previews whose source bytes and decoding format match.
func (im *Image) SameContent(other *Image) bool {
	return other != nil && im.Format == other.Format && im.digest == other.digest
}

// Bounds returns the image dimensions even when only metadata is cached.
func (im *Image) Bounds() image.Rectangle { return im.bounds }

func (im *Image) HasPixels() bool { return im.pixels != nil }

// DecodedBytes estimates the retained pixel storage for the supported formats.
func (im *Image) DecodedBytes() int {
	switch pixels := im.pixels.(type) {
	case nil:
		return 0
	case *image.RGBA:
		return len(pixels.Pix)
	case *image.NRGBA:
		return len(pixels.Pix)
	case *image.YCbCr:
		return len(pixels.Y) + len(pixels.Cb) + len(pixels.Cr)
	case *image.Gray:
		return len(pixels.Pix)
	case *image.Paletted:
		return len(pixels.Pix) + len(pixels.Palette)*16
	default:
		return im.Bounds().Dx() * im.Bounds().Dy() * 8
	}
}

// WithoutPixels preserves the tab metadata without mutating an image that a
// background viewport render may still be using.
func (im *Image) WithoutPixels() *Image {
	metadata := *im
	metadata.pixels = nil
	return &metadata
}

type previewOutput struct{ buf bytes.Buffer }

func (out *previewOutput) Write(p []byte) (int, error) {
	if len(p) > MaxBytes-out.buf.Len() {
		return 0, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	return out.buf.Write(p)
}
