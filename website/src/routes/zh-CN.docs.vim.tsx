import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../docs/VIM.zh-CN.md?vl-doc"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/zh-CN/docs/vim")({
  head: () => docHead(docs["zh-CN"].vim, "zh-CN"),
  component: () => <DocPage content={content} locale="zh-CN" />,
})
