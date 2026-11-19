package imagepreview

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func kittyPixelRows(id, columns, rows int, pixels image.Image) (string, []string, error) {
	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, pixels); err != nil {
		return "", nil, err
	}
	opts := kitty.Options{
		Action: kitty.TransmitAndPut, ID: id, PlacementID: 1,
		Transmission: kitty.Direct, Format: kitty.PNG, Quiet: 2,
		VirtualPlacement: true, Columns: columns, Rows: rows, DoNotMoveCursor: true,
	}
	var upload strings.Builder
	const chunkBytes = kitty.MaxChunkSize / 4 * 3
	upload.Grow(base64.StdEncoding.EncodedLen(encoded.Len()) + 20*(encoded.Len()/chunkBytes+1) + 128)
	var payload [kitty.MaxChunkSize]byte
	options := strings.Join(opts.Options(), ",")
	for data := encoded.Bytes(); len(data) > 0; {
		n := min(len(data), chunkBytes)
		base64.StdEncoding.Encode(payload[:], data[:n])
		data = data[n:]
		more := "m=0"
		if len(data) > 0 {
			more = "m=1"
		}
		upload.WriteString("\x1b_G")
		upload.WriteString(options)
		upload.WriteByte(',')
		upload.WriteString(more)
		upload.WriteByte(';')
		upload.Write(payload[:base64.StdEncoding.EncodedLen(n)])
		upload.WriteString("\x1b\\")
		options = "q=2"
	}
	result := make([]string, rows)
	for y := 0; y < rows; y++ {
		var line strings.Builder
		// Indexed foreground plus the third diacritic keeps IDs exact even when
		// the workbench renderer uses a 256-color profile.
		fmt.Fprintf(&line, "\x1b[38;5;%dm", id&255)
		for x := 0; x < columns; x++ {
			line.WriteRune(kitty.Placeholder)
			line.WriteRune(kitty.Diacritic(y))
			line.WriteRune(kitty.Diacritic(x))
			line.WriteRune(kitty.Diacritic(id >> 24))
		}
		line.WriteString("\x1b[0m")
		result[y] = line.String()
	}
	return upload.String(), result, nil
}

func DeleteKitty(id int) string {
	return ansi.KittyGraphics(nil, fmt.Sprintf("a=d,d=I,i=%d,q=2", id))
}
