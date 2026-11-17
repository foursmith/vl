package theme

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

// Pinned VS Code registrations, extracted from source rather than approximated
// from the appearance of one bundled theme. See tools/generate-defaults.cjs.
//
//go:embed defaults.json
var defaultsJSON []byte

type colorDefault struct {
	Hex      string                   `json:"hex"`
	Ref      string                   `json:"ref"`
	Variants map[string]*colorDefault `json:"variants"`
	Op       string                   `json:"op"`
	Args     []json.RawMessage        `json:"args"`
}

var registeredColors = func() map[string]*colorDefault {
	var data struct {
		Colors map[string]*colorDefault `json:"colors"`
	}
	if err := json.Unmarshal(defaultsJSON, &data); err != nil {
		panic(err)
	}
	return data.Colors
}()

// Color returns the VS Code color before terminal composition. Empty means
// undefined/null: the component inherits its parent instead of painting a color.
// References and transforms resolve against THIS theme, including its overrides.
func (t *Theme) Color(id string) string {
	if c := t.colors[id]; c != "" {
		return c
	}
	return t.resolveDefault(registeredColors[id])
}
func (t *Theme) resolveDefault(d *colorDefault) string {
	if d == nil {
		return ""
	}
	if d.Hex != "" {
		return normalizeHex(d.Hex)
	}
	if d.Ref != "" {
		return t.Color(d.Ref)
	}
	if d.Variants != nil {
		kind := map[string]string{"vs": "light", "vs-dark": "dark", "hc-black": "hcDark", "hc-light": "hcLight"}[t.Type]
		return t.resolveDefault(d.Variants[kind])
	}
	arg := func(i int) *colorDefault {
		var a *colorDefault
		if err := json.Unmarshal(d.Args[i], &a); err != nil {
			panic(err)
		}
		return a
	}
	switch d.Op {
	case "transparent":
		var factor float64
		if err := json.Unmarshal(d.Args[1], &factor); err != nil {
			panic(err)
		}
		return WithOpacity(t.resolveDefault(arg(0)), factor)
	case "oneOf":
		for i := range d.Args {
			if c := t.resolveDefault(arg(i)); c != "" {
				return c
			}
		}
	default:
		panic(fmt.Sprintf("unsupported registered color operation %q", d.Op))
	}
	return ""
}
func WithOpacity(color string, factor float64) string {
	if color == "" {
		return ""
	}
	color = normalizeHex(color)
	alpha := float64(255)
	if len(color) == 9 {
		b, _ := hex.DecodeString(color[7:])
		alpha = float64(b[0])
	}
	return normalizeHex(fmt.Sprintf("%s%02X", color[:7], int(math.Round(alpha*factor))))
}

// Composite is the terminal boundary: keep RGBA and undefined colors intact
// until the actual surface behind them is known.
func Composite(color, bg string) string {
	if normalizeHex(color) == "" || normalizeHex(bg) == "" {
		return color
	}
	return opaque(color, bg)
}
func DecorationBadge(color, bg string) string { return Composite(WithOpacity(color, .75), bg) }

// The raw registry remains unchanged by terminal-only popup edge adaptation.
func (t *Theme) colorOr(id, parent string) string {
	if color := t.Color(id); color != "" {
		return color
	}
	return parent
}
