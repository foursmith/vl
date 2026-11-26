import { createFileRoute, notFound } from "@tanstack/solid-router"
import { DocPage, docHead } from "../doc-page"
import { findDoc } from "../docs-paths"

export const Route = createFileRoute("/{-$locale}/docs/$slug")({
  beforeLoad: ({ context: { locale }, params: { slug } }) => {
    const doc = findDoc(locale, slug)
    if (!doc) throw notFound()
    return { doc }
  },
  head: ({ match: { context: { locale, doc } } }) => doc ? docHead(doc, locale) : {},
  component: Page,
})

function Page() {
  const context = Route.useRouteContext()
  return <DocPage doc={context().doc} locale={context().locale} />
}
