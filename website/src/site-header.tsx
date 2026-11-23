import { useLocation } from "@tanstack/solid-router"
import logo from "../../assets/logo.svg?url"
import { docs, repository } from "./docs-paths"
import { localizedPath, locales, type Locale } from "./locales"

export function SiteHeader(props: { locale: Locale }) {
  const pathname = useLocation({ select: (location) => location.pathname })
  const labels = locales[props.locale]
  const home = localizedPath("/", props.locale)
  const otherLocale = props.locale === "en" ? "zh-CN" : "en"

  return (
    <header class="sticky top-0 z-10 border-b border-line bg-canvas">
      <div class="page-gutter flex min-h-14 items-center justify-between gap-3">
        <a href={home} aria-label={labels.home}>
          <img src={logo} alt="" width="40" height="24" class="h-auto w-10" />
        </a>
        <nav class="flex items-center gap-[clamp(0.7rem,3vw,1.75rem)] whitespace-nowrap text-sm font-medium text-muted" aria-label={labels.navigation}>
          <a class="site-link" href={`${home}#features`}>{labels.workbench}</a>
          <a class="site-link" href={docs[props.locale].install.path}>{labels.docs}</a>
          <a class="site-link text-ink" href={repository}>GitHub</a>
          <a class="site-link" href={localizedPath(pathname(), otherLocale)} lang={otherLocale} hreflang={otherLocale}>
            {otherLocale === "en" ? "English" : "简体中文"}
          </a>
        </nav>
      </div>
    </header>
  )
}
