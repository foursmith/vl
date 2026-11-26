import { useLocation } from "@tanstack/solid-router"
import logo from "../../assets/logo.svg?url"
import { docs, repository } from "./docs-paths"
import { localizedPath, locales, type Locale } from "./locales"
import { siteThemes } from "./site-theme"

export function SiteHeader(props: { locale: Locale }) {
  const pathname = useLocation({ select: (location) => location.pathname })
  const labels = locales[props.locale]
  const { install, usage } = docs[props.locale]
  const home = localizedPath("/", props.locale)
  const otherLocale = props.locale === "en" ? "zh-CN" : "en"

  return (
    <header class="sticky top-0 z-10 border-b border-line bg-canvas">
      <div class="page-gutter grid grid-cols-[auto_1fr] grid-rows-[3rem_2.5rem] items-center gap-x-3 sm:min-h-14 sm:grid-cols-[auto_1fr_auto] sm:grid-rows-1 sm:gap-x-5">
        <a href={home} aria-label={labels.home}>
          <img src={logo} alt="" width="40" height="24" class="h-auto w-10" />
        </a>
        <div class="flex justify-self-end border border-line" role="group" aria-label={labels.theme}>
          {siteThemes.map((theme) => (
            <button class="tab site-theme-button" type="button" data-site-theme={theme} aria-pressed={theme === "dark" ? "true" : "false"}>
              {labels[theme]}
            </button>
          ))}
        </div>
        <nav class="col-span-2 flex items-center justify-between gap-3 whitespace-nowrap text-sm font-medium text-muted sm:col-span-1 sm:gap-[clamp(0.7rem,3vw,1.75rem)]" aria-label={labels.navigation}>
          <a class="site-link" href={install.path}>{install.title}</a>
          <a class="site-link" href={usage.path}>{labels.docs}</a>
          <a class="site-link text-ink" href={repository}>GitHub</a>
          <a class="site-link" href={localizedPath(pathname(), otherLocale)} lang={otherLocale} hreflang={otherLocale}>
            {otherLocale === "en" ? "English" : "简体中文"}
          </a>
        </nav>
      </div>
    </header>
  )
}
