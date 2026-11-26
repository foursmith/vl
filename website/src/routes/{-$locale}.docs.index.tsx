import { createFileRoute } from "@tanstack/solid-router"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/{-$locale}/docs/")({
  head: ({ match: { context: { locale } } }) => docHead(docs[locale].usage, locale),
  component: Page,
})

function Page() {
  const locale = Route.useRouteContext({ select: (context) => context.locale })
  return <DocPage doc={docs[locale()].usage} locale={locale()} />
}
