package imagepreview

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var imageClient = &http.Client{Timeout: 10 * time.Second}

// OpenURL downloads one HTTP image and applies the same limits as a local preview.
func OpenURL(ctx context.Context, location string) (*Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")
	response, err := imageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image request returned %s", response.Status)
	}
	if response.ContentLength > MaxBytes {
		return nil, fmt.Errorf("image exceeds 32 MiB preview limit")
	}
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentType == "text/html" {
		return nil, fmt.Errorf("image URL returned an HTML page")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
	if err != nil {
		return nil, err
	}

	path := response.Request.URL.Path
	if contentType == "image/svg+xml" {
		path = "remote.svg"
	} else if strings.EqualFold(filepath.Ext(path), ".svg") && strings.HasPrefix(contentType, "image/") {
		path = "" // Raster decoders detect the format from the response bytes.
	}
	return Decode(path, data)
}
