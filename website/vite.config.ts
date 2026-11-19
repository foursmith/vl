import solid from "@solidjs/vite-plugin"
import tailwindcss from "@tailwindcss/vite"
import { tanstackStart } from "@tanstack/solid-start/plugin/vite"
import { defineConfig } from "vite"
import { renderMarkdownDocs } from "./markdown.ts"
import { docs } from "./src/docs-paths.ts"

export default defineConfig({
  plugins: [
    tailwindcss(),
    renderMarkdownDocs(),
    tanstackStart({
      pages: ["/", ...Object.values(docs).map(({ path }) => path)].map((path) => ({ path })),
      prerender: {
        enabled: true,
        autoStaticPathsDiscovery: false,
        crawlLinks: false,
        autoSubfolderIndex: false,
      },
    }),
    solid({ ssr: true }),
  ],
})
