import { createCodeCopyHandler } from "./code-copy"
import { docs, siteUrl, type RenderedDocument } from "./docs-paths"

type Doc = (typeof docs)[keyof typeof docs]

export function docHead(doc: Doc) {
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
    links: [{ rel: "canonical", href: url }],
  }
}

export function DocPage(props: { content: RenderedDocument }) {
  const copyCode = createCodeCopyHandler()

  return (
    <>
      <details class="page-gutter border-b border-line py-3 md:hidden">
        <summary class="cursor-pointer text-sm font-medium">On this page</summary>
        <SectionLinks headings={props.content.headings} />
      </details>
      <div class="grid md:grid-cols-[minmax(0,1fr)_14rem] lg:grid-cols-[minmax(0,1fr)_17rem]">
        <article class="doc-body page-gutter min-w-0 py-7 sm:py-10" onClick={copyCode} innerHTML={props.content.html} />
        <aside class="hidden min-w-0 border-s border-line md:block">
          <div class="doc-outline sticky overflow-auto px-5 py-8 lg:px-6">
            <h2 class="text-sm font-semibold">On this page</h2>
            <SectionLinks headings={props.content.headings} />
          </div>
        </aside>
      </div>
    </>
  )
}

function SectionLinks(props: { headings: RenderedDocument["headings"] }) {
  return (
    <nav aria-label="On this page" class="mt-2">
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
