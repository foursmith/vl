import { render } from "@solidjs/web"
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from "@ui-sdk/solid/command"
import { Dialog, DialogBackdrop, DialogClose, DialogContent, DialogShortcut, DialogTitle, DialogTrigger } from "@ui-sdk/solid/dialog"
import { createSignal, For, Show } from "solid-js"
import { SearchIcon, SearchTriggerContent } from "./header-parts"
import { locales, type Locale } from "./locales"
import { loadDocumentSearch, type SearchResult } from "./search-documents"

function DocSearch(props: { locale: Locale }) {
  const labels = locales[props.locale]
  const [open, setOpen] = createSignal(false)
  const [query, setQuery] = createSignal("")
  const [results, setResults] = createSignal<SearchResult[]>([])
  const [status, setStatus] = createSignal<"idle" | "loading" | "error">("idle")
  let revision = 0

  async function updateQuery(value: string) {
    setQuery(value)
    setResults([])
    const requested = ++revision
    const term = value.trim()
    setStatus(term ? "loading" : "idle")
    if (!term) return

    try {
      const find = await loadDocumentSearch(props.locale)
      if (requested !== revision) return
      setResults(find(term))
      setStatus("idle")
    } catch {
      if (requested === revision) setStatus("error")
    }
  }

  return (
    <Dialog open={open()} onOpenChange={setOpen}>
      <DialogTrigger class="header-icon doc-search-button" shortcut="mod+k" aria-label={labels.searchDocs}>
        <SearchTriggerContent label={labels.searchDocs} shortcut={<DialogShortcut />} />
      </DialogTrigger>
      <DialogBackdrop class="doc-search-backdrop" />
      <DialogContent class="doc-search" ref={(element) => {
        element.addEventListener("keydown", (event) => {
          if (event.key !== "Escape" || event.isComposing) return
          event.preventDefault()
          event.stopPropagation()
          setOpen(false)
        }, { capture: true })
      }}>
        <DialogTitle class="sr-only">{labels.searchDocs}</DialogTitle>
        <Command query={query()} onQueryChange={updateQuery} filter={false} autoHighlight loop status={status()} onAction={() => setOpen(false)}>
          <div class="doc-search-field">
            <SearchIcon />
            <CommandInput class="min-w-0 flex-1 py-2 text-base text-ink focus-visible:outline-none" aria-label={labels.searchDocs} placeholder={labels.searchDocs + "…"} />
            <DialogClose class="control px-1" aria-label={labels.closeSearch}>
              <kbd aria-hidden="true">Esc</kbd>
            </DialogClose>
          </div>
          <Show when={query().trim() && status() === "idle" && results().length === 0}>
            <CommandEmpty class="px-4 py-5 text-sm text-muted">{labels.noSearchResults}</CommandEmpty>
          </Show>
          <Show when={status() === "error"}>
            <p class="px-4 py-5 text-sm text-muted" role="status">{labels.searchUnavailable}</p>
          </Show>
          <CommandList class="doc-search-results" aria-label={labels.searchDocs}>
            <For each={results()}>{(result) => (
              <CommandItem value={result.url} label={result.title} render={(attributes) => <a {...attributes} href={result.url} />}>
                <strong>{result.title}</strong>
                <Show when={result.heading !== result.title}><span>{result.heading}</span></Show>
                <Show when={result.excerpt}><p>{result.excerpt}</p></Show>
              </CommandItem>
            )}</For>
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  )
}

export function mountDocSearch(container: HTMLElement, locale: Locale) {
  container.replaceChildren()
  render(() => <DocSearch locale={locale} />, container)
  return container.querySelector<HTMLButtonElement>('[data-slot="dialog-trigger"]')!
}
