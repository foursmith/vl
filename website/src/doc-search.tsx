import { locales, type Locale } from "./locales"

export function DocSearchButton(props: { locale: Locale }) {
  return (
    <button class="doc-search-button" type="button" aria-haspopup="dialog" aria-controls="doc-search" data-open-doc-search="">
      {locales[props.locale].searchDocs}
    </button>
  )
}

export function DocSearch(props: { locale: Locale }) {
  const labels = locales[props.locale]
  return (
    <dialog id="doc-search" class="doc-search" aria-label={labels.searchDocs}>
      <div class="flex items-center gap-2 p-3">
        <input
          class="min-w-0 flex-1 border border-line bg-canvas px-3 py-2 text-sm"
          type="search"
          aria-label={labels.searchDocs}
          placeholder={labels.searchDocs}
          autocomplete="off"
          autofocus
        />
        <form method="dialog">
          <button class="control control-icon text-xl" type="submit" aria-label={labels.closeSearch}>×</button>
        </form>
      </div>
      <p class="px-4 pb-3 text-sm text-muted empty:hidden" role="status"></p>
      <ul class="doc-search-results"></ul>
    </dialog>
  )
}
