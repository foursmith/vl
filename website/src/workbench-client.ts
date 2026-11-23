import { screenshots, screenshotSrcset, themes } from "./workbench-images"

export function enableWorkbench(section: HTMLElement) {
  const image = section.querySelector<HTMLImageElement>("figure img")!
  const details = section.querySelector("[data-workbench-details]")!
  const views = [...section.querySelectorAll<HTMLButtonElement>("[data-workbench-view]")]
  const themeButtons = [...section.querySelectorAll<HTMLButtonElement>("[data-workbench-theme]")]
  const templates = [...section.querySelectorAll<HTMLTemplateElement>("[data-workbench-view-details]")]
  let view = 0
  let theme: (typeof themes)[number] = "Dark"

  function updateImage() {
    const source = screenshots[view][theme]
    image.srcset = screenshotSrcset(source)
    image.src = source.src
  }

  views.forEach((button, index) => button.addEventListener("click", () => {
    if (index === view) return
    views[view].setAttribute("aria-pressed", "false")
    button.setAttribute("aria-pressed", "true")
    view = index
    const template = templates[view]
    image.alt = template.dataset.imageAlt!
    details.replaceChildren(template.content.cloneNode(true))
    updateImage()
  }))

  themeButtons.forEach((button, index) => button.addEventListener("click", () => {
    const nextTheme = themes[index]
    if (nextTheme === theme) return
    themeButtons[themes.indexOf(theme)].setAttribute("aria-pressed", "false")
    button.setAttribute("aria-pressed", "true")
    theme = nextTheme
    updateImage()
  }))
}
