import { locales, type Locale } from "./locales"

type SearchMatch = { title: string; url: string; plain_excerpt: string }
type SearchPage = {
  url: string
  meta: { title: string; url?: string }
  plain_excerpt: string
  sub_results: SearchMatch[]
}
type SearchIndex = {
  init(): Promise<void>
  search(query: string): Promise<{ results: Array<{ data(): Promise<SearchPage> }> }>
}

const indexPath = "/pagefind/pagefind.js"

export function enableDocSearch(dialog: HTMLDialogElement) {
  const input = dialog.querySelector("input")!
  const results = dialog.querySelector("ul")!
  const status = dialog.querySelector<HTMLElement>("[role=status]")!
  const labels = locales[document.documentElement.lang as Locale]
  let index: Promise<SearchIndex> | undefined
  let timer: ReturnType<typeof setTimeout>
  let revision = 0

  function loadIndex() {
    index ??= import(/* @vite-ignore */ indexPath)
      .then(async (module: SearchIndex) => {
        await module.init()
        return module
      })
      .catch((error: unknown) => {
        index = undefined
        throw error
      })
    return index
  }

  async function search(query: string, requested: number) {
    try {
      const { results: matches } = await (await loadIndex()).search(query)
      const pages = await Promise.all(matches.slice(0, 8).map((match) => match.data()))
      if (requested !== revision) return

      results.replaceChildren(...pages.map(searchResult))
      status.textContent = pages.length ? "" : labels.noSearchResults
    } catch {
      if (requested === revision) status.textContent = labels.searchUnavailable
    }
  }

  for (const button of document.querySelectorAll("[data-open-doc-search]")) {
    button.addEventListener("click", () => {
      dialog.showModal()
      void loadIndex().catch(() => {})
    })
  }

  function updateSearch() {
    clearTimeout(timer)
    const requested = ++revision
    const query = input.value.trim()
    results.replaceChildren()
    status.textContent = ""
    if (query) timer = setTimeout(() => void search(query, requested), 120)
  }

  input.addEventListener("input", (event) => {
    if (!(event as InputEvent).isComposing) updateSearch()
  })
  input.addEventListener("compositionend", updateSearch)
  input.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !event.isComposing) {
      event.preventDefault()
      dialog.close()
    }
  })
  results.addEventListener("click", (event) => {
    if (event.target instanceof Element && event.target.closest("a")) dialog.close()
  })
}

function searchResult(page: SearchPage) {
  const match = page.sub_results.find(({ url }) => url.includes("#")) ?? page.sub_results[0]
  const title = document.createElement("strong")
  title.textContent = page.meta.title
  const summary = document.createElement("p")
  summary.textContent = match?.plain_excerpt || page.plain_excerpt

  const link = document.createElement("a")
  link.href = match?.url ?? page.meta.url ?? page.url
  link.append(title)
  if (match?.title && match.title !== page.meta.title) {
    const section = document.createElement("span")
    section.textContent = match.title
    link.append(section)
  }
  link.append(summary)

  const item = document.createElement("li")
  item.append(link)
  return item
}
