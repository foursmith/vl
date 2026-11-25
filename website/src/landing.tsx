import { AnsiTerminal } from "./ansi-logo"
import { docs, repository } from "./docs-paths"
import type { Locale } from "./locales"
import { SiteHeader } from "./site-header"
import { screenshots, screenshotSizes, screenshotSrcset, themes } from "./workbench-images"

const workflows = [
  {
    label: { en: "Browse code", "zh-CN": "浏览代码" },
    description: { en: "Explorer reuses a single preview tab as you browse. Use Outline to jump between Markdown headings.", "zh-CN": "在资源管理器中浏览文件时会复用预览标签页。使用大纲在 Markdown 标题之间跳转。" },
    keys: [
      { keys: ["Ctrl+G"], action: { en: "Go to File", "zh-CN": "转到文件" } },
      { keys: ["j", "k"], action: { en: "Preview files from the sidebar", "zh-CN": "在侧边栏中预览文件" } },
      { keys: ["Enter"], action: { en: "Keep the preview tab open", "zh-CN": "保留预览标签页" } },
      { keys: ["Ctrl+Shift+F"], action: { en: "Search across files", "zh-CN": "在文件中搜索" } },
      { keys: ["Tab", "Shift+Tab"], action: { en: "Cycle sidebar tabs when focused", "zh-CN": "侧边栏获得焦点时循环切换标签" } },
      { keys: ["F12"], action: { en: "Toggle the sidebar", "zh-CN": "显示或隐藏侧边栏" } },
    ],
    imageDescription: { en: "vl displaying a Markdown README with an inline image, the Explorer file tree, and Outline", "zh-CN": "vl 中显示带内嵌图片的 Markdown README、资源管理器文件树和大纲" },
    anchor: { en: "workbench", "zh-CN": "工作台" },
    link: { en: "Workbench shortcuts", "zh-CN": "工作台快捷键" },
  },
  {
    label: { en: "Review diffs", "zh-CN": "审阅差异" },
    description: { en: "Review staged and unstaged changes in side-by-side or inline diffs. Stage or unstage individual hunks or selected lines.", "zh-CN": "通过并排或内联视图审阅已暂存和未暂存的更改。可暂存或取消暂存单个区块或选中的行。" },
    keys: [
      { keys: ["Ctrl+3"], action: { en: "Open Changes", "zh-CN": "打开更改视图" } },
      { keys: ["[", "]"], action: { en: "Jump between hunks", "zh-CN": "在差异区块之间跳转" } },
      { keys: ["V"], action: { en: "Start a line selection", "zh-CN": "开始按行选择" } },
      { keys: ["s"], action: { en: "Stage or unstage a hunk or selection", "zh-CN": "暂存或取消暂存区块或选区" } },
      { keys: ["F7"], action: { en: "Open the context menu", "zh-CN": "打开右键菜单" } },
    ],
    imageDescription: { en: "vl displaying staged changes to a Go file in a side-by-side diff", "zh-CN": "vl 中并排显示一个 Go 文件已暂存的更改" },
    anchor: { en: "diff-editor", "zh-CN": "diff-编辑器" },
    link: { en: "Diff shortcuts", "zh-CN": "差异编辑器快捷键" },
  },
  {
    label: { en: "Commit history", "zh-CN": "提交历史" },
    description: { en: "Browse commits in History, expand a commit to see its changed files, and open a file to inspect its diff.", "zh-CN": "在历史视图中浏览提交，展开提交查看更改的文件，再打开文件查看差异。" },
    keys: [
      { keys: ["Ctrl+4"], action: { en: "Open History", "zh-CN": "打开历史视图" } },
      { keys: ["j", "k"], action: { en: "Move through commits and files", "zh-CN": "在提交和文件之间移动" } },
      { keys: ["Enter"], action: { en: "Expand a commit or open a file diff", "zh-CN": "展开提交或打开文件差异" } },
    ],
    imageDescription: { en: "vl displaying a Git commit with a side-by-side diff and the changed files in the sidebar", "zh-CN": "vl 中显示 Git 提交的并排差异，侧边栏列出更改的文件" },
    anchor: { en: "sidebar", "zh-CN": "侧边栏" },
    link: { en: "Sidebar shortcuts", "zh-CN": "侧边栏快捷键" },
  },
] as const


const metrics = [
  { value: "~10", unit: "MB", label: { en: "Executable size", "zh-CN": "可执行文件大小" } },
  { value: "~20", unit: "MiB", label: { en: "Editor memory", "zh-CN": "编辑器内存" }, detail: { en: "1 Go file open", "zh-CN": "打开 1 个 Go 文件" } },
  { value: "~80", unit: "MiB", label: { en: "Workbench memory", "zh-CN": "工作台内存" }, detail: { en: "Sample vl session", "zh-CN": "示例 vl 会话" } },
  { value: "<1", unit: "%", label: { en: "Idle CPU", "zh-CN": "空闲 CPU 占用" } },
]

const metricsContext = {
  en: "vl 0.1.3 on Apple M4 with macOS 15.7.9. Memory is the physical footprint. Idle CPU is the mean over 10 seconds with one Go file open in a 120 × 40 terminal.",
  "zh-CN": "在运行 macOS 15.7.9 的 Apple M4 上测试 vl 0.1.3。内存为实际物理内存占用。空闲 CPU 为 120 × 40 的终端中打开一个 Go 文件时 10 秒内的平均占用。",
}

const modes = [
  {
    title: { en: "Open a workspace", "zh-CN": "打开工作区" },
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
    insideWorkbench: "Inside the workbench",
    workbenchView: "Workbench view",
    screenshotTheme: "Screenshot theme",
    Dark: "Dark",
    Light: "Light",
    shortcuts: "Workbench shortcuts",
    shell: "Run vl from the shell",
    setup: "Workbench setup",
    setupAnchor: "setup",
    yazi: "Yazi integration",
    yaziAnchor: "yazi-integration",
    releases: "Releases",
  },
  "zh-CN": {
    insideWorkbench: "工作台预览",
    workbenchView: "工作台视图",
    screenshotTheme: "截图主题",
    Dark: "深色",
    Light: "浅色",
    shortcuts: "工作台快捷键",
    shell: "从终端运行 vl",
    setup: "环境准备",
    setupAnchor: "环境准备",
    yazi: "Yazi 集成",
    yaziAnchor: "yazi-集成",
    releases: "版本发布",
  },
}

function WorkbenchSection(props: { locale: Locale }) {
  const copy = labels[props.locale]
  const firstImage = screenshots[0].Dark

  return (
    <section id="features" class="landing-section" aria-labelledby="workbench-title">
      <div class="section-heading">
        <h2 id="workbench-title" class="section-title">{copy.insideWorkbench}</h2>
      </div>

      <div class="tab-bar justify-between border-t">
        <div class="flex flex-wrap" role="group" aria-label={copy.workbenchView}>
          {workflows.map((view, index) => (
            <button class="tab" type="button" data-workbench-view={index} aria-pressed={index === 0 ? "true" : "false"}>
              {view.label[props.locale]}
            </button>
          ))}
        </div>
        <div class="flex border-line max-sm:basis-full max-sm:border-t sm:border-s" role="group" aria-label={copy.screenshotTheme}>
          {themes.map((name) => (
            <button class="tab" type="button" data-workbench-theme={name} aria-pressed={name === "Dark" ? "true" : "false"}>
              {copy[name]}
            </button>
          ))}
        </div>
      </div>

      <figure>
        <img
          src={firstImage.src}
          srcset={screenshotSrcset(firstImage)}
          sizes={screenshotSizes}
          width="1200"
          height="831"
          class="h-auto w-full"
          alt={workflows[0].imageDescription[props.locale]}
          fetchpriority="high"
          decoding="async"
        />
      </figure>

      <div data-workbench-details class="page-gutter grid gap-6 border-t border-line py-6 md:grid-cols-[minmax(0,5fr)_minmax(0,4fr)] md:gap-10 md:py-8">
        <WorkbenchDetails workflow={workflows[0]} locale={props.locale} />
      </div>
      {workflows.map((workflow, index) => (
        <template data-workbench-view-details={index} data-image-alt={workflow.imageDescription[props.locale]}>
          <WorkbenchDetails workflow={workflow} locale={props.locale} />
        </template>
      ))}
    </section>
  )
}

function WorkbenchDetails(props: { workflow: (typeof workflows)[number]; locale: Locale }) {
  const shortcuts = docs[props.locale].shortcuts.path

  return (
    <>
      <div>
        <p class="max-w-[52ch] leading-relaxed text-muted">{props.workflow.description[props.locale]}</p>
        <a class="site-link text-link mt-3" href={`${shortcuts}#${props.workflow.anchor[props.locale]}`}>{props.workflow.link[props.locale]}</a>
      </div>
      <dl class="divide-y divide-line border-y border-line" aria-label={labels[props.locale].shortcuts}>
        {props.workflow.keys.map(({ keys, action }) => (
          <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2.5 text-sm">
            <dt class="text-muted">{action[props.locale]}</dt>
            <dd class="flex gap-1">{keys.map((key) => <kbd>{key}</kbd>)}</dd>
          </div>
        ))}
      </dl>
    </>
  )
}

function UsageSection(props: { locale: Locale }) {
  const copy = labels[props.locale]
  const usage = docs[props.locale].usage.path

  return (
    <section id="usage" class="landing-section" aria-labelledby="usage-title">
      <div class="section-heading">
        <h2 id="usage-title" class="section-title">{copy.shell}</h2>
      </div>

      <div class="grid gap-px border-y border-line bg-line md:grid-cols-3">
        {modes.map(({ command, title }) => (
          <article class="page-gutter min-w-0 bg-canvas py-5 sm:py-6">
            <h3 class="mb-2.5 text-sm font-medium text-muted">{title[props.locale]}</h3>
            <code class="block text-sm text-accent">{command}</code>
          </article>
        ))}
      </div>

      <div class="page-gutter flex flex-wrap items-center gap-x-6 gap-y-2 py-4">
        <a class="site-link text-link" href={`${usage}#${copy.setupAnchor}`}>{copy.setup}</a>
        <a class="site-link text-link" href={`${usage}#${copy.yaziAnchor}`}>{copy.yazi}</a>
      </div>
    </section>
  )
}

export function Landing(props: { locale: Locale }) {
  const install = docs[props.locale].install
  const metricsHint = metricsContext[props.locale]

  return (
    <div class="site-frame">
      <SiteHeader locale={props.locale} />

      <main>
        <section aria-labelledby="hero-title">
          <AnsiTerminal locale={props.locale} />
          <div class="grid gap-px border-t border-line bg-line md:grid-cols-4">
            <p class="page-gutter bg-canvas py-5 text-[clamp(1rem,1.5vw,1.125rem)] leading-snug text-muted sm:py-6 md:col-span-3">
              {props.locale === "en" ? (
                <>
                  <span class="font-medium text-ink">A <strong>V</strong>SCode-<strong>l</strong>ike terminal workbench</span>
                  <br />
                  with <strong>V</strong>im-<strong>l</strong>ike keybindings.
                </>
              ) : (
                <>
                  <span class="font-medium text-ink">类 VS Code 的终端工作台</span>
                  <br />
                  支持 Vim 风格键位。
                </>
              )}
            </p>
            <a
              class="page-gutter flex min-h-12 items-center justify-between gap-3 bg-canvas font-mono text-sm font-medium text-accent hover:bg-workspace focus-visible:-outline-offset-2"
              href={install.path}
            >
              {install.title}
              <span class="text-lg leading-none" aria-hidden="true">→</span>
            </a>
          </div>
        </section>

        <dl class="grid grid-cols-2 gap-px border-t border-line bg-line md:grid-cols-4" aria-label={metricsHint} title={metricsHint}>
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
        <UsageSection locale={props.locale} />
      </main>

      <footer class="border-t border-line font-mono text-sm text-muted">
        <div class="page-gutter flex min-h-17 flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3">
          <a class="site-link" href={`${repository}/blob/main/LICENSE`}>MPL 2.0</a>
          <a class="site-link" href={`${repository}/releases`}>{labels[props.locale].releases}</a>
        </div>
      </footer>
    </div>
  )
}
