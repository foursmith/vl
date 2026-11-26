import { createFileRoute, notFound } from "@tanstack/solid-router"
import type { Locale } from "../locales"

export const Route = createFileRoute("/{-$locale}")({
  beforeLoad: ({ params }) => {
    const prefix = params.locale
    if (prefix !== undefined && prefix !== "zh-CN") throw notFound()

    const locale: Locale = prefix ?? "en"
    return { locale }
  },
})
