import type { Locale } from "./locales"
import { paintRow, projectWordmark, restingPose, wordmarkAspect } from "./ansi-geometry"

const firstFrame = projectWordmark(restingPose.yaw, restingPose.pitch).map(paintRow)

const sidebarRows = [
  "w-7/12", "w-5/12", "w-1/2", "w-2/3", "ms-2.5 w-1/3", "ms-2.5 w-5/12", "w-1/2",
]

const homeShortcuts = [
  { command: { en: "Show All Commands", "zh-CN": "打开命令面板" }, keys: "Ctrl+P" },
  { command: { en: "Go to File", "zh-CN": "转到文件" }, keys: "Ctrl+G" },
  { command: { en: "Open Recent Workspace", "zh-CN": "打开最近的工作区" }, keys: "Ctrl+R" },
  { command: { en: "Quit", "zh-CN": "退出" }, keys: "Ctrl+Q" },
]

const terminalLabels = {
  en: {
    rotation: "3D wordmark. Drag or use the arrow keys to rotate.",
    drag: "Drag to rotate",
    shortcuts: "vl Home shortcuts",
  },
  "zh-CN": {
    rotation: "立体标志。拖动或使用方向键旋转。",
    drag: "拖动旋转",
    shortcuts: "vl 首页快捷键",
  },
}

function TerminalSidebar() {
  return (
    <aside class="row-span-2 grid auto-rows-[1.25rem] content-start bg-canvas" aria-hidden="true">
      {/* vl reserves the second navigation row for badges. */}
      <div class="row-span-2 grid grid-rows-2">
        <div class="flex items-center justify-between px-2.5">
          <span class="size-1.5 rounded-xs bg-accent/65" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
        </div>
      </div>
      <div class="flex items-center px-2.5">
        <span class="h-1 w-2/5 rounded-full bg-accent/28" />
      </div>
      {sidebarRows.map((shape) => (
        <div class="flex items-center px-2.5">
        <span class={`h-1 rounded-full bg-terminal-line ${shape}`} />
        </div>
      ))}
    </aside>
  )
}

export function AnsiTerminal(props: { locale: Locale }) {
  const copy = terminalLabels[props.locale]

  return (
    <div class="@container overflow-hidden bg-canvas font-mono">
      <div class="grid grid-cols-[minmax(0,min(28%,12rem))_minmax(0,1fr)] bg-workspace">
        <TerminalSidebar />
        <div
          data-wordmark
          class="@container grid min-h-[clamp(10.75rem,37cqi,26rem)] min-w-0 touch-none cursor-grab place-items-center px-3 py-4 focus-visible:-outline-offset-2 active:cursor-grabbing sm:px-6"
          role="group"
          aria-label={copy.rotation}
          tabindex={0}
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
            {firstFrame.map((background) => (
              <span style={{ "background-image": background }} aria-hidden="true" />
            ))}
          </h1>
        </div>
        <div class="col-start-2 grid justify-items-center px-3 pb-6 text-xs leading-snug text-muted sm:px-6 sm:pb-8 sm:text-sm">
          <p class="sm:mb-4">{copy.drag}</p>
          <dl class="hidden w-full max-w-[44ch] grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-5 gap-y-1 sm:grid" aria-label={copy.shortcuts}>
            {homeShortcuts.map(({ command, keys }) => (
              <div class="contents">
                <dt>{command[props.locale]}</dt>
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
