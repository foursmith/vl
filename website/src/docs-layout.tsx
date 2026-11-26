import { Outlet, useLocation } from "@tanstack/solid-router"
import { docGroups, docs } from "./docs-paths"
import { locales, type Locale } from "./locales"
import { SiteHeader } from "./site-header"

export function DocsLayout(props: { locale: Locale }) {
  const pathname = useLocation({ select: (location) => location.pathname.replace(/\/$/, "") })
  const labels = locales[props.locale]

  return (
    <div class="site-frame docs-frame min-h-dvh">
      <SiteHeader locale={props.locale} />
      <div class="border-b border-line md:hidden">
        <details>
          <summary class="page-gutter cursor-pointer py-3 text-sm font-medium">{labels.allDocs}</summary>
          <DocsNavigation locale={props.locale} pathname={pathname()} />
        </details>
      </div>
      <div class="grid md:min-h-[calc(100dvh_-_var(--site-header-height))] md:grid-cols-[12rem_minmax(0,1fr)] lg:grid-cols-[13rem_minmax(0,1fr)]">
        <aside class="hidden border-e border-line md:block">
          <DocsNavigation locale={props.locale} pathname={pathname()} />
        </aside>
        <main class="min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function DocsNavigation(props: { locale: Locale; pathname: string }) {
  const pages = docs[props.locale]

  return (
    <nav class="docs-navigation" aria-label={locales[props.locale].documentation}>
      {docGroups.map((group) => (
        <section aria-label={group.label[props.locale]}>
          <p class="docs-navigation-label">{group.label[props.locale]}</p>
          <ul>
            {group.pages.map(({ key }) => {
              const { path, title } = pages[key]
              return (
                <li>
                  <a href={path} aria-current={props.pathname === path.replace(/\/$/, "") ? "page" : undefined}>{title}</a>
                </li>
              )
            })}
          </ul>
        </section>
      ))}
    </nav>
  )
}
