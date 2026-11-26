import { useLocation } from "@tanstack/solid-router"
import logo from "../../assets/logo.svg?url"
import { docs, repository } from "./docs-paths"
import { ChevronDown, SearchTriggerContent } from "./header-parts"
import { localizedPath, locales, type Locale } from "./locales"

export function SiteHeader(props: { locale: Locale }) {
  const pathname = useLocation({ select: (location) => location.pathname })
  const labels = locales[props.locale]
  const pages = docs[props.locale]
  const home = localizedPath("/", props.locale)
  const otherLocale = props.locale === "en" ? "zh-CN" : "en"
  const language = otherLocale === "en" ? "English" : "简体中文"

  return (
    <header class="sticky top-0 z-10 border-b border-line bg-canvas">
      <div class="page-gutter grid grid-cols-[auto_1fr] grid-rows-[3rem_2.5rem] items-center gap-x-4 whitespace-nowrap sm:min-h-14 sm:grid-cols-[auto_auto_1fr] sm:grid-rows-1 sm:gap-x-6">
        <a class="col-start-1 row-start-1" href={home} aria-label={labels.home}>
          <img src={logo} alt="" width="40" height="24" class="h-auto w-8 sm:w-10" />
        </a>
        <nav class="col-span-2 row-start-2 flex items-center justify-between gap-3 text-sm font-medium text-muted sm:col-start-2 sm:col-span-1 sm:row-start-1 sm:justify-start sm:gap-5" aria-label={labels.navigation}>
          <a class="site-link" href={pages.install.path}>{pages.install.title}</a>
          <a class="site-link" href={pages.usage.path}>{pages.usage.title}</a>
          <a class="site-link" href={pages.shortcuts.path}>{labels.shortcuts}</a>
          <div data-docs-menu="">
            <a class="flex items-center gap-1 hover:text-accent" href={pages.usage.path}>
              {labels.docs}
              <ChevronDown />
            </a>
          </div>
        </nav>
        <div class="col-start-2 row-start-1 flex items-center justify-self-end gap-0.5 sm:col-start-3 sm:gap-1">
          <div class="h-9 w-8 sm:w-9 md:w-52" data-doc-search="">
            <button class="header-icon doc-search-button" type="button" aria-label={labels.searchDocs} aria-haspopup="dialog">
              <SearchTriggerContent label={labels.searchDocs} />
            </button>
            <p class="sr-only" role="status" data-search-unavailable={labels.searchUnavailable}></p>
          </div>
          <a class="header-icon" href={repository} aria-label="GitHub" title="GitHub">
            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 .75a11.25 11.25 0 0 0-3.56 21.92c.56.1.77-.24.77-.54v-2.09c-3.13.68-3.79-1.33-3.79-1.33-.51-1.3-1.25-1.65-1.25-1.65-1.02-.7.08-.69.08-.69 1.13.08 1.72 1.15 1.72 1.15 1 1.72 2.64 1.22 3.28.93.1-.72.39-1.22.71-1.5-2.5-.29-5.12-1.25-5.12-5.57 0-1.23.44-2.24 1.15-3.03-.11-.28-.5-1.43.11-2.98 0 0 .94-.3 3.09 1.15a10.8 10.8 0 0 1 5.62 0c2.15-1.46 3.09-1.15 3.09-1.15.61 1.55.22 2.7.11 2.98.72.79 1.15 1.8 1.15 3.03 0 4.33-2.63 5.28-5.14 5.56.4.35.77 1.03.77 2.08v3.11c0 .3.2.65.77.54A11.25 11.25 0 0 0 12 .75Z" />
            </svg>
          </a>
          <a class="header-icon" href={localizedPath(pathname(), otherLocale)} lang={otherLocale} hreflang={otherLocale} aria-label={language} title={language}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M3 6h11m-5.5 0V3M5 9c1.4 2.2 3.5 4.2 6 5.5M12 6c-1.7 4.5-4.6 7.6-8 9.5M12 21l4.5-10 4.5 10m-7.4-3.5h5.8" />
            </svg>
          </a>
          <button class="header-icon" type="button" data-toggle-site-theme="" aria-pressed="false" aria-label={labels.lightMode} title={labels.lightMode}>
            <svg data-theme-icon="dark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M20 15.4A8.5 8.5 0 0 1 8.6 4a8.5 8.5 0 1 0 11.4 11.4Z" />
            </svg>
            <svg data-theme-icon="light" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" aria-hidden="true">
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2m0 16v2M2 12h2m16 0h2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4m0-14.2-1.4 1.4M6.3 17.7l-1.4 1.4" />
            </svg>
          </button>
        </div>
      </div>
    </header>
  )
}
