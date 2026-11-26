import { createFileRoute } from "@tanstack/solid-router"
import { DocsLayout } from "../docs-layout"

export const Route = createFileRoute("/{-$locale}/docs")({
  component: Docs,
})

function Docs() {
  const locale = Route.useRouteContext({ select: (context) => context.locale })
  return <DocsLayout locale={locale()} />
}
