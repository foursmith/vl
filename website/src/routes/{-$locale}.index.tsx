import { createFileRoute } from "@tanstack/solid-router"
import { pageLinks } from "../docs-paths"
import { Landing } from "../landing"

export const Route = createFileRoute("/{-$locale}/")({
  head: ({ match: { context: { locale } } }) => ({ links: pageLinks("/", locale) }),
  component: Home,
})

function Home() {
  const locale = Route.useRouteContext({ select: (context) => context.locale })
  return <Landing locale={locale()} />
}
