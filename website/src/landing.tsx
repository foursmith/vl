import { AnsiTerminal } from "./ansi-logo"
import { docs, repository } from "./docs-paths"
import type { Locale } from "./locales"
import { SiteHeader } from "./site-header"
import { screenshots, screenshotSizes, screenshotSrcset } from "./workbench-images"

const workflows = [
  {
    label: { en: "Read code", "zh-CN": "读代码" },
    description: { en: "Preview files as you browse. Use Outline to jump to Markdown headings.", "zh-CN": "浏览文件时复用预览标签页，用大纲跳转到 Markdown 标题。" },
    keys: [
      { keys: ["Ctrl+G"], action: { en: "Go to File", "zh-CN": "转到文件" } },
      { keys: ["Ctrl+F"], action: { en: "Find in the current file", "zh-CN": "在当前文件中查找" } },
      { keys: ["Ctrl+Shift+F"], action: { en: "Search the workspace", "zh-CN": "搜索工作区" } },
      { keys: ["Ctrl+T", "Ctrl+Shift+T"], action: { en: "Cycle editor tabs", "zh-CN": "切换编辑器标签页" } },
      { keys: ["Ctrl+W"], action: { en: "Close the current tab", "zh-CN": "关闭当前标签页" } },
      { keys: ["F12"], action: { en: "Toggle the sidebar", "zh-CN": "显示或隐藏侧栏" } },
      { keys: ["Ctrl+P"], action: { en: "Open the command palette", "zh-CN": "打开命令面板" } },
      { keys: ["Ctrl+R"], action: { en: "Open recent workspaces", "zh-CN": "打开最近的工作区" } },
    ],
    imageDescription: { en: "vl showing README.md with an inline image, the file tree, and the Markdown outline", "zh-CN": "vl 中打开的 README.md，包含内嵌图片，左侧显示文件树和 Markdown 大纲" },
    guide: "usage",
    fragment: { en: "browse-and-search-code", "zh-CN": "浏览和搜索代码" },
    link: { en: "Browse and search code", "zh-CN": "浏览和搜索代码" },
  },
  {
    label: { en: "Review Git diffs", "zh-CN": "审阅 Git 改动" },
    description: { en: "Compare changes side by side. Stage or unstage a hunk or the lines you select.", "zh-CN": "并排查看文件改动，按区块或选区暂存、取消暂存。" },
    keys: [
      { keys: ["[", "]"], action: { en: "Jump between hunks", "zh-CN": "在差异区块之间跳转" } },
      { keys: ["Enter"], action: { en: "Expand or collapse context", "zh-CN": "展开或折叠上下文" } },
      { keys: ["V"], action: { en: "Select changed lines", "zh-CN": "按行选择改动" } },
      { keys: ["s"], action: { en: "Stage or unstage a hunk or selection", "zh-CN": "暂存或取消暂存区块或选区" } },
      { keys: ["S"], action: { en: "Stage or unstage the whole file", "zh-CN": "暂存或取消暂存整个文件" } },
      { keys: ["d"], action: { en: "Discard a hunk or selection…", "zh-CN": "放弃区块或选区…" } },
      { keys: ["D"], action: { en: "Discard all unstaged file changes…", "zh-CN": "放弃文件的未暂存改动…" } },
      { keys: ["F7"], action: { en: "Open the diff menu", "zh-CN": "打开差异菜单" } },
    ],
    imageDescription: { en: "vl showing the staged changes to package.json in a side-by-side diff", "zh-CN": "vl 中并排显示 package.json 已暂存的改动" },
    guide: "usage",
    fragment: { en: "review-git-changes", "zh-CN": "审阅-git-改动" },
    link: { en: "Review Git changes", "zh-CN": "审阅 Git 改动" },
  },
  {
    label: { en: "Make quick edits", "zh-CN": "顺手改几行" },
    description: { en: "Jump to the line you need, edit with Vim keys, and save.", "zh-CN": "打开文件直接跳到要改的那一行，用 Vim 键位改完就在终端里保存。" },
    keys: [
      { keys: ["i", "a"], action: { en: "Enter Insert mode", "zh-CN": "进入插入模式" } },
      { keys: ["Esc"], action: { en: "Return to Normal mode", "zh-CN": "返回普通模式" } },
      { keys: ["ciw"], action: { en: "Change the current word", "zh-CN": "改写光标所在的词" } },
      { keys: ["dd"], action: { en: "Delete the current line", "zh-CN": "删除当前行" } },
      { keys: ["gcc"], action: { en: "Toggle the line comment", "zh-CN": "切换当前行的注释" } },
      { keys: ["u", "U"], action: { en: "Undo / redo", "zh-CN": "撤销 / 重做" } },
      { keys: ["."], action: { en: "Repeat the last edit", "zh-CN": "重复上次编辑" } },
      { keys: [":w"], action: { en: "Save the file", "zh-CN": "保存文件" } },
    ],
    imageDescription: { en: "vl's single-file editor with the cursor in install.sh", "zh-CN": "光标位于 vl 单文件编辑器中打开的 install.sh" },
    guide: "vim",
    link: { en: "Vim editing guide", "zh-CN": "Vim 编辑指南" },
  },
] as const


const metrics = [
  { value: "~10", unit: "MB", label: { en: "Executable size", "zh-CN": "可执行文件大小" } },
  { value: "~20", unit: "MiB", label: { en: "Editor memory", "zh-CN": "编辑器内存" }, detail: { en: "1 Go file open", "zh-CN": "打开 1 个 Go 文件" } },
  { value: "~80", unit: "MiB", label: { en: "Workbench memory", "zh-CN": "工作台内存" }, detail: { en: "6 files open", "zh-CN": "打开 6 个文件" } },
  { value: "<1", unit: "%", label: { en: "Idle CPU", "zh-CN": "空闲 CPU 占用" } },
]

const metricsContext = {
  en: "Measured with vl 0.1.5-37-g668b477-dirty in a 120 × 40 terminal on Apple M4 with macOS 15.7.9. Memory is the physical footprint. Idle CPU is the 10-second average with one Go file open.",
  "zh-CN": "使用 vl 0.1.5-37-g668b477-dirty 在 Apple M4（macOS 15.7.9）的 120 × 40 终端中测试。内存为物理内存占用。空闲 CPU 为打开 1 个 Go 文件后 10 秒内的平均占用。",
}

const modes = [
  {
    title: { en: "Reopen the last workspace", "zh-CN": "重新打开最近的工作区" },
    command: "vl",
  },
  {
    title: { en: "Open the current workspace", "zh-CN": "打开当前工作区" },
    command: "vl .",
  },
  {
    title: { en: "Edit at a line and column", "zh-CN": "打开文件并定位到指定行列" },
    command: "vl main.go:42:8",
  },
  {
    title: { en: "Print a highlighted preview", "zh-CN": "输出带语法高亮的预览" },
    command: "vl preview main.go",
  },
]

const labels = {
  en: {
    intro: "Read code. Review Git diffs. Make quick edits.",
    controls: "A VS Code-like terminal workbench with mouse support and Vim keys.",
    install: "Install vl",
    shortcuts: "Workbench shortcuts",
    releases: "Releases",
  },
  "zh-CN": {
    intro: "读代码，审阅 Git 改动，顺手改几行。",
    controls: "VS Code 风格的终端工作台，支持鼠标和 Vim 键位。",
    install: "安装 vl",
    shortcuts: "工作台快捷键",
    releases: "版本发布",
  },
}

function WorkbenchSection(props: { locale: Locale }) {
  return (
    <div id="features" class="landing-section divide-y divide-line">
      {workflows.map((workflow, index) => {
        const { Dark, Light } = screenshots[index]
        const screenshotOnLeft = index % 2 === 1

        return (
          <article class={`grid ${screenshotOnLeft ? "lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]" : "lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]"}`}>
            <figure class={`self-start lg:row-start-1 ${screenshotOnLeft ? "lg:col-start-1" : "lg:col-start-2"}`}>
              <picture>
                <source data-light-preview media="not all" srcset={screenshotSrcset(Light)} sizes={screenshotSizes} width="1440" height="1003" />
                <img
                  class="w-full"
                  src={Dark.src}
                  srcset={screenshotSrcset(Dark)}
                  sizes={screenshotSizes}
                  width="1440"
                  height={Dark.height}
                  alt={workflow.imageDescription[props.locale]}
                  loading="lazy"
                  decoding="async"
                />
              </picture>
            </figure>
            <div class={`grid gap-5 border-t border-line pt-6 lg:row-start-1 lg:grid-rows-[auto_1fr] lg:border-t-0 ${screenshotOnLeft ? "lg:col-start-2 lg:border-s" : "lg:col-start-1 lg:border-e"}`}>
              <WorkbenchDetails workflow={workflow} locale={props.locale} />
            </div>
          </article>
        )
      })}
    </div>
  )
}

function WorkbenchDetails(props: { workflow: (typeof workflows)[number]; locale: Locale }) {
  const guide = docs[props.locale][props.workflow.guide]
  const fragment = "fragment" in props.workflow ? `#${props.workflow.fragment[props.locale]}` : ""

  return (
    <>
      <div class="page-gutter lg:px-6">
        <h2 class="mb-3 text-lg font-semibold">{props.workflow.label[props.locale]}</h2>
        <p class="max-w-[52ch] leading-relaxed text-muted">{props.workflow.description[props.locale]}</p>
        <a class="site-link text-link mt-3" href={guide.path + fragment}>{props.workflow.link[props.locale]}</a>
      </div>
      <dl class="divide-y divide-line border-t border-line lg:grid lg:auto-rows-[minmax(min-content,1fr)]" aria-label={labels[props.locale].shortcuts}>
        {props.workflow.keys.map(({ keys, action }) => (
          <div class="page-gutter grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2.5 text-sm lg:px-6 lg:max-xl:py-1.5">
            <dt class="text-muted">{action[props.locale]}</dt>
            <dd class="flex gap-1">{keys.map((key) => <kbd>{key}</kbd>)}</dd>
          </div>
        ))}
      </dl>
    </>
  )
}

function UsageSection(props: { locale: Locale }) {
  return (
    <div id="usage" class="landing-section landing-summary-row grid gap-px bg-line sm:grid-cols-2 lg:grid-cols-4">
      {modes.map(({ command, title }) => (
        <article class="page-gutter flex min-w-0 flex-col justify-center bg-canvas py-5 sm:py-6">
          <h2 class="mb-2.5 text-sm font-medium text-muted">{title[props.locale]}</h2>
          <code class="block text-sm text-accent">{command}</code>
        </article>
      ))}
    </div>
  )
}

export function Landing(props: { locale: Locale }) {
  const { install, usage } = docs[props.locale]
  const copy = labels[props.locale]
  const metricsHint = metricsContext[props.locale]

  return (
    <div class="site-frame">
      <SiteHeader locale={props.locale} />

      <main>
        <section aria-labelledby="hero-title">
          <AnsiTerminal locale={props.locale} />
          <div class="landing-summary-row grid gap-px border-t border-line bg-line md:grid-cols-4">
            <div class="page-gutter flex flex-col justify-center gap-2 bg-canvas py-5 sm:py-6 md:col-span-3">
              <p class="text-[clamp(1.375rem,2.4vw,2rem)] leading-tight font-semibold tracking-[-0.035em] text-ink">
                {copy.intro}
              </p>
              <p class="text-sm leading-relaxed text-muted">{copy.controls}</p>
            </div>
            <div class="grid gap-px bg-line font-mono text-sm max-md:grid-cols-2">
              <a
                class="page-gutter flex min-h-11 items-center justify-between gap-2 bg-canvas font-medium text-accent hover:bg-workspace focus-visible:-outline-offset-2"
                href={install.path}
              >
                {copy.install}
                <span class="text-lg leading-none" aria-hidden="true">→</span>
              </a>
              <a
                class="page-gutter flex min-h-11 items-center justify-between gap-2 bg-canvas text-muted hover:bg-workspace focus-visible:-outline-offset-2"
                href={usage.path}
              >
                {usage.title}
                <span class="text-lg leading-none" aria-hidden="true">→</span>
              </a>
            </div>
          </div>
        </section>

        <UsageSection locale={props.locale} />

        <dl class="landing-summary-row grid grid-cols-2 gap-px border-t border-line bg-line md:grid-cols-4" aria-label={metricsHint} title={metricsHint}>
          {metrics.map(({ value, unit, label, detail }) => (
            <div class="page-gutter flex flex-col gap-2 bg-canvas py-5 sm:gap-3 sm:py-7">
              <dt class="order-last font-mono text-xs text-muted sm:text-sm">
                {label[props.locale]}
                {detail && <span class="mt-1 block text-xs">{detail[props.locale]}</span>}
              </dt>
              <dd class="flex items-baseline gap-1.5">
                <span class="text-[clamp(1.65rem,3vw,2.25rem)] leading-none font-semibold tracking-tight tabular-nums">{value}</span>
                <span class="text-sm text-muted sm:text-base">{unit}</span>
              </dd>
            </div>
          ))}
        </dl>

        <WorkbenchSection locale={props.locale} />
      </main>

      <footer class="border-t border-line font-mono text-sm text-muted">
        <div class="page-gutter flex min-h-17 flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 lg:px-6">
          <a class="site-link" href={`${repository}/blob/main/LICENSE`}>MIT</a>
          <a class="site-link" href={`${repository}/releases`}>{labels[props.locale].releases}</a>
        </div>
      </footer>
    </div>
  )
}
