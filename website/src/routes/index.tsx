import { createFileRoute } from "@tanstack/solid-router"
import { createSignal } from "solid-js"
import screenshots from "../../../assets/workbench.webp?url"
import { AnsiTerminal } from "../ansi-logo"
import { docs, repository, siteUrl } from "../docs-paths"
import { SiteHeader } from "../site-header"

const shortcuts = docs.shortcuts.path

const workflows = [
  {
    label: "Browse code",
    description: "Explorer reuses a single preview tab as you browse. Use Outline to jump between Markdown headings.",
    keys: [
      { keys: ["Ctrl+G"], action: "Go to File" },
      { keys: ["j", "k"], action: "Preview files from the sidebar" },
      { keys: ["Enter"], action: "Keep the preview tab open" },
      { keys: ["Ctrl+2"], action: "Search across files" },
    ],
    imageRow: 0,
    imageDescription: "vl displaying a Markdown README with an inline image, the Explorer file tree, and Outline",
    href: `${shortcuts}#workbench`,
    link: "Workbench shortcuts",
  },
  {
    label: "Review diffs",
    description: "Review staged and unstaged changes in side-by-side or inline diffs. Stage or unstage individual hunks or selected lines.",
    keys: [
      { keys: ["Ctrl+3"], action: "Open Changes" },
      { keys: ["[", "]"], action: "Jump between hunks" },
      { keys: ["V"], action: "Start a line selection" },
      { keys: ["s"], action: "Stage or unstage a hunk or selection" },
    ],
    imageRow: 1,
    imageDescription: "vl displaying staged changes to a Go file in a side-by-side diff",
    href: `${shortcuts}#diff-editor`,
    link: "Diff shortcuts",
  },
  {
    label: "Commit history",
    description: "Browse commits in History, expand a commit to see its changed files, and open a file to inspect its diff.",
    keys: [
      { keys: ["Ctrl+4"], action: "Open History" },
      { keys: ["j", "k"], action: "Move through commits and files" },
      { keys: ["Enter"], action: "Expand a commit or open a file diff" },
    ],
    imageRow: 2,
    imageDescription: "vl displaying a Git commit with a side-by-side diff and the changed files in the sidebar",
    href: `${shortcuts}#sidebar`,
    link: "Sidebar shortcuts",
  },
] as const

const themes = ["Dark", "Light"] as const

const metrics = [
  { value: "~10", unit: "MB", label: "Executable size" },
  { value: "~20", unit: "MiB", label: "Editor memory", detail: "1 Go file open" },
  { value: "~80", unit: "MiB", label: "Workbench memory", detail: "Sample vl session" },
  { value: "<1", unit: "%", label: "Idle CPU" },
]

const metricsContext = "vl 0.1.3 on Apple M4 with macOS 15.7.9. Memory is the physical footprint. Idle CPU is the mean over 10 seconds with one Go file open in a 120 × 40 terminal."

const modes = [
  {
    title: "Open a workspace",
    command: "vl .",
  },
  {
    title: "Edit at a line and column",
    command: "vl main.go:42:8",
  },
  {
    title: "Print a highlighted preview",
    command: "vl preview main.go",
  },
]

export const Route = createFileRoute("/")({
  head: () => ({ links: [{ rel: "canonical", href: siteUrl }] }),
  component: Landing,
})

function WorkbenchSection() {
  const [workflow, setWorkflow] = createSignal<(typeof workflows)[number]>(workflows[0])
  const [theme, setTheme] = createSignal<(typeof themes)[number]>("Dark")

  return (
    <section id="features" class="landing-section" aria-labelledby="workbench-title">
      <div class="section-heading">
        <h2 id="workbench-title" class="section-title">Inside the workbench</h2>
      </div>

      <div class="tab-bar justify-between border-t">
        <div class="flex flex-wrap" role="group" aria-label="Workbench view">
          {workflows.map((view) => (
            <button class="tab" type="button" aria-pressed={workflow() === view ? "true" : "false"} onClick={() => setWorkflow(view)}>
              {view.label}
            </button>
          ))}
        </div>
        <div class="flex border-line max-sm:basis-full max-sm:border-t sm:border-s" role="group" aria-label="Screenshot theme">
          {themes.map((name) => (
            <button class="tab" type="button" aria-pressed={theme() === name ? "true" : "false"} onClick={() => setTheme(name)}>
              {name}
            </button>
          ))}
        </div>
      </div>

      <figure class="relative aspect-[1200/831] overflow-hidden">
        <img
          src={screenshots}
          width="2400"
          height="2493"
          class={`absolute h-auto w-[200%] max-w-none ${theme() === "Dark" ? "left-0" : "-left-full"}`}
          style={{ top: `${-100 * workflow().imageRow}%` }}
          alt={workflow().imageDescription}
          decoding="async"
        />
      </figure>

      <div class="page-gutter grid gap-6 border-t border-line py-6 md:grid-cols-[minmax(0,5fr)_minmax(0,4fr)] md:gap-10 md:py-8">
        <div>
          <p class="max-w-[52ch] leading-relaxed text-muted">{workflow().description}</p>
          <a class="site-link text-link mt-3" href={workflow().href}>{workflow().link}</a>
        </div>
        <dl class="divide-y divide-line border-y border-line" aria-label="Workbench shortcuts">
          {workflow().keys.map(({ keys, action }) => (
            <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2.5 text-sm">
              <dt class="text-muted">{action}</dt>
              <dd class="flex gap-1">{keys.map((key) => <kbd>{key}</kbd>)}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  )
}

function UsageSection() {
  return (
    <section id="usage" class="landing-section" aria-labelledby="usage-title">
      <div class="section-heading">
        <h2 id="usage-title" class="section-title">Run vl from the shell</h2>
      </div>

      <div class="grid gap-px border-y border-line bg-line md:grid-cols-3">
        {modes.map(({ command, title }) => (
          <article class="page-gutter min-w-0 bg-canvas py-5 sm:py-6">
            <h3 class="mb-2.5 text-sm font-medium text-muted">{title}</h3>
            <code class="block text-sm text-accent">{command}</code>
          </article>
        ))}
      </div>

      <div class="page-gutter flex flex-wrap items-center gap-x-6 gap-y-2 py-4">
        <a class="site-link text-link" href={`${docs.usage.path}#setup`}>Workbench setup</a>
        <a class="site-link text-link" href={`${docs.usage.path}#yazi-integration`}>Yazi integration</a>
      </div>
    </section>
  )
}

function Landing() {
  return (
    <div class="site-frame">
      <SiteHeader />

      <main>
        <section aria-labelledby="hero-title">
          <AnsiTerminal />
          <div class="grid gap-px border-t border-line bg-line md:grid-cols-4">
            <p class="page-gutter bg-canvas py-5 text-[clamp(1rem,1.5vw,1.125rem)] leading-snug text-muted sm:py-6 md:col-span-3">
              <span class="font-medium text-ink">A <strong>V</strong>SCode-<strong>l</strong>ike terminal workbench</span>
              <br />
              with <strong>V</strong>im-<strong>l</strong>ike keybindings.
            </p>
            <a
              class="page-gutter flex min-h-12 items-center justify-between gap-3 bg-canvas font-mono text-sm font-medium text-accent hover:bg-workspace focus-visible:-outline-offset-2"
              href={docs.install.path}
            >
              Install
              <span class="text-lg leading-none" aria-hidden="true">→</span>
            </a>
          </div>
        </section>

        <dl class="grid grid-cols-2 gap-px border-t border-line bg-line md:grid-cols-4" aria-label={metricsContext} title={metricsContext}>
          {metrics.map(({ value, unit, label, detail }) => (
            <div class="page-gutter flex flex-col gap-2 bg-canvas py-5 sm:gap-3 sm:py-7">
              <dt class="order-last font-mono text-xs text-muted sm:text-sm">
                {label}
                {detail && <span class="mt-1 block text-xs">{detail}</span>}
              </dt>
              <dd class="flex items-baseline gap-1.5">
                <span class="text-[clamp(1.65rem,3vw,2.25rem)] leading-none font-semibold tracking-tight tabular-nums">{value}</span>
                <span class="text-sm text-muted sm:text-base">{unit}</span>
              </dd>
            </div>
          ))}
        </dl>

        <WorkbenchSection />
        <UsageSection />
      </main>

      <footer class="border-t border-line font-mono text-sm text-muted">
        <div class="page-gutter flex min-h-17 flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3">
          <a class="site-link" href={`${repository}/blob/main/LICENSE`}>MPL 2.0</a>
          <a class="site-link" href={`${repository}/releases`}>Releases</a>
        </div>
      </footer>
    </div>
  )
}
