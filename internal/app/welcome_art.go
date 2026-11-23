package app

import (
	"math"
	"strings"

	"github.com/foursmith/vl/internal/theme"
)

type welcomePoint struct{ x, y, z float64 }

func (p welcomePoint) sub(q welcomePoint) welcomePoint {
	return welcomePoint{p.x - q.x, p.y - q.y, p.z - q.z}
}

func (p welcomePoint) cross(q welcomePoint) welcomePoint {
	return welcomePoint{p.y*q.z - p.z*q.y, p.z*q.x - p.x*q.z, p.x*q.y - p.y*q.x}
}

type welcomeTriangle struct{ a, b, c welcomePoint }

// Source Code Pro Bold capitals, centered at their original equal advances.
var welcomeMesh = append(
	extrudeWelcomeLetter(
		[][2]float64{
			{-.993, -.88}, {-1.525, .88}, {-1.104, .88}, {-.893, .059}, {-.823, -.219}, {-.748, -.505},
			{-.675, -.219}, {-.599, .059}, {-.391, .88}, {.013, .88}, {-.516, -.88},
		},
		[][3]int{
			{0, 1, 2}, {0, 2, 3}, {0, 3, 4}, {0, 4, 5}, {10, 0, 5}, {10, 5, 6}, {10, 6, 7}, {10, 7, 8}, {8, 9, 10},
		},
	),
	extrudeWelcomeLetter(
		[][2]float64{
			{.343, -.88}, {.343, .88}, {.740, .88}, {.740, -.545}, {1.522, -.545}, {1.522, -.88},
		},
		[][3]int{{0, 1, 2}, {0, 2, 3}, {5, 0, 3}, {3, 4, 5}},
	)...,
)

// The outlines wind clockwise when seen head-on. Inset caps and wider
// shoulders form bevels that catch the light as the letters rotate.
func extrudeWelcomeLetter(outline [][2]float64, caps [][3]int) []welcomeTriangle {
	inset := make([][2]float64, len(outline))
	inward := func(a, b [2]float64) (float64, float64) {
		dx, dy := b[0]-a[0], b[1]-a[1]
		length := math.Hypot(dx, dy)
		return dy / length, -dx / length
	}
	for i, p := range outline {
		ax, ay := inward(outline[(i+len(outline)-1)%len(outline)], p)
		bx, by := inward(p, outline[(i+1)%len(outline)])
		offset := .024 / (1 + ax*bx + ay*by)
		inset[i] = [2]float64{p[0] + (ax+bx)*offset, p[1] + (ay+by)*offset}
	}
	point := func(p [2]float64, z float64) welcomePoint { return welcomePoint{p[0], p[1], z} }
	var mesh []welcomeTriangle
	for _, cap := range caps {
		a, b, c := inset[cap[0]], inset[cap[1]], inset[cap[2]]
		mesh = append(mesh, welcomeTriangle{point(a, .33), point(c, .33), point(b, .33)})
		mesh = append(mesh, welcomeTriangle{point(a, -.33), point(b, -.33), point(c, -.33)})
	}
	for i, a := range outline {
		j := (i + 1) % len(outline)
		b := outline[j]
		strip := [4][2]welcomePoint{
			{point(inset[i], .33), point(inset[j], .33)},
			{point(a, .275), point(b, .275)},
			{point(a, -.275), point(b, -.275)},
			{point(inset[i], -.33), point(inset[j], -.33)},
		}
		for k := 1; k < len(strip); k++ {
			p, q := strip[k-1][0], strip[k-1][1]
			r, s := strip[k][0], strip[k][1]
			mesh = append(mesh, welcomeTriangle{p, q, r}, welcomeTriangle{q, s, r})
		}
	}
	return mesh
}

type welcomeOrbit struct {
	yaw, pitch float64
	dragging   bool
	x, y       int
}

const welcomeDefaultPitch = -39 * math.Pi / 180

func (o *welcomeOrbit) rotate(dx, dy float64) {
	o.yaw = math.Remainder(o.yaw+dx, 2*math.Pi)
	o.pitch = min(1.45-welcomeDefaultPitch, max(-1.45-welcomeDefaultPitch, o.pitch+dy))
}

// Use a fixed character-cell aspect so a terminal pixel-size reply cannot
// change the welcome art after the first frame.
const welcomeCellAspect = .5

func (m *Model) renderWelcomeArt(width, height int) []string {
	ramp := [...]string{"░", "░", "░", "▒", "▒", "▓", "▓", "▓", "█"}
	ratio := welcomeCellAspect
	const samplesPerEdge = 4
	const samplesPerCell = samplesPerEdge * samplesPerEdge
	sampleWidth, sampleHeight := width*samplesPerEdge, height*samplesPerEdge
	// Leave room for the depth of the letters even when they turn edge-on.
	scale := samplesPerEdge * min(float64(width-2)/3.56, float64(height-1)/(2.32*ratio))
	sy, cy := math.Sincos(m.welcomeOrbit.yaw)
	sp, cp := math.Sincos(m.welcomeOrbit.pitch + welcomeDefaultPitch)
	project := func(p welcomePoint) welcomePoint {
		x, z := cy*p.x+sy*p.z, -sy*p.x+cy*p.z
		y := cp*p.y - sp*z
		return welcomePoint{float64(sampleWidth)*.5 + x*scale, float64(sampleHeight)*.5 - y*scale*ratio, sp*p.y + cp*z}
	}
	shades := make([]byte, sampleWidth*sampleHeight)
	depth := make([]float64, len(shades))
	for _, face := range welcomeMesh {
		a, b, c := project(face.a), project(face.b), project(face.c)
		rasterWelcomeTriangle(sampleWidth, sampleHeight, ratio, scale, a, b, c, depth, shades, len(ramp))
	}

	ink := m.colors.Accent
	if ink == "" {
		ink = m.colors.Foreground
	}
	var tones [len(ramp)]string
	for i := range tones {
		opacity := .56 + .44*float64(i)/float64(len(tones)-1)
		tones[i] = theme.Composite(theme.WithOpacity(ink, opacity), m.colors.Background)
	}
	lines := make([]string, height)
	type cell struct{ tone, glyph int }
	cells := make([]cell, width)
	for y := range lines {
		var line strings.Builder
		for x := range cells {
			var lit, covered int
			for dy := range samplesPerEdge {
				start := (y*samplesPerEdge+dy)*sampleWidth + x*samplesPerEdge
				for _, shade := range shades[start : start+samplesPerEdge] {
					if shade != 0 {
						lit += int(shade)
						covered++
					}
				}
			}
			cells[x] = cell{}
			if covered > 0 {
				// An edge cell gets a sparser glyph without dimming the letter's ink.
				light := (lit + covered/2) / covered
				coverage := (covered*len(ramp) + samplesPerCell - 1) / samplesPerCell
				cells[x] = cell{tone: light, glyph: min(light, coverage)}
			}
		}
		for start := 0; start < len(cells); {
			c := cells[start]
			end := start + 1
			for end < len(cells) && cells[end] == c {
				end++
			}
			if c.glyph == 0 {
				line.WriteString(strings.Repeat(" ", end-start))
			} else {
				line.WriteString(theme.Style(tones[c.tone-1], m.colors.Background).Render(strings.Repeat(ramp[c.glyph-1], end-start)))
			}
			start = end
		}
		lines[y] = line.String()
	}
	return lines
}

func rasterWelcomeTriangle(width, height int, ratio, scale float64, a, b, c welcomePoint, depth []float64, shades []byte, steps int) {
	ab, ac := b.sub(a), c.sub(a)
	area := ab.x*ac.y - ab.y*ac.x
	if area >= -1e-7 {
		return
	}
	normal := welcomePoint{ab.x / scale, -ab.y / (scale * ratio), ab.z}.cross(
		welcomePoint{ac.x / scale, -ac.y / (scale * ratio), ac.z},
	)
	length := math.Sqrt(normal.x*normal.x + normal.y*normal.y + normal.z*normal.z)
	light := max(0, (.78*normal.x+.42*normal.y+.46*normal.z)/length)
	base := .26 + .68*light
	left, right := max(0, int(math.Floor(min(a.x, b.x, c.x)))), min(width-1, int(math.Ceil(max(a.x, b.x, c.x))))
	top, bottom := max(0, int(math.Floor(min(a.y, b.y, c.y)))), min(height-1, int(math.Ceil(max(a.y, b.y, c.y))))
	for y := top; y <= bottom; y++ {
		dy := float64(y) + .5 - a.y
		for x := left; x <= right; x++ {
			dx := float64(x) + .5 - a.x
			u := (dx*ac.y - dy*ac.x) / area
			v := (ab.x*dy - ab.y*dx) / area
			if u < -1e-7 || v < -1e-7 || u+v > 1+1e-7 {
				continue
			}
			i := y*width + x
			z := a.z + u*ab.z + v*ac.z
			if shades[i] != 0 && z <= depth[i] {
				continue
			}
			depth[i] = z
			// A gentle depth gradient keeps broad faces from reading as flat stripes.
			shade := min(1, max(0, base+.11*z-.05*float64(y)/float64(height)))
			shades[i] = 1 + byte(math.Round(shade*float64(steps-1)))
		}
	}
}
