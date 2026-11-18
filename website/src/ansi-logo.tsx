import type { JSX } from "@solidjs/web"
import { createSignal, onSettled } from "solid-js"
import { projectWordmark, restingPose } from "./ansi-geometry"

const firstFrame = projectWordmark(restingPose.yaw, restingPose.pitch)

function AnsiFrame(props: { frame: string[]; onLayer: (layer: HTMLSpanElement, index: number) => void }) {
  return (
    <span class="ansi-frame justify-self-center" aria-hidden="true">
      {props.frame.map((text, index) => (
        <span
          ref={(layer) => { props.onLayer(layer, index) }}
          class="col-start-1 row-start-1 whitespace-pre"
          style={{ "--ink-opacity": `${56 + 44 * index / (props.frame.length - 1)}%` }}
        >{text}</span>
      ))}
    </span>
  )
}

const sidebarRows = [
  "w-7/12", "w-5/12", "w-1/2", "w-2/3", "ms-2.5 w-1/3", "ms-2.5 w-5/12", "w-1/2",
]

function TerminalSidebar() {
  return (
    <aside class="border-e border-line bg-panel sm:row-span-2" aria-hidden="true">
      <div class="flex h-9 items-center justify-between px-2.5">
        <span class="size-1.5 rounded-xs bg-accent/65" />
        <span class="size-1.5 rounded-xs bg-[#33363b]" />
        <span class="size-1.5 rounded-xs bg-[#33363b]" />
        <span class="size-1.5 rounded-xs bg-[#33363b]" />
        <span class="size-1.5 rounded-xs bg-[#33363b]" />
      </div>
      <div class="flex h-5 items-center bg-accent/8 px-2.5">
        <span class="h-1 w-2/5 rounded-full bg-accent/28" />
      </div>
      <div class="grid gap-2.5 px-2.5 pt-2.5">
        {sidebarRows.map((shape) => (
          <span class={`h-1 rounded-full bg-[#30343a] ${shape}`} />
        ))}
      </div>
    </aside>
  )
}

export function AnsiTerminal(props: { children: JSX.Element }) {
  const [playing, setPlaying] = createSignal(false)
  let stage!: HTMLDivElement
  const layers: HTMLSpanElement[][] = [[], []]
  let frameRequest = 0
  let inView = false
  let previousTime = 0
  let phase = 0
  let orbit = { ...restingPose }
  let visiblePose = orbit
  let drag: { pointer: number; x: number; y: number } | undefined

  function draw(pose: typeof restingPose) {
    visiblePose = pose
    projectWordmark(pose.yaw, pose.pitch).forEach((frame, letter) => {
      frame.forEach((text, index) => {
        const layer = layers[letter][index]
        if (layer.textContent !== text) layer.textContent = text
      })
    })
  }

  function rotate(yaw: number, pitch: number) {
    phase = 0
    orbit = {
      yaw: (visiblePose.yaw + yaw) % (2 * Math.PI),
      pitch: Math.min(1.45, Math.max(-1.45, visiblePose.pitch + pitch)),
    }
    draw(orbit)
  }

  function animate(now: number) {
    if (!drag && now - previousTime >= 30) {
      phase += Math.min(now - previousTime, 50) / 1150
      previousTime = now
      draw({
        yaw: orbit.yaw + Math.sin(phase) * .28,
        pitch: orbit.pitch,
      })
    }
    frameRequest = requestAnimationFrame(animate)
  }

  function syncAnimation() {
    cancelAnimationFrame(frameRequest)
    if (playing() && inView) {
      previousTime = performance.now()
      frameRequest = requestAnimationFrame(animate)
    }
  }

  function setMotion(enabled: boolean) {
    setPlaying(enabled)
    syncAnimation()
  }

  onSettled(() => {
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)")
    const syncPreference = () => setMotion(!preference.matches)
    const observer = new IntersectionObserver(([entry]) => {
      inView = entry.isIntersecting
      syncAnimation()
    })
    observer.observe(stage)
    syncPreference()
    preference.addEventListener("change", syncPreference)
    return () => {
      observer.disconnect()
      preference.removeEventListener("change", syncPreference)
      cancelAnimationFrame(frameRequest)
    }
  })

  function startDrag(event: PointerEvent) {
    if (event.button !== 0 || drag) return
    stage.setPointerCapture(event.pointerId)
    drag = { pointer: event.pointerId, x: event.clientX, y: event.clientY }
    orbit = visiblePose
    phase = 0
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
    <div class="@container overflow-hidden rounded-md border border-line bg-inset font-mono">
      <div class="flex min-h-10 items-center gap-2.5 border-b border-line bg-panel ps-4 pe-1 text-xs text-muted">
        <span class="text-accent" aria-hidden="true">&gt;_</span>
        <span>vl .</span>
        <button
          type="button"
          class="ms-auto grid size-8 cursor-pointer place-items-center rounded-sm text-muted hover:bg-line hover:text-accent"
          aria-label={playing() ? "Pause logo animation" : "Play logo animation"}
          title={playing() ? "Pause logo animation" : "Play logo animation"}
          onClick={() => setMotion(!playing())}
        >
          <span
            aria-hidden="true"
            class={playing()
              ? "h-3 w-2.5 border-x-2 border-current"
              : "size-0 border-y-[6px] border-l-[9px] border-y-transparent border-l-current"}
          />
        </button>
      </div>
      <div class="grid grid-cols-[minmax(0,min(28%,12rem))_minmax(0,1fr)]">
        <TerminalSidebar />
        <div
          ref={stage}
          class="ansi-stage @container grid min-h-[clamp(10.75rem,37cqi,26rem)] min-w-0 touch-none cursor-grab place-items-center px-3 py-4 focus-visible:-outline-offset-2 active:cursor-grabbing sm:px-6"
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
            class="grid grid-cols-[22ch_18ch] items-end gap-[clamp(0.5rem,2.5cqi,2rem)] text-[min(1.375rem,2.65cqi)]"
            aria-label="VL"
          >
            {firstFrame.map((frame, letter) => (
              <AnsiFrame frame={frame} onLayer={(layer, index) => { layers[letter][index] = layer }} />
            ))}
          </h1>
        </div>
        <div class="col-span-2 grid items-center gap-4 border-t border-line bg-panel px-4 py-4 sm:col-span-1 sm:col-start-2 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:gap-6">
          {props.children}
        </div>
      </div>
      <div class="flex items-center gap-3 border-t border-line bg-accent/8 pe-3 text-xs leading-[1.8] text-muted" aria-hidden="true">
        <span class="bg-accent/15 px-2.5 font-medium text-accent">NORMAL</span>
        <span>main</span>
        <span class="ms-auto">UTF-8</span>
      </div>
    </div>
  )
}
