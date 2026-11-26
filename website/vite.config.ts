import { mkdir, writeFile } from "node:fs/promises"
import { dirname, resolve } from "node:path"
import solid from "@solidjs/vite-plugin"
import tailwindcss from "@tailwindcss/vite"
import { tanstackStart } from "@tanstack/solid-start/plugin/vite"
import { defineConfig } from "vite"
import { renderMarkdownDocs } from "./markdown.ts"
import { sitePaths, siteUrl } from "./src/docs-paths.ts"
import { localeCodes, localizedPath, searchCorpusPath } from "./src/locales.ts"

let clientOutputDirectory: string

export default defineConfig({
  environments: {
    ssr: { build: { emitAssets: true } },
  },
  plugins: [
    tailwindcss(),
    renderMarkdownDocs(),
    {
      name: "vl-prerender-screenshots",
      apply: "build",
      configResolved(config) {
        clientOutputDirectory = resolve(config.root, config.environments.client.build.outDir)
      },
      async writeBundle(_options, bundle) {
        if (this.environment.name !== "ssr") return
        // The screenshots are imported by the server-rendered homepage.
        await Promise.all(Object.values(bundle).map(async (asset) => {
          if (asset.type !== "asset" || !asset.fileName.endsWith(".webp")) return
          const output = resolve(clientOutputDirectory, asset.fileName)
          await mkdir(dirname(output), { recursive: true })
          await writeFile(output, asset.source)
        }))
      },
    },
    {
      name: "vl-sitemap",
      generateBundle() {
        if (this.environment.name !== "client") return
        const urls = sitePaths.map((path) => `  <url><loc>${new URL(path, siteUrl).href}</loc></url>`)
        this.emitFile({
          type: "asset",
          fileName: "sitemap.xml",
          source: [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
            ...urls,
            "</urlset>",
            "",
          ].join("\n"),
        })
      },
    },
    tanstackStart({
      client: { entry: "client.ts" },
      router: { codeSplittingOptions: { defaultBehavior: [] } },
      pages: [...sitePaths, ...localeCodes.map((locale) => localizedPath(searchCorpusPath, locale))].map((path) => ({ path })),
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
