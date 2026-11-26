import { siteThemes, themeStorageKey } from "./site-theme"

export function enableSiteTheme() {
  const root = document.documentElement
  const buttons = document.querySelectorAll<HTMLButtonElement>("[data-site-theme]")
  const lightPreviews = document.querySelectorAll<HTMLSourceElement>("[data-light-preview]")
  const browserColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')

  function applyTheme(theme: (typeof siteThemes)[number]) {
    root.dataset.theme = theme
    buttons.forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.siteTheme === theme)))
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

  buttons.forEach((button, index) => button.addEventListener("click", () => {
    const theme = siteThemes[index]
    if (root.dataset.theme === theme) return
    applyTheme(theme)
    try {
      localStorage.setItem(themeStorageKey, theme)
    } catch {
      // Theme switching still works on the current page if storage is unavailable.
    }
  }))
}
