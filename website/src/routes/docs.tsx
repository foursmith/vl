import { createFileRoute, Outlet, useLocation } from "@tanstack/solid-router"
import { docs } from "../docs-paths"
import { SiteHeader } from "../site-header"

export const Route = createFileRoute("/docs")({
  component: DocsLayout,
})

function DocsLayout() {
  const pathname = useLocation({ select: (location) => location.pathname.replace(/\/$/, "") })

  return (
    <div class="site-frame docs-frame min-h-dvh">
      <SiteHeader />
      <main>
        <nav class="tab-bar docs-navigation" aria-label="Documentation">
          {Object.values(docs).map(({ path, title }) => (
            <a
              class="tab"
              aria-current={pathname() === path.replace(/\/$/, "") ? "page" : undefined}
              href={path}
            >{title}</a>
          ))}
        </nav>
        <Outlet />
      </main>
    </div>
  )
}
