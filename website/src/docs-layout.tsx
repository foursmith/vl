import { Outlet, useLocation } from "@tanstack/solid-router"
import { docs } from "./docs-paths"
import { locales, type Locale } from "./locales"
import { SiteHeader } from "./site-header"

export function DocsLayout(props: { locale: Locale }) {
  const pathname = useLocation({ select: (location) => location.pathname.replace(/\/$/, "") })

  return (
    <div class="site-frame docs-frame min-h-dvh">
      <SiteHeader locale={props.locale} />
      <main>
        <nav class="tab-bar docs-navigation" aria-label={locales[props.locale].documentation}>
          {Object.values(docs[props.locale]).map(({ path, title }) => (
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
