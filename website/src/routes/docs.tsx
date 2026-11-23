import { createFileRoute } from "@tanstack/solid-router"
import { DocsLayout } from "../docs-layout"

export const Route = createFileRoute("/docs")({
  component: () => <DocsLayout locale="en" />,
})
