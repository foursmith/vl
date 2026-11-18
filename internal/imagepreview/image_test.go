package imagepreview

import (
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRaster(pixels image.Image) *Image {
	return &Image{pixels: pixels}
}

func TestFormatsAndTransparency(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "gif"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample."+format)
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			pixels := image.NewNRGBA(image.Rect(0, 0, 8, 4))
			pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
			switch format {
			case "png":
				err = png.Encode(f, pixels)
			case "jpeg":
				err = jpeg.Encode(f, pixels, nil)
			case "gif":
				err = gif.Encode(f, pixels, nil)
			}
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			im, err := Open(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if im.Format != format || im.Bounds().Dx() != 8 {
				t.Fatalf("%+v", im)
			}
			if format == "png" {
				_, _, _, alpha := im.pixels.At(1, 0).RGBA()
				if alpha != 0 {
					t.Fatal("lost transparency")
				}
				red, green, blue, alpha := im.pixels.At(0, 0).RGBA()
				if red != 65535 || green != 0 || blue != 0 || alpha != 65535 {
					t.Fatal("changed opaque pixel")
				}
			}
		})
	}
}

func TestFitPreservesAspectAndBounds(t *testing.T) {
	im := testRaster(image.NewNRGBA(image.Rect(0, 0, 800, 400)))
	for _, size := range [][2]int{{80, 24}, {1, 1}, {500, 500}, {0, 0}} {
		region := im.ViewRegion(size[0], size[1], 8, 16, 1, .5, .5)
		w, h := region.Columns, region.Rows
		if w > size[0] || h > size[1] {
			t.Fatalf("fit %v: %d %d", size, w, h)
		}
		if w > 1 && h > 1 && w != h*4 {
			t.Fatalf("aspect: %d %d", w, h)
		}
	}
}

func TestSVGPreviewWithResvg(t *testing.T) {
	const path = "icon.SVG"
	if !Supported(path) {
		t.Skip("resvg is unavailable")
	}
	source := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20"><rect width="20" height="20" fill="red"/></svg>`)
	im, err := Decode(t.Context(), path, source)
	if err != nil {
		t.Fatal(err)
	}
	bounds := im.Bounds()
	if im.Format != "svg" || bounds.Dx() != 2*bounds.Dy() || bounds.Dx() > 2048 {
		t.Fatalf("unexpected SVG preview: %s %v", im.Format, bounds)
	}
	red, _, _, alpha := im.pixels.At(0, 0).RGBA()
	if red != 65535 || alpha != 65535 {
		t.Fatal("SVG lost its opaque red pixels")
	}
	if _, _, _, alpha := im.pixels.At(bounds.Max.X-1, 0).RGBA(); alpha != 0 {
		t.Fatal("SVG lost its transparent pixels")
	}
}

func TestRejectInvalidAndOversizedImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.png")
	os.WriteFile(path, []byte("not an image"), 0600)
	if _, err := Open(t.Context(), path); err == nil {
		t.Fatal("accepted corrupt image")
	}
	f, _ := os.Create(path)
	f.Truncate(MaxBytes + 1)
	f.Close()
	if _, err := Open(t.Context(), path); err == nil {
		t.Fatal("accepted oversized image")
	}
	// PNG's IHDR is sufficient for DecodeConfig; reject dimensions before decoding pixels.
	var data []byte
	// Use an encoded image and replace the dimensions and CRC.
	tiny := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	f, _ = os.Create(path)
	png.Encode(f, tiny)
	f.Close()
	data, _ = os.ReadFile(path)
	data[16], data[17], data[18], data[19] = 0, 1, 0, 0
	data[20], data[21], data[22], data[23] = 0, 1, 0, 0
	// Recompute CRC so this exercises the allocation guard.
	checksum := crc32.ChecksumIEEE(data[12:29])
	binary.BigEndian.PutUint32(data[29:33], checksum)
	os.WriteFile(path, data, 0600)
	if _, err := Open(t.Context(), path); err == nil || !strings.Contains(err.Error(), "megapixel") {
		t.Fatalf("dimension guard: %v", err)
	}
}
