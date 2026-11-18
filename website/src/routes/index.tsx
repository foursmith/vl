import { createFileRoute } from "@tanstack/solid-router"
import logo from "../../../assets/logo.svg?url"
import screenshots from "../../../assets/screenshots.webp?url"
import { AnsiTerminal } from "../ansi-logo"
import { InstallSection } from "../install"

const repository = "https://github.com/foursmith/vl"
const workbenchScreens = [
  { label: "Git diff · Dark", position: "top-0 left-0" },
  { label: "Git diff · Light", position: "top-0 -left-full" },
  { label: "Editor · Dark", position: "-top-full left-0" },
  { label: "Editor · Light", position: "-top-full -left-full" },
]

const modes = [
  {
    title: "Browse and review",
    description: "Explore files, search the workspace, and review changes side by side.",
    command: "vl .",
  },
  {
    title: "Edit a file",
    description: "Open a file directly for a quick edit with familiar Vim motions.",
    command: "vl path/to/file",
  },
  {
    title: "Preview code",
    description: "Print a syntax-highlighted file straight to the terminal.",
    command: "vl preview path/to/file",
  },
]

export const Route = createFileRoute("/")({
  component: Landing,
})

function Landing() {
  return (
    <div class="site-frame">
      <header class="sticky top-0 z-10 border-b border-line bg-canvas/95 backdrop-blur-md">
        <div class="page-gutter flex min-h-14 items-center justify-between">
          <a href="/" aria-label="vl home">
            <img src={logo} alt="" width="40" height="24" class="h-auto w-10" />
          </a>
          <nav class="flex items-center gap-[clamp(0.85rem,3vw,1.75rem)] text-sm font-medium text-muted" aria-label="Main navigation">
            <a href="#features">Features</a>
            <a href="#install">Install</a>
            <a class="text-ink hover:text-accent" href={repository}>GitHub</a>
          </nav>
        </div>
      </header>

      <main>
        <section class="page-gutter py-[clamp(1.75rem,5vw,4.5rem)]" aria-labelledby="hero-title">
          <AnsiTerminal>
            <p class="max-w-[27rem] font-sans text-[clamp(1rem,1.5vw,1.125rem)] leading-relaxed text-pretty text-muted">
              Browse code, review diffs, and make quick edits.
            </p>
            <a class="inline-flex min-h-10 items-center justify-self-start rounded-md bg-ink px-4 font-sans text-sm font-semibold text-canvas hover:bg-white/85 hover:text-canvas lg:justify-self-end" href="#install">
              Install
            </a>
          </AnsiTerminal>
        </section>

        <section id="features" class="landing-section" aria-labelledby="features-title">
          <div class="mb-6 flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
            <h2 id="features-title" class="section-title">
              Features
            </h2>
            <p class="text-muted">
              <strong>V</strong>SCode-<strong>l</strong>ike terminal workbench with <strong>V</strong>im-<strong>l</strong>ike keybindings
            </p>
          </div>

          <div class="grid gap-px overflow-hidden rounded-md border border-line bg-line sm:grid-cols-2">
            {workbenchScreens.map(({ label, position }) => (
              <figure class="min-w-0 bg-panel p-2 sm:p-3">
                <div class="relative aspect-[1600/919] overflow-hidden">
                  <img
                    src={screenshots}
                    width="3200"
                    height="1838"
                    class={`absolute h-auto w-[200%] max-w-none ${position}`}
                    alt=""
                    loading="lazy"
                    decoding="async"
                  />
                </div>
                <figcaption class="px-1.5 py-1.5 font-mono text-xs text-muted sm:px-2">{label}</figcaption>
              </figure>
            ))}
          </div>

          <div class="mt-8 grid gap-px border-y border-line bg-line sm:grid-cols-3">
            {modes.map((mode) => (
              <article class="bg-canvas p-5 sm:p-6">
                <code class="wrap-anywhere text-xs text-accent">{mode.command}</code>
                <h3 class="mt-2.5 text-base font-semibold">{mode.title}</h3>
                <p class="mt-1.5 leading-relaxed text-pretty text-muted">{mode.description}</p>
              </article>
            ))}
          </div>

          <div class="mt-9 grid gap-7 sm:grid-cols-2 sm:gap-0">
            <section aria-labelledby="omitted-vscode-title">
              <h3 id="omitted-vscode-title" class="text-lg font-semibold">Omitted VS Code features</h3>
              <ul class="mt-3 list-disc space-y-1.5 ps-5 leading-relaxed text-muted marker:text-accent">
                <li>
                  Language server integration
                  <ul class="mt-1 list-[circle] space-y-1 ps-5 marker:text-muted">
                    <li>Code completion</li>
                    <li>Diagnostics</li>
                    <li>Go to Definition</li>
                  </ul>
                </li>
                <li>Debugging</li>
                <li>Integrated terminal</li>
                <li>Build and test tasks</li>
                <li>Extension runtime and marketplace</li>
              </ul>
            </section>
            <section class="border-t border-line pt-6 sm:border-s sm:border-t-0 sm:ps-8 sm:pt-0" aria-labelledby="omitted-vim-title">
              <h3 id="omitted-vim-title" class="text-lg font-semibold">Omitted Vim features</h3>
              <ul class="mt-3 list-disc space-y-1.5 ps-5 leading-relaxed text-muted marker:text-accent">
                <li>Visual block mode</li>
                <li>Macros</li>
                <li>Named registers</li>
                <li>Marks and mark jumps</li>
                <li>Ex commands</li>
                <li>Vimscript configuration and plugins</li>
              </ul>
            </section>
          </div>
        </section>

        <InstallSection />
      </main>

      <footer class="border-t border-line font-mono text-sm text-muted">
        <div class="page-gutter flex min-h-17 items-center justify-between">
          <span>vl <span class="text-[#6a737f]">/</span> <a href={`${repository}/blob/main/LICENSE`}>MPL 2.0</a></span>
          <a href={`${repository}/releases`}>Releases</a>
        </div>
      </footer>
    </div>
  )
}
