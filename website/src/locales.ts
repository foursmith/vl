export type Locale = "en" | "zh-CN"

export const locales = {
  en: {
    home: "vl home",
    workbench: "Workbench",
    docs: "Docs",
    navigation: "Main navigation",
    documentation: "Documentation",
    onThisPage: "On this page",
    referenceTable: "Reference table",
    copyCode: "Copy code",
    copied: "Copied",
    copyFailed: "Select the text to copy it manually.",
    siteTitle: "VL — VS Code-like Terminal Workbench with Vim-like Keybindings",
    description: "A VS Code-like terminal workbench with Vim-like keybindings. Browse code, review diffs, and make quick edits.",
    imageAlt: "Blue 3D VL wordmark above the workbench shortcuts and Install link.",
  },
  "zh-CN": {
    home: "vl 首页",
    workbench: "工作台",
    docs: "文档",
    navigation: "主导航",
    documentation: "文档导航",
    onThisPage: "本页目录",
    referenceTable: "参考表格",
    copyCode: "复制代码",
    copied: "已复制",
    copyFailed: "请选中文字并手动复制。",
    siteTitle: "VL — 类 VS Code 的终端工作台，支持 Vim 风格键位",
    description: "在终端中浏览代码、审阅差异和快速编辑，使用类似 VS Code 的工作台和 Vim 风格键位。",
    imageAlt: "蓝色立体 VL 标志，下方是工作台快捷键和安装链接。",
  },
} satisfies Record<Locale, Record<string, string>>

export function localeForPath(path: string): Locale {
  return path.split("/")[1] === "zh-CN" ? "zh-CN" : "en"
}

export function localizedPath(path: string, locale: Locale) {
  const base = path.replace(/^\/zh-CN(?=\/|$)/, "").replace(/\/$/, "")
  const canonical = base === "" || base === "/docs" ? `${base}/` : base
  return locale === "zh-CN" ? `/zh-CN${canonical}` : canonical
}
