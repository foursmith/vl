export type SiteTheme = "dark" | "light"
export const themeStorageKey = "vl-site-theme"

export const themeInit = `try {
  if (localStorage.getItem(${JSON.stringify(themeStorageKey)}) === "light") {
    document.documentElement.dataset.theme = "light"
  }
} catch {}`
