export const siteUrl = "https://vl.foursmith.com/"
export const repository = "https://github.com/foursmith/vl"

export const docs = {
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
} as const

export type RenderedDocument = {
  html: string
  headings: { id: string; title: string; depth: number }[]
}
