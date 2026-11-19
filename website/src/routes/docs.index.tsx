import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../README.md?vl-doc"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/docs/")({
  head: () => docHead(docs.usage),
  component: () => <DocPage content={content} />,
})
