import { createFileRoute } from "@tanstack/solid-router"
import { pageLinks } from "../docs-paths"
import { Landing } from "../landing"

export const Route = createFileRoute("/zh-CN/")({
  head: () => ({ links: pageLinks("/zh-CN/", "zh-CN") }),
  component: () => <Landing locale="zh-CN" />,
})
