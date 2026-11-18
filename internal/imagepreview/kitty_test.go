package imagepreview

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKittyUploadCarriesRealPixelsInBoundedChunks(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 128, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 128; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(x*17 + y*37), G: uint8(x*73 + y*11), B: uint8(x * y), A: 255})
		}
	}
	im := testRaster(pixels)
	upload, lines, err := im.KittyRegion(0x12000056, im.ViewRegion(16, 6, 8, 16, 1, .5, .5))
	if err != nil {
		t.Fatal(err)
	}
	chunks := kittyChunks.FindAllStringSubmatch(upload, -1)
	if len(chunks) < 2 {
		t.Fatal("test image did not exercise chunking")
	}
	for _, chunk := range chunks {
		if len(chunk[2]) > 4096 || len(chunk[2])%4 != 0 {
			t.Fatal("invalid chunk size")
		}
	}
	first := chunks[0][1]
	for _, option := range []string{"a=T", "U=1", "f=100", "q=2", "C=1", "i=301989974", "c=16", "r=6"} {
		if !strings.Contains(first, option) {
			t.Fatalf("missing %s: %s", option, first)
		}
	}
	decoded := decodeKittyUpload(t, upload)
	if decoded.Bounds() != pixels.Bounds() {
		t.Fatalf("lost image resolution: %v", decoded.Bounds())
	}
	for _, line := range lines {
		if ansi.StringWidth(line) != 16 {
			t.Fatalf("placeholder width: %d", ansi.StringWidth(line))
		}
	}
}

func TestSVGRenderAndHighResolutionPlacement(t *testing.T) {
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20">
 <defs><linearGradient id="g"><stop stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>
 <clipPath id="c"><circle cx="10" cy="10" r="8"/></clipPath></defs>
 <rect width="20" height="20" fill="url(#g)" clip-path="url(#c)"/>
 <text x="21" y="14" font-family="sans-serif" font-size="12">A</text>
 </svg>`)
	im, err := openSVG("test.svg", data)
	if err != nil {
		t.Fatal(err)
	}
	if im.Format != "svg" || im.Bounds().Dx() != 40 || im.Bounds().Dy() != 20 {
		t.Fatal("incorrect SVG dimensions")
	}
	pixels := decodeRegion(t, im, Region{Width: 40, Height: 20, Scale: 1, Columns: 5, Rows: 2})
	_, _, _, corner := pixels.At(0, 0).RGBA()
	red, _, blue, alpha := pixels.At(10, 10).RGBA()
	if corner != 0 || red == 0 || blue == 0 || alpha == 0 {
		t.Fatal("SVG transparency, gradient or clip lost")
	}
	textPixels := 0
	for y := 0; y < 20; y++ {
		for x := 21; x < 40; x++ {
			_, _, _, a := pixels.At(x, y).RGBA()
			if a != 0 {
				textPixels++
			}
		}
	}
	if textPixels == 0 {
		t.Fatal("SVG text was not rendered")
	}
	region := im.ViewRegion(80, 20, 8, 16, 1, .5, .5)
	upload, _, err := im.KittyRegion(42, region)
	if err != nil {
		t.Fatal(err)
	}
	raster := decodeKittyUpload(t, upload)
	if raster.Bounds().Dx() <= 40 || raster.Bounds().Dy() <= 20 {
		t.Fatalf("SVG did not rerender at display resolution: %v", raster.Bounds())
	}
}

func TestSVGInvalidAndOversized(t *testing.T) {
	for _, data := range []string{
		"<svg", "<html/>",
		`<svg xmlns="http://www.w3.org/2000/svg" width="1000000" height="1000000"><rect width="10" height="10"/></svg>`,
	} {
		if _, err := openSVG("bad.svg", []byte(data)); err == nil {
			t.Fatalf("accepted invalid/oversized SVG %s", data)
		}
	}
}
