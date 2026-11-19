import { readFile } from "node:fs/promises"
import { posix, relative } from "node:path"
import { fileURLToPath } from "node:url"
import MarkdownIt from "markdown-it"
import type { Plugin } from "vite"
import { docs, repository, type RenderedDocument } from "./src/docs-paths.ts"

const repoRoot = fileURLToPath(new URL("../", import.meta.url))
const markdown = new MarkdownIt({ html: false })
const docsBySource = new Map<string, string>([
  [docs.usage.source, docs.usage.path],
  [docs.shortcuts.source, docs.shortcuts.path],
  [docs.vim.source, docs.vim.path],
])

markdown.renderer.rules.table_open = () => '<div class="doc-table" tabindex="0" role="region" aria-label="Reference table"><table>\n'
markdown.renderer.rules.table_close = () => "</table></div>\n"

for (const rule of ["fence", "code_block"] as const) {
  const renderCode = markdown.renderer.rules[rule]!
  markdown.renderer.rules[rule] = (...args) => `<div class="code-box code-block">
${renderCode(...args)}
<button type="button" class="control control-icon" data-copy-code aria-label="Copy code" title="Copy code"><span class="copy-icon" aria-hidden="true"></span></button>
<span class="sr-only" role="status"></span>
</div>\n`
}

function renderDocument(source: string, text: string, section?: "Install" | "Usage"): RenderedDocument {
  if (section) {
    const match = text.match(new RegExp(`^## ${section}\\r?\\n[\\s\\S]*?(?=^## |(?![\\s\\S]))`, "m"))
    if (!match) throw new Error(`${source} is missing its ${section} section`)
    text = match[0]
  }

  const env = {}
  const tokens = markdown.parse(text, env)
  const headings: RenderedDocument["headings"] = []
  const usedIds = new Set<string>()

  for (const [index, token] of tokens.entries()) {
    if (section && (token.type === "heading_open" || token.type === "heading_close")) {
      token.tag = `h${Number(token.tag.slice(1)) - 1}`
    }
    if (token.type === "heading_open") {
      const title = tokens[index + 1].content
      const base = title.toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, "").replace(/\s+/g, "-")
      let id = base
      for (let suffix = 1; usedIds.has(id); suffix++) id = `${base}-${suffix}`
      usedIds.add(id)
      token.attrSet("id", id)
      const depth = Number(token.tag.slice(1))
      if (depth > 1) headings.push({ id, title, depth })
    }

    for (const child of token.children ?? []) {
      if (child.type !== "link_open") continue
      const href = child.attrGet("href")
      if (typeof href !== "string" || /^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(href)) continue
      const [target, fragment] = href.split("#", 2)
      const sourcePath = posix.join(posix.dirname(source), target)
      const targetPath = docsBySource.get(sourcePath) ?? `${repository}/blob/main/${sourcePath}`
      child.attrSet("href", targetPath + (fragment ? `#${fragment}` : ""))
    }
  }

  return { html: markdown.renderer.render(tokens, markdown.options, env), headings }
}

export function renderMarkdownDocs(): Plugin {
  return {
    name: "vl-markdown-docs",
    enforce: "pre",
    async load(id) {
      const [file, query] = id.split("?")
      if (query !== "vl-doc" && query !== "vl-doc-install") return
      const source = relative(repoRoot, file).split("\\").join("/")
      if (!docsBySource.has(source)) return
      if (query === "vl-doc-install" && source !== docs.install.source) return
      const readmeSection = query === "vl-doc-install" ? docs.install.section : docs.usage.section
      const section = source === docs.usage.source ? readmeSection : undefined
      this.addWatchFile(file)
      const document = renderDocument(source, await readFile(file, "utf8"), section)
      return `export default ${JSON.stringify(document)}`
    },
  }
}
