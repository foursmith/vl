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
    <aside class="landing-gutter row-span-2 grid auto-rows-[1.25rem] grid-cols-[minmax(0,13rem)] content-start border-e border-line bg-canvas" aria-hidden="true">
      {/* vl reserves the second navigation row for badges. */}
      <div class="row-span-2 grid grid-rows-2">
        <div class="flex items-center justify-between">
          <span class="size-1.5 rounded-xs bg-accent/65" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
          <span class="size-1.5 rounded-xs bg-terminal-icon" />
        </div>
      </div>
      <div class="flex items-center">
        <span class="h-1 w-2/5 rounded-full bg-accent/28" />
      </div>
      {sidebarRows.map((shape) => (
        <div class="flex items-center">
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
      <div class="landing-columns grid bg-workspace">
        <TerminalSidebar />
        <div
          data-wordmark
          class="landing-gutter @container grid min-w-0 touch-none cursor-grab justify-items-center pt-5 focus-visible:-outline-offset-2 active:cursor-grabbing sm:pt-7"
          role="group"
          aria-label={copy.rotation}
          tabindex={0}
        >
          <h1
            id="hero-title"
            class="grid w-[min(35rem,88cqi)]"
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
        <div class="landing-gutter col-start-2 grid justify-items-center gap-3 pb-5 text-xs leading-snug text-muted sm:pb-7 sm:text-sm">
          <p>{copy.drag}</p>
          <dl class="hidden grid-cols-[minmax(0,max-content)_auto] items-baseline gap-x-6 gap-y-1 sm:grid" aria-label={copy.shortcuts}>
            {homeShortcuts.map(({ command, keys }) => (
              <div class="contents">
                <dt>{command[props.locale]}</dt>
                <dd class="text-end">
                  <kbd class="border-0 bg-line/55 px-1 py-0 text-[inherit] text-inherit">{keys}</kbd>
                </dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </div>
  )
}
