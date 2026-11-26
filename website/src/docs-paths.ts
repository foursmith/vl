import { localizedPath, localeCodes, type Locale } from "./locales.ts"

export const siteUrl = "https://vl.foursmith.com/"
export const repository = "https://github.com/foursmith/vl"

export function pageLinks(path: string, locale: Locale) {
  const english = new URL(localizedPath(path, "en"), siteUrl).href
  const chinese = new URL(localizedPath(path, "zh-CN"), siteUrl).href
  return [
    { rel: "canonical", href: locale === "en" ? english : chinese },
    { rel: "alternate", hreflang: "en", href: english },
    { rel: "alternate", hreflang: "zh-CN", href: chinese },
  ]
}

export type Document = {
  slug: string
  path: string
  source: string
  title: string
  description: string
}

export const docGroups = [
  {
    label: { en: "Getting started", "zh-CN": "入门" },
    pages: [
      {
        key: "usage",
        slug: "",
        file: "GETTING-STARTED.md",
        title: { en: "Quickstart", "zh-CN": "快速上手" },
        description: {
          en: "Open the VL workbench, find files, and review Git changes.",
          "zh-CN": "打开 VL 工作台，查找文件并审阅 Git 改动。",
        },
      },
      {
        key: "install",
        slug: "install",
        file: "INSTALL.md",
        title: { en: "Install", "zh-CN": "安装" },
        description: {
          en: "Install VL with Homebrew, Go, a release script, or a source build.",
          "zh-CN": "通过 Homebrew、Go、发布脚本或从源码构建来安装 VL。",
        },
      },
    ],
  },
  {
    label: { en: "Configuration", "zh-CN": "配置" },
    pages: [
      {
        key: "configuration",
        slug: "configuration",
        file: "CONFIGURATION.md",
        title: { en: "Configure VL", "zh-CN": "VL 配置" },
        description: {
          en: "Configure themes, keys, input methods, and integrations.",
          "zh-CN": "设置主题、键位、输入法和集成。",
        },
      },
      {
        key: "chineseWordSegmentation",
        slug: "chinese-word-segmentation",
        file: "CHINESE-WORD-SEGMENTATION.md",
        title: { en: "Chinese word segmentation", "zh-CN": "中文分词" },
        description: {
          en: "Set up a Jieba dictionary for Chinese word motions and double-click selection.",
          "zh-CN": "安装 Jieba 词典，按中文词语移动光标和双击选词。",
        },
      },
      {
        key: "inputMethod",
        slug: "input-method",
        file: "INPUT-METHOD.md",
        title: { en: "Input method switching", "zh-CN": "自动切换输入法" },
        description: {
          en: "Switch input methods automatically when you return to keyboard navigation in VL.",
          "zh-CN": "在 VL 回到键盘导航时自动切换输入法。",
        },
      },
      {
        key: "yazi",
        slug: "yazi",
        file: "YAZI.md",
        title: { en: "Yazi integration", "zh-CN": "Yazi 集成" },
        description: {
          en: "Set up VL to preview code and open text files from Yazi.",
          "zh-CN": "在 Yazi 中用 VL 预览代码并打开文本文件。",
        },
      },
    ],
  },
  {
    label: { en: "Reference", "zh-CN": "参考" },
    pages: [
      {
        key: "cli",
        slug: "cli",
        file: "CLI.md",
        title: { en: "Command line", "zh-CN": "命令行" },
        description: {
          en: "Open workspaces and files, set startup options, and print syntax-highlighted previews with VL.",
          "zh-CN": "用 VL 打开工作区和文件、设置启动选项，以及输出带语法高亮的预览。",
        },
      },
      {
        key: "shortcuts",
        slug: "shortcuts",
        file: "KEYBINDINGS.md",
        title: { en: "Keyboard shortcuts", "zh-CN": "键盘快捷键" },
        description: {
          en: "Keyboard shortcuts for the VL workbench, including navigation, search, and Git diffs.",
          "zh-CN": "VL 工作台的键盘快捷键，包括导航、搜索和 Git 差异。",
        },
      },
      {
        key: "vim",
        slug: "vim",
        file: "VIM.md",
        title: { en: "Vim keys in VL", "zh-CN": "Vim 键位" },
        description: {
          en: "Vim motions, editing commands, and the ways VL differs from Vim.",
          "zh-CN": "Vim 光标移动、编辑命令和 VL 与 Vim 的键位差异。",
        },
      },
    ],
  },
] as const

const allPages = docGroups.flatMap(({ pages }) => [...pages])
type DocKey = (typeof allPages)[number]["key"]

function localizeDocs(locale: Locale) {
  const sourceDirectory = locale === "zh-CN" ? "docs/zh.CN" : "docs"
  return Object.fromEntries(allPages.map(({ key, slug, file, title, description }) => [
    key,
    {
      slug,
      path: localizedPath(`/docs/${slug}`, locale),
      source: `${sourceDirectory}/${file}`,
      title: title[locale],
      description: description[locale],
    },
  ])) as Record<DocKey, Document>
}

export const docs = { en: localizeDocs("en"), "zh-CN": localizeDocs("zh-CN") }

export const sitePaths = localeCodes.flatMap((locale) => [
  localizedPath("/", locale),
  ...Object.values(docs[locale]).map(({ path }) => path),
])

export function findDoc(locale: Locale, slug: string) {
  return Object.values(docs[locale]).find((doc) => doc.slug === slug)
}

export type RenderedDocument = {
  html: string
  headings: { id: string; title: string; depth: number }[]
  searchSections: SearchSection[]
}

export type SearchSection = [fragment: string, heading: string, text: string]
export type SearchDocument = [path: string, title: string, sections: SearchSection[]]
