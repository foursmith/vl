import { createFileRoute } from "@tanstack/solid-router"
import content from "../../../README.zh-CN.md?vl-doc-install"
import { DocPage, docHead } from "../doc-page"
import { docs } from "../docs-paths"

export const Route = createFileRoute("/zh-CN/docs/install")({
  head: () => docHead(docs["zh-CN"].install, "zh-CN"),
  component: () => <DocPage content={content} locale="zh-CN" />,
})
