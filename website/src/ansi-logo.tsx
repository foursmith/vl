import { projectWordmark, restingPose, wordmarkAspect } from "./ansi-geometry"

function paintRow(cells: number[]): string {
  const stops: string[] = []
  for (let start = 0; start < cells.length;) {
    const opacity = cells[start]
    let end = start + 1
    while (end < cells.length && cells[end] === opacity) end++
    const color = opacity ? `color-mix(in srgb, var(--color-accent) ${opacity}%, transparent)` : "transparent"
    stops.push(`${color} ${start * 100 / cells.length}% ${end * 100 / cells.length}%`)
    start = end
  }
  return stops.length === 1 ? "none" : `linear-gradient(to right, ${stops.join(", ")})`
}

const firstFrame = projectWordmark(restingPose.yaw, restingPose.pitch).map(paintRow)

const sidebarRows = [
  "w-7/12", "w-5/12", "w-1/2", "w-2/3", "ms-2.5 w-1/3", "ms-2.5 w-5/12", "w-1/2",
]

const homeShortcuts = [
  { command: "Show All Commands", keys: "Ctrl+P" },
  { command: "Go to File", keys: "Ctrl+G" },
  { command: "Open Recent Workspace", keys: "Ctrl+R" },
  { command: "Quit", keys: "Ctrl+Q" },
]

function TerminalSidebar() {
  return (
    <aside class="row-span-2 grid auto-rows-[1.25rem] content-start bg-canvas" aria-hidden="true">
      {/* vl reserves the second navigation row for badges. */}
      <div class="row-span-2 grid grid-rows-2">
        <div class="flex items-center justify-between px-2.5">
          <span class="size-1.5 rounded-xs bg-accent/65" />
          <span class="size-1.5 rounded-xs bg-[#33363b]" />
          <span class="size-1.5 rounded-xs bg-[#33363b]" />
          <span class="size-1.5 rounded-xs bg-[#33363b]" />
          <span class="size-1.5 rounded-xs bg-[#33363b]" />
        </div>
      </div>
      <div class="flex items-center px-2.5">
        <span class="h-1 w-2/5 rounded-full bg-accent/28" />
      </div>
      {sidebarRows.map((shape) => (
        <div class="flex items-center px-2.5">
          <span class={`h-1 rounded-full bg-[#30343a] ${shape}`} />
        </div>
      ))}
    </aside>
  )
}

export function AnsiTerminal() {
  let stage!: HTMLDivElement
  const rows: HTMLSpanElement[] = []
  let pose = { ...restingPose }
  let drag: { pointer: number; x: number; y: number } | undefined

  function draw() {
    projectWordmark(pose.yaw, pose.pitch).forEach((cells, index) => {
      rows[index].style.backgroundImage = paintRow(cells)
    })
  }

  function rotate(yaw: number, pitch: number) {
    pose = {
      yaw: (pose.yaw + yaw) % (2 * Math.PI),
      pitch: Math.min(1.45, Math.max(-1.45, pose.pitch + pitch)),
    }
    draw()
  }

  function startDrag(event: PointerEvent) {
    if (event.button !== 0 || drag) return
    stage.setPointerCapture(event.pointerId)
    drag = { pointer: event.pointerId, x: event.clientX, y: event.clientY }
  }

  function moveDrag(event: PointerEvent) {
    if (drag?.pointer !== event.pointerId) return
    const sensitivity = 3.9 / stage.clientWidth
    rotate((event.clientX - drag.x) * sensitivity, (drag.y - event.clientY) * sensitivity)
    drag.x = event.clientX
    drag.y = event.clientY
  }

  function endDrag(event: PointerEvent) {
    if (drag?.pointer === event.pointerId) drag = undefined
  }

  function rotateWithKey(event: KeyboardEvent) {
    switch (event.key) {
      case "ArrowLeft": rotate(-.16, 0); break
      case "ArrowRight": rotate(.16, 0); break
      case "ArrowUp": rotate(0, .16); break
      case "ArrowDown": rotate(0, -.16); break
      default: return
    }
    event.preventDefault()
  }

  return (
    <div class="@container overflow-hidden bg-canvas font-mono">
      <div class="grid grid-cols-[minmax(0,min(28%,12rem))_minmax(0,1fr)] bg-workspace">
        <TerminalSidebar />
        <div
          ref={stage}
          class="@container grid min-h-[clamp(10.75rem,37cqi,26rem)] min-w-0 touch-none cursor-grab place-items-center px-3 py-4 focus-visible:-outline-offset-2 active:cursor-grabbing sm:px-6"
          role="group"
          aria-label="3D wordmark. Drag or use the arrow keys to rotate."
          tabindex={0}
          onPointerDown={startDrag}
          onPointerMove={moveDrag}
          onLostPointerCapture={endDrag}
          onKeyDown={rotateWithKey}
        >
          <h1
            id="hero-title"
            class="grid w-[min(38rem,74cqi)]"
            style={{
              "aspect-ratio": wordmarkAspect,
              "grid-template-rows": `repeat(${firstFrame.length}, minmax(0, 1fr))`,
            }}
            aria-label="VL"
          >
            {firstFrame.map((background, index) => (
              <span ref={(row) => { rows[index] = row }} style={{ "background-image": background }} aria-hidden="true" />
            ))}
          </h1>
        </div>
        <div class="col-start-2 grid justify-items-center px-3 pb-6 text-xs leading-snug text-muted sm:px-6 sm:pb-8 sm:text-sm">
          <p class="sm:mb-4">Drag to rotate</p>
          <dl class="hidden w-full max-w-[44ch] grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-5 gap-y-1 sm:grid" aria-label="vl Home shortcuts">
            {homeShortcuts.map(({ command, keys }) => (
              <div class="contents">
                <dt>{command}</dt>
                <dd class="text-end">
                  <kbd class="rounded-none border-0 bg-line/55 px-1 py-0 text-[inherit] text-inherit">{keys}</kbd>
                </dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </div>
  )
}
