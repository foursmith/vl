import { createFileRoute } from "@tanstack/solid-router"
import { pageLinks } from "../docs-paths"
import { Landing } from "../landing"

export const Route = createFileRoute("/")({
  head: () => ({ links: pageLinks("/", "en") }),
  component: () => <Landing locale="en" />,
})
