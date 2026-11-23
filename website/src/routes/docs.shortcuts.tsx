import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../docs/KEYBINDINGS.md?vl-doc"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/docs/shortcuts")({
  head: () => docHead(docs["en"].shortcuts, "en"),
  component: () => <DocPage content={content} locale="en" />,
})
