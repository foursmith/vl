import { readFile } from "node:fs/promises"
import { posix, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import type { Root } from "hast"
import { toString } from "hast-util-to-string"
import { h } from "hastscript"
import rehypeShiki from "@shikijs/rehype"
import rehypeSanitize, { defaultSchema } from "rehype-sanitize"
import rehypeStringify from "rehype-stringify"
import remarkGfm from "remark-gfm"
import remarkParse from "remark-parse"
import remarkRehype from "remark-rehype"
import { unified } from "unified"
import { SKIP, visit } from "unist-util-visit"
import { normalizePath, type Plugin } from "vite"
import darkModern from "../internal/theme/themes/dark_modern.json" with { type: "json" }
import darkPlus from "../internal/theme/themes/dark_plus.json" with { type: "json" }
import darkVs from "../internal/theme/themes/dark_vs.json" with { type: "json" }
import lightModern from "../internal/theme/themes/light_modern.json" with { type: "json" }
import lightPlus from "../internal/theme/themes/light_plus.json" with { type: "json" }
import lightVs from "../internal/theme/themes/light_vs.json" with { type: "json" }
import { docs, repository, type Document, type RenderedDocument } from "./src/docs-paths.ts"
import { localeCodes, locales, type Locale } from "./src/locales.ts"

const repoRoot = fileURLToPath(new URL("../", import.meta.url))
const docsBySource = new Map<string, { doc: Document; locale: Locale }>()
for (const locale of localeCodes) {
  for (const doc of Object.values(docs[locale])) {
    docsBySource.set(doc.source, { doc, locale })
  }
}

// Shiki needs the inherited TextMate rules alongside each Modern workbench palette.
const themes = {
  dark: {
    name: "dark-modern",
    type: "dark" as const,
    colors: { ...darkVs.colors, ...darkModern.colors },
    tokenColors: [...darkVs.tokenColors, ...darkPlus.tokenColors],
  },
  light: {
    name: "light-modern",
    type: "light" as const,
    colors: { ...lightVs.colors, ...lightModern.colors },
    tokenColors: [...lightVs.tokenColors, ...lightPlus.tokenColors],
  },
}

function createMarkdown(locale: Locale) {
  return unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkRehype, locale === "zh-CN" ? { footnoteLabel: "脚注", footnoteBackLabel: "返回脚注引用" } : {})
    // remark-rehype drops raw HTML and already prefixes the IDs it generates for footnotes.
    .use(rehypeSanitize, { ...defaultSchema, clobberPrefix: "" })
    .use(rehypeShiki, {
      themes,
      defaultColor: false,
      langs: [],
      lazy: true,
      fallbackLanguage: "text",
      addLanguageClass: true,
    })
    .use(rehypeStringify)
}

const markdownByLocale = { en: createMarkdown("en"), "zh-CN": createMarkdown("zh-CN") }

async function renderDocument(doc: Document, text: string, locale: Locale): Promise<RenderedDocument> {
  const markdown = markdownByLocale[locale]
  const tree: Root = await markdown.run(markdown.parse(text))
  const headings: RenderedDocument["headings"] = []
  const usedIds = new Set<string>()
  const { referenceTable, copyCode } = locales[locale]

  visit(tree, "element", ({ properties }) => {
    if (typeof properties.id === "string") usedIds.add(properties.id)
  })

  visit(tree, "element", (node) => {
    if (/^h[1-6]$/.test(node.tagName) && node.properties.id === undefined) {
      const title = toString(node)
      const base = title.toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, "").replace(/\s+/g, "-")
      let id = base
      for (let suffix = 1; usedIds.has(id); suffix++) id = `${base}-${suffix}`
      usedIds.add(id)
      node.properties.id = id
      const depth = Number(node.tagName.slice(1))
      if (depth > 1) headings.push({ id, title, depth })
    }

    const { href } = node.properties
    if (node.tagName !== "a" || typeof href !== "string" || /^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(href)) return
    const [target, fragment] = href.split("#", 2)
    const sourcePath = posix.join(posix.dirname(doc.source), target)
    const targetPath = docsBySource.get(sourcePath)?.doc.path ?? `${repository}/blob/main/${sourcePath}`
    node.properties.href = targetPath + (fragment ? `#${fragment}` : "")
  })

  visit(tree, "element", (node, index, parent) => {
    if (index === undefined || !parent) return
    if (node.tagName === "table") {
      parent.children[index] = h("div.doc-table", { tabIndex: 0, role: "region", ariaLabel: referenceTable }, [node])
      return SKIP
    }
    if (node.tagName === "pre") {
      parent.children[index] = h("div.code-box.code-block", [
        node,
        h("button.control.control-icon", { type: "button", dataCopyCode: "", ariaLabel: copyCode, title: copyCode }, [
          h("span.copy-icon", { ariaHidden: "true" }),
        ]),
        h("span.sr-only", { role: "status" }),
      ])
      return SKIP
    }
  })

  return { html: markdown.stringify(tree), headings }
}

export function renderMarkdownDocs(): Plugin {
  const moduleId = "virtual:vl-documents"
  const resolvedId = `\0${moduleId}`

  return {
    name: "vl-markdown-docs",
    enforce: "pre",
    resolveId(id) {
      if (id === moduleId) return resolvedId
    },
    async load(id) {
      if (id === resolvedId) {
        const sources = [...docsBySource.keys()]
        const imports = sources.map((source, index) => {
          const path = normalizePath(resolve(repoRoot, source))
          return `import doc${index} from ${JSON.stringify(`${path}?vl-doc`)}`
        })
        const entries = sources.map((source, index) => `${JSON.stringify(source)}: doc${index}`)
        return `${imports.join("\n")}\nexport default { ${entries.join(", ")} }`
      }
      const [file, query] = id.split("?")
      if (query !== "vl-doc") return
      const source = relative(repoRoot, file).split("\\").join("/")
      const entry = docsBySource.get(source)
      if (!entry) return
      const { doc, locale } = entry
      this.addWatchFile(file)
      const document = await renderDocument(doc, await readFile(file, "utf8"), locale)
      return `export default ${JSON.stringify(document)}`
    },
  }
}
