import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../docs/VIM.md?vl-doc"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/docs/vim")({
  head: () => docHead(docs["en"].vim, "en"),
  component: () => <DocPage content={content} locale="en" />,
})
