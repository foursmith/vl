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

func TestZoomRegionCropsRasterAndSVG(t *testing.T) {
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
	vector, err := openSVG("crop.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20"><path d="M0 0H20V20H0Z" fill="red"/><path d="M20 0H40V20H20Z" fill="blue"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range []*Image{testRaster(raster), vector} {
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
}
