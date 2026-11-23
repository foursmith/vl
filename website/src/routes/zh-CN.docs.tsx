import { createFileRoute } from "@tanstack/solid-router"
import { DocsLayout } from "../docs-layout"

export const Route = createFileRoute("/zh-CN/docs")({
  component: () => <DocsLayout locale="zh-CN" />,
})
