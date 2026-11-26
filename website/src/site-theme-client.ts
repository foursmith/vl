import { themeStorageKey, type SiteTheme } from "./site-theme"

export function enableSiteTheme() {
  const root = document.documentElement
  const button = document.querySelector<HTMLButtonElement>("[data-toggle-site-theme]")
  const lightPreviews = document.querySelectorAll<HTMLSourceElement>("[data-light-preview]")
  const browserColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')

  function applyTheme(theme: SiteTheme) {
    root.dataset.theme = theme
    button?.setAttribute("aria-pressed", String(theme === "light"))
    lightPreviews.forEach((source) => { source.media = theme === "light" ? "all" : "not all" })
    if (browserColor) browserColor.content = getComputedStyle(root).getPropertyValue("--color-canvas").trim()
  }

  applyTheme(root.dataset.theme === "light" ? "light" : "dark")

  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return
    try {
      applyTheme(localStorage.getItem(themeStorageKey) === "light" ? "light" : "dark")
    } catch {
      // Keep the current theme if storage is unavailable.
    }
  })

  button?.addEventListener("click", () => {
    const theme = root.dataset.theme === "light" ? "dark" : "light"
    applyTheme(theme)
    try {
      localStorage.setItem(themeStorageKey, theme)
    } catch {
      // Theme switching still works on the current page if storage is unavailable.
    }
  })
}
