import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../README.md?vl-doc-install"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/docs/install")({
  head: () => docHead(docs["en"].install, "en"),
  component: () => <DocPage content={content} locale="en" />,
})
