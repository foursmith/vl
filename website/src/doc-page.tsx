import { pageLinks, siteUrl, type Document, type RenderedDocument } from "./docs-paths"
import { locales, type Locale } from "./locales"

export function docHead(doc: Document, locale: Locale) {
  const title = `${doc.title} · vl`
  const url = new URL(doc.path, siteUrl).href
  return {
    meta: [
      { title },
      { name: "description", content: doc.description },
      { property: "og:title", content: title },
      { property: "og:description", content: doc.description },
      { property: "og:url", content: url },
      { name: "twitter:title", content: title },
      { name: "twitter:description", content: doc.description },
    ],
    links: pageLinks(doc.path, locale),
  }
}

export function DocPage(props: { content: RenderedDocument; locale: Locale }) {
  const labels = locales[props.locale]

  return (
    <>
      <details class="page-gutter border-b border-line py-3 md:hidden">
        <summary class="cursor-pointer text-sm font-medium">{labels.onThisPage}</summary>
        <SectionLinks headings={props.content.headings} label={labels.onThisPage} />
      </details>
      <div class="grid md:grid-cols-[minmax(0,1fr)_14rem] lg:grid-cols-[minmax(0,1fr)_17rem]">
        <article class="doc-body page-gutter min-w-0 py-7 sm:py-10" innerHTML={props.content.html} />
        <aside class="hidden min-w-0 border-s border-line md:block">
          <div class="doc-outline sticky overflow-auto px-5 py-8 lg:px-6">
            <h2 class="text-sm font-semibold">{labels.onThisPage}</h2>
            <SectionLinks headings={props.content.headings} label={labels.onThisPage} />
          </div>
        </aside>
      </div>
    </>
  )
}

function SectionLinks(props: { headings: RenderedDocument["headings"]; label: string }) {
  return (
    <nav aria-label={props.label} class="mt-2">
      <ul class="space-y-1 text-sm text-muted">
        {props.headings.map(({ id, title, depth }) => (
          <li style={{ "padding-inline-start": `${Math.max(0, depth - 2) * 0.75}rem` }}>
            <a class="site-link inline-block py-1" href={`#${id}`}>{title}</a>
          </li>
        ))}
      </ul>
    </nav>
  )
}
