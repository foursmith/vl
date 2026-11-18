// Package imagepreview decodes bounded, read-only raster and SVG previews.
package imagepreview

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xo/okage/svg"
	_ "golang.org/x/image/webp"
)

const MaxBytes = 32 << 20
const MaxPixels = 16 << 20

type Image struct {
	Path, Format string
	pixels       image.Image
	bounds       image.Rectangle
	vector       *svg.Document
	digest       [sha256.Size]byte
}

func Supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return true
	}
	return false
}

func Open(path string) (*Image, error) {
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
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	return Decode(path, data)
}

// Decode applies the same preview limits to Git blobs and disk files.
func Decode(path string, data []byte) (*Image, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		im, err := openSVG(path, data)
		if im != nil {
			im.digest = sha256.Sum256(data)
		}
		return im, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image preview: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxPixels/config.Height {
		return nil, fmt.Errorf("image exceeds 16 megapixel preview limit")
	}
	pixels, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image preview: %w", err)
	}
	return &Image{Path: path, Format: format, pixels: pixels, bounds: pixels.Bounds(), digest: sha256.Sum256(data)}, nil
}

// SameContent identifies previews whose source bytes and decoding format match.
func (im *Image) SameContent(other *Image) bool {
	return other != nil && im.Format == other.Format && im.digest == other.digest
}

// Bounds returns source dimensions without rasterizing vector content.
func (im *Image) Bounds() image.Rectangle { return im.bounds }
