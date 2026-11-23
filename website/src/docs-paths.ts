import { localizedPath, type Locale } from "./locales.ts"

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
  path: string
  source: string
  section?: string
  title: string
  description: string
}

export const docs = {
  en: {
    install: {
      path: "/docs/install",
      source: "README.md",
      section: "Install",
      title: "Install",
      description: "Install vl with Homebrew, Go, a release script, or a source build.",
    },
    usage: {
      path: "/docs/",
      source: "README.md",
      section: "Usage",
      title: "Usage",
      description: "Set up the vl workbench and use vl from the shell or from Yazi.",
    },
    shortcuts: {
      path: "/docs/shortcuts",
      source: "docs/KEYBINDINGS.md",
      title: "Keyboard shortcuts",
      description: "Keyboard shortcuts for the vl workbench, including navigation, search, and Git diffs.",
    },
    vim: {
      path: "/docs/vim",
      source: "docs/VIM.md",
      title: "Vim keys in vl",
      description: "Vim motions, editing commands, and the ways vl differs from Vim.",
    },
  },
  "zh-CN": {
    install: {
      path: "/zh-CN/docs/install",
      source: "README.zh-CN.md",
      section: "安装",
      title: "安装",
      description: "通过 Homebrew、Go、发布脚本或从源码构建来安装 vl。",
    },
    usage: {
      path: "/zh-CN/docs/",
      source: "README.zh-CN.md",
      section: "用法",
      title: "用法",
      description: "配置 vl 工作台，并从终端或 Yazi 中使用 vl。",
    },
    shortcuts: {
      path: "/zh-CN/docs/shortcuts",
      source: "docs/KEYBINDINGS.zh-CN.md",
      title: "键盘快捷键",
      description: "vl 工作台的键盘快捷键，包括导航、搜索和 Git 差异。",
    },
    vim: {
      path: "/zh-CN/docs/vim",
      source: "docs/VIM.zh-CN.md",
      title: "vl 的 Vim 键位",
      description: "Vim 光标移动、编辑命令和 vl 与 Vim 的键位差异。",
    },
  },
} satisfies Record<Locale, Record<"install" | "usage" | "shortcuts" | "vim", Document>>

export type RenderedDocument = {
  html: string
  headings: { id: string; title: string; depth: number }[]
}
