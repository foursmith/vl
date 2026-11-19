package imagepreview

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
	"testing"
)

func decodeRegion(t *testing.T, im *Image, region Region) image.Image {
	t.Helper()
	upload, _, err := im.KittyRegion(42, region)
	if err != nil {
		t.Fatal(err)
	}
	return decodeKittyUpload(t, upload)
}

var kittyChunks = regexp.MustCompile("\x1b_G([^;]*);([^\x1b]*)\x1b\\\\")

func decodeKittyUpload(t *testing.T, upload string) image.Image {
	t.Helper()
	var payload strings.Builder
	for _, chunk := range kittyChunks.FindAllStringSubmatch(upload, -1) {
		payload.WriteString(chunk[2])
	}
	data, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil {
		t.Fatal(err)
	}
	result, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestZoomRegionCropsRaster(t *testing.T) {
	raster := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if x >= 20 {
				c = color.NRGBA{B: 255, A: 255}
			}
			raster.SetNRGBA(x, y, c)
		}
	}
	im := testRaster(raster)
	for _, side := range []float64{0, 1} {
		region := im.ViewRegion(10, 5, 8, 16, 4, side, .5)
		got := decodeRegion(t, im, region)
		if got.Bounds().Dx() > 80 || got.Bounds().Dy() > 80 {
			t.Fatal("zoom allocated outside viewport")
		}
		r, _, b, a := got.At(got.Bounds().Dx()/2, got.Bounds().Dy()/2).RGBA()
		if a == 0 || side == 0 && (r < 60000 || b > 100) || side == 1 && (b < 60000 || r > 100) {
			t.Fatalf("incorrect source crop for %s side %v: %d %d", im.Format, side, r, b)
		}
	}
}

func TestZoomRegionBoundsAtExtremeScale(t *testing.T) {
	im := testRaster(image.NewNRGBA(image.Rect(0, 0, 4000, 2000)))
	region := im.ViewRegion(80, 24, 8, 16, 32, 1, 0)
	if region.X+region.Width > 4000 || region.Y < 0 || region.Columns > 80 || region.Rows > 24 {
		t.Fatal("invalid crop bounds")
	}
	got := decodeRegion(t, im, region)
	if got.Bounds().Dx() > 640 || got.Bounds().Dy() > 384 {
		t.Fatal("allocated full zoomed image")
	}
	wide := im.ViewRegion(255, 6, 32, 32, 64, 1, 0)
	if got := decodeRegion(t, im, wide).Bounds().Size(); got != image.Pt(4096, 96) {
		t.Errorf("large viewport uploaded %v, want 4096×96", got)
	}
}

func TestKittyRegionScalesRasterBySourceSize(t *testing.T) {
	const width, height = 160, 128
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		size, width, height int
	}{
		{500_000, 160, 128},
		{500_001, 160, 128},
		{781_250, 128, 102},
		{1_000_000, 113, 91},
		{2_000_000, 80, 64},
	} {
		// PNG decoding ends at IEND. Padding varies the file size without
		// changing the source raster.
		source := make([]byte, tc.size)
		copy(source, pngData.Bytes())
		im, err := Decode(t.Context(), "preview.png", source)
		if err != nil {
			t.Fatal(err)
		}
		region := im.ViewRegion(48, 12, 8, 16, 1, .5, .5)
		upload, _, err := im.KittyRegion(42, region)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(upload, ",c=20,r=8,") {
			t.Fatalf("%d-byte source changed the cell placement", tc.size)
		}
		if got := decodeKittyUpload(t, upload).Bounds().Size(); got != image.Pt(tc.width, tc.height) {
			t.Errorf("%d-byte source uploaded %v, want %d×%d", tc.size, got, tc.width, tc.height)
		}
	}
}
