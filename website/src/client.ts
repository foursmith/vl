import "./styles.css"
import type { Locale } from "./locales"
import { enableSiteTheme } from "./site-theme-client"

// Navigation and page content are already in the prerendered HTML.
const wordmark = document.querySelector<HTMLElement>("[data-wordmark]")
if (wordmark) void import("./wordmark-client").then(({ enableWordmark }) => enableWordmark(wordmark))

enableSiteTheme()

const search = document.querySelector<HTMLDialogElement>("#doc-search")
if (search) void import("./doc-search-client").then(({ enableDocSearch }) => enableDocSearch(search))

const article = document.querySelector<HTMLElement>("article.doc-body")
if (article?.querySelector("[data-copy-code]")) {
  const locale = document.documentElement.lang as Locale
  void import("./code-copy").then(({ createCodeCopyHandler }) => {
    article.addEventListener("click", createCodeCopyHandler(locale))
  })
}
