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
