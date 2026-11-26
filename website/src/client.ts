import "./styles.css"
import type { Locale } from "./locales"
import { enableSiteTheme } from "./site-theme-client"

const locale = document.documentElement.lang as Locale

// Navigation and page content are already in the prerendered HTML.
const wordmark = document.querySelector<HTMLElement>("[data-wordmark]")
if (wordmark) void import("./wordmark-client").then(({ enableWordmark }) => enableWordmark(wordmark))

enableSiteTheme()

const search = document.querySelector<HTMLElement>("[data-doc-search]")
if (search) prepareDocSearch(search)

const docsMenu = document.querySelector<HTMLElement>("[data-docs-menu]")
if (docsMenu) prepareDocsMenu(docsMenu)

const article = document.querySelector<HTMLElement>("article.doc-body")
if (article?.querySelector("[data-copy-code]")) {
  void import("./code-copy").then(({ createCodeCopyHandler }) => {
    article.addEventListener("click", createCodeCopyHandler(locale))
  })
}

function prepareDocSearch(container: HTMLElement) {
  const button = container.querySelector("button")!
  const shortcut = button.querySelector("kbd")!
  const status = container.querySelector<HTMLElement>("[data-search-unavailable]")!
  const isApple = /Mac|iPhone|iPad|iPod/i.test(navigator.platform)
  let loading = false

  if (isApple) shortcut.textContent = "⌘K"
  button.setAttribute("aria-keyshortcuts", isApple ? "Meta+K" : "Control+K")
  button.addEventListener("click", openSearch)
  document.addEventListener("keydown", handleShortcut)

  function handleShortcut(event: KeyboardEvent) {
    if (!(isApple ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey)) return
    if (event.isComposing || event.altKey || event.shiftKey || event.key.toLowerCase() !== "k") return
    event.preventDefault()
    openSearch()
  }

  function openSearch() {
    if (loading) return
    loading = true
    status.textContent = ""
    button.setAttribute("aria-busy", "true")
    void import("./doc-search").then(({ mountDocSearch }) => {
      const trigger = mountDocSearch(container, locale)
      document.removeEventListener("keydown", handleShortcut)
      trigger.focus({ preventScroll: true })
      trigger.click()
    }, () => {
      loading = false
      button.removeAttribute("aria-busy")
      status.textContent = status.dataset.searchUnavailable ?? ""
    })
  }
}

function prepareDocsMenu(container: HTMLElement) {
  const link = container.querySelector("a")!
  let loading = false

  link.setAttribute("aria-haspopup", "menu")
  link.addEventListener("click", (event) => {
    if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return
    event.preventDefault()
    openMenu()
  })
  link.addEventListener("keydown", (event) => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp" && event.key !== " ") return
    event.preventDefault()
    openMenu(event.key === " " ? undefined : event.key)
  })

  function openMenu(key?: "ArrowDown" | "ArrowUp") {
    if (loading) return
    loading = true
    link.setAttribute("aria-busy", "true")
    void import("./docs-menu").then(({ mountDocsMenu }) => {
      const trigger = mountDocsMenu(container, locale)
      trigger.focus({ preventScroll: true })
      if (key) trigger.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }))
      else trigger.click()
    }, () => {
      loading = false
      link.removeAttribute("aria-busy")
      location.assign(link.href)
    })
  }
}
