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

	_ "golang.org/x/image/webp"
)

const MaxBytes = 32 << 20
const MaxPixels = 16 << 20

type Image struct {
	Path, Format string
	sourceBytes  int
	pixels       image.Image
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
	if svg {
		format = "svg"
	}
	return &Image{Path: path, Format: format, sourceBytes: len(source), pixels: pixels, digest: sha256.Sum256(source)}, nil
}

// SameContent identifies previews whose source bytes and decoding format match.
func (im *Image) SameContent(other *Image) bool {
	return other != nil && im.Format == other.Format && im.digest == other.digest
}

// Bounds returns the decoded image bounds.
func (im *Image) Bounds() image.Rectangle { return im.pixels.Bounds() }

type previewOutput struct{ buf bytes.Buffer }

func (out *previewOutput) Write(p []byte) (int, error) {
	if len(p) > MaxBytes-out.buf.Len() {
		return 0, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	return out.buf.Write(p)
}
