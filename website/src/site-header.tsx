import logo from "../../assets/logo.svg?url"
import { docs, repository } from "./docs-paths"

export function SiteHeader() {
  return (
    <header class="sticky top-0 z-10 border-b border-line bg-canvas">
      <div class="page-gutter flex min-h-14 items-center justify-between gap-3">
        <a href="/" aria-label="vl home">
          <img src={logo} alt="" width="40" height="24" class="h-auto w-10" />
        </a>
        <nav class="flex items-center gap-[clamp(0.7rem,3vw,1.75rem)] text-sm font-medium text-muted" aria-label="Main navigation">
          <a class="site-link" href="/#features">Workbench</a>
          <a class="site-link" href={docs.install.path}>Docs</a>
          <a class="site-link text-ink" href={repository}>GitHub</a>
        </nav>
      </div>
    </header>
  )
}
