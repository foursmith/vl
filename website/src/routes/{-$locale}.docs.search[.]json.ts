import { createFileRoute } from "@tanstack/solid-router"
import documents from "virtual:vl-documents"
import { docs, type SearchDocument } from "../docs-paths"

export const Route = createFileRoute("/{-$locale}/docs/search.json")({
  server: {
    handlers: {
      GET: ({ params }) => {
        const prefix = params.locale
        if (prefix !== undefined && prefix !== "zh-CN") return new Response(null, { status: 404 })
        const locale = prefix ?? "en"
        const corpus = Object.values(docs[locale]).map((doc): SearchDocument =>
          [doc.path, doc.title, documents[doc.source].searchSections],
        )
        return Response.json(corpus)
      },
    },
  },
})
