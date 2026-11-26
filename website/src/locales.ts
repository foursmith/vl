export const localeCodes = ["en", "zh-CN"] as const
export type Locale = (typeof localeCodes)[number]

export const locales = {
  en: {
    home: "VL home",
    docs: "Docs",
    navigation: "Main navigation",
    theme: "Color theme",
    dark: "Dark",
    light: "Light",
    documentation: "Documentation",
    allDocs: "All documentation",
    searchDocs: "Search docs",
    closeSearch: "Close search",
    noSearchResults: "No matching documents.",
    searchUnavailable: "Search is unavailable. Try again.",
    onThisPage: "On this page",
    referenceTable: "Reference table",
    copyCode: "Copy code",
    copied: "Copied",
    copyFailed: "Select the text to copy it manually.",
    siteTitle: "VL — VS Code-like terminal workbench",
    description: "A VS Code-like terminal workbench with Vim-like keybindings. Browse code, review diffs, and make quick edits.",
    imageAlt: "Blue 3D VL wordmark above the workbench shortcuts and Install link.",
  },
  "zh-CN": {
    home: "VL 首页",
    docs: "文档",
    navigation: "主导航",
    theme: "配色主题",
    dark: "深色",
    light: "浅色",
    documentation: "文档导航",
    allDocs: "所有文档",
    searchDocs: "搜索文档",
    closeSearch: "关闭搜索",
    noSearchResults: "没有找到相关文档。",
    searchUnavailable: "暂时无法搜索，请重试。",
    onThisPage: "本页目录",
    referenceTable: "参考表格",
    copyCode: "复制代码",
    copied: "已复制",
    copyFailed: "请选中文字并手动复制。",
    siteTitle: "VL — VS Code 风格的终端工作台",
    description: "在终端中浏览代码、审阅差异和快速编辑，使用类似 VS Code 的工作台和 Vim 风格键位。",
    imageAlt: "蓝色立体 VL 标志，下方是工作台快捷键和安装链接。",
  },
} satisfies Record<Locale, Record<string, string>>

export function localizedPath(path: string, locale: Locale) {
  const base = path.replace(/^\/zh-CN(?=\/|$)/, "").replace(/\/$/, "")
  const canonical = base === "" || base === "/docs" ? `${base}/` : base
  return locale === "zh-CN" ? `/zh-CN${canonical}` : canonical
}
