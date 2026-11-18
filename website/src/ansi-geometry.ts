type Point = [x: number, y: number, z: number]
type OutlinePoint = [x: number, y: number]
type Triangle = [Point, Point, Point]
type Indices = [number, number, number]

const columns = 60
const rows = 17
const cellAspect = 0.6
const samplesPerEdge = 4
const samplesPerCell = samplesPerEdge ** 2
const sampleWidth = columns * samplesPerEdge
const sampleHeight = rows * samplesPerEdge
const scale = samplesPerEdge * Math.min((columns - 2) / 3.56, (rows - 1) / (2.32 * cellAspect))
const ramp = ["░", "░", "░", "▒", "▒", "▓", "▓", "▓", "█"]

export const restingPose = { yaw: Math.PI / 15, pitch: 0 }

// Port of internal/app/welcome_art.go. Use the same Source Code Pro Bold
// outline and cap triangulation as the native welcome screen.
const letters = [
  extrudeLetter(
    [
      [-.993, -.88], [-1.525, .88], [-1.104, .88], [-.893, .059], [-.823, -.219], [-.748, -.505],
      [-.675, -.219], [-.599, .059], [-.391, .88], [.013, .88], [-.516, -.88],
    ],
    [[0, 1, 2], [0, 2, 3], [0, 3, 4], [0, 4, 5], [10, 0, 5], [10, 5, 6], [10, 6, 7], [10, 7, 8], [8, 9, 10]],
  ),
  extrudeLetter(
    [[.343, -.88], [.343, .88], [.740, .88], [.740, -.545], [1.522, -.545], [1.522, -.88]],
    [[0, 1, 2], [0, 2, 3], [5, 0, 3], [3, 4, 5]],
  ),
]

function extrudeLetter(outline: OutlinePoint[], caps: Indices[]): Triangle[] {
  function inward([ax, ay]: OutlinePoint, [bx, by]: OutlinePoint): OutlinePoint {
    const dx = bx - ax
    const dy = by - ay
    const length = Math.hypot(dx, dy)
    return [dy / length, -dx / length]
  }

  const inset = outline.map((point, i): OutlinePoint => {
    const [ax, ay] = inward(outline.at(i - 1)!, point)
    const [bx, by] = inward(point, outline[(i + 1) % outline.length])
    const offset = .024 / (1 + ax * bx + ay * by)
    return [point[0] + (ax + bx) * offset, point[1] + (ay + by) * offset]
  })

  const mesh: Triangle[] = []
  const point = ([x, y]: OutlinePoint, z: number): Point => [x, y, z]
  for (const [i, j, k] of caps) {
    const [a, b, c] = [inset[i], inset[j], inset[k]]
    mesh.push([point(a, .33), point(c, .33), point(b, .33)])
    mesh.push([point(a, -.33), point(b, -.33), point(c, -.33)])
  }
  for (const [i, a] of outline.entries()) {
    const j = (i + 1) % outline.length
    const b = outline[j]
    const strip = [
      [point(inset[i], .33), point(inset[j], .33)],
      [point(a, .275), point(b, .275)],
      [point(a, -.275), point(b, -.275)],
      [point(inset[i], -.33), point(inset[j], -.33)],
    ]
    for (let k = 1; k < strip.length; k++) {
      const [p, q] = strip[k - 1]
      const [r, s] = strip[k]
      mesh.push([p, q, r], [q, s, r])
    }
  }
  return mesh
}

function rasterTriangle(a: Point, b: Point, c: Point, depth: Float64Array, shades: Uint8Array) {
  const [abX, abY, abZ] = [b[0] - a[0], b[1] - a[1], b[2] - a[2]]
  const [acX, acY, acZ] = [c[0] - a[0], c[1] - a[1], c[2] - a[2]]
  const area = abX * acY - abY * acX
  if (area >= -1e-7) return

  const [ux, uy, vx, vy] = [abX / scale, -abY / (scale * cellAspect), acX / scale, -acY / (scale * cellAspect)]
  const [nx, ny, nz] = [uy * acZ - abZ * vy, abZ * vx - ux * acZ, ux * vy - uy * vx]
  const light = Math.max(0, (.78 * nx + .42 * ny + .46 * nz) / Math.hypot(nx, ny, nz))
  const base = .26 + .68 * light
  const left = Math.max(0, Math.floor(Math.min(a[0], b[0], c[0])))
  const right = Math.min(sampleWidth - 1, Math.ceil(Math.max(a[0], b[0], c[0])))
  const top = Math.max(0, Math.floor(Math.min(a[1], b[1], c[1])))
  const bottom = Math.min(sampleHeight - 1, Math.ceil(Math.max(a[1], b[1], c[1])))

  for (let y = top; y <= bottom; y++) {
    const dy = y + .5 - a[1]
    for (let x = left; x <= right; x++) {
      const dx = x + .5 - a[0]
      const u = (dx * acY - dy * acX) / area
      const v = (abX * dy - abY * dx) / area
      if (u < -1e-7 || v < -1e-7 || u + v > 1 + 1e-7) continue

      const index = y * sampleWidth + x
      const z = a[2] + u * abZ + v * acZ
      if (shades[index] !== 0 && z <= depth[index]) continue
      depth[index] = z
      const shade = Math.min(1, Math.max(0, base + .11 * z - .05 * y / sampleHeight))
      shades[index] = 1 + Math.round(shade * (ramp.length - 1))
    }
  }
}

export function projectWordmark(yaw: number, pitch: number) {
  const cy = Math.cos(yaw)
  const sy = Math.sin(yaw)
  const cp = Math.cos(pitch)
  const sp = Math.sin(pitch)

  function project([x, y, z]: Point): Point {
    const horizontal = cy * x + sy * z
    const rotatedZ = -sy * x + cy * z
    const vertical = cp * y - sp * rotatedZ
    return [sampleWidth * .5 + horizontal * scale, sampleHeight * .5 - vertical * scale * cellAspect, sp * y + cp * rotatedZ]
  }

  const projected = letters.map((mesh) => rasterizeLetter(mesh, project))
  // The letters share vertical bounds so their glyph rows remain aligned.
  const top = Math.min(...projected.map((letter) => letter.top))
  const bottom = Math.max(...projected.map((letter) => letter.bottom))
  return projected.map(({ tones, glyphs, left, right }) =>
    ramp.map((_, layer) => {
      let text = ""
      for (let row = top; row < bottom; row++) {
        if (row > top) text += "\n"
        for (let column = left; column < right; column++) {
          const index = row * columns + column
          text += tones[index] === layer + 1 ? ramp[glyphs[index] - 1] : " "
        }
      }
      return text
    }),
  )
}

function rasterizeLetter(mesh: Triangle[], project: (point: Point) => Point) {
  const depth = new Float64Array(sampleWidth * sampleHeight)
  const shades = new Uint8Array(depth.length)
  for (const [a, b, c] of mesh) rasterTriangle(project(a), project(b), project(c), depth, shades)

  const tones = new Uint8Array(columns * rows)
  const glyphs = new Uint8Array(tones.length)
  let left = columns
  let right = 0
  let top = rows
  let bottom = 0
  for (let row = 0; row < rows; row++) {
    for (let column = 0; column < columns; column++) {
      let lit = 0
      let covered = 0
      for (let dy = 0; dy < samplesPerEdge; dy++) {
        const start = (row * samplesPerEdge + dy) * sampleWidth + column * samplesPerEdge
        for (let dx = 0; dx < samplesPerEdge; dx++) {
          const shade = shades[start + dx]
          if (shade === 0) continue
          lit += shade
          covered++
        }
      }
      if (covered === 0) continue
      const index = row * columns + column
      const tone = Math.round(lit / covered)
      const coverage = Math.ceil(covered * ramp.length / samplesPerCell)
      tones[index] = tone
      glyphs[index] = Math.min(tone, coverage)
      left = Math.min(left, column)
      right = Math.max(right, column + 1)
      top = Math.min(top, row)
      bottom = row + 1
    }
  }
  return { tones, glyphs, left, right, top, bottom }
}
