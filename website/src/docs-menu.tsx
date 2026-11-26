import { render } from "@solidjs/web"
import { Menu, MenuContent, MenuItem, MenuTrigger } from "@ui-sdk/solid/menu"
import { For } from "solid-js"
import { docGroups, docs } from "./docs-paths"
import { ChevronDown } from "./header-parts"
import { locales, type Locale } from "./locales"

const menuPages = docGroups.flatMap(({ pages }) =>
  pages.filter(({ key }) => key !== "install" && key !== "usage" && key !== "shortcuts"),
)

function DocsMenu(props: { locale: Locale }) {
  const pages = docs[props.locale]

  return (
    <Menu>
      <MenuTrigger class="flex cursor-pointer items-center gap-1 hover:text-accent data-open:text-ink">
        {locales[props.locale].docs}
        <ChevronDown />
      </MenuTrigger>
      <MenuContent
        class="z-20 max-h-[min(60dvh,var(--available-height))] w-56 overflow-y-auto border border-line bg-canvas p-1 text-sm font-medium whitespace-normal text-muted shadow-lg"
        align="start"
        sideOffset={10}
        collisionPadding={12}
      >
        <For each={menuPages}>{({ key }) => {
          const { path, title } = pages[key]
          return (
            <MenuItem
              class="block px-3 py-2 wrap-anywhere data-highlighted:bg-workspace data-highlighted:text-ink"
              value={key}
              render={(attributes) => <a {...attributes} href={path} />}
            >
              {title}
            </MenuItem>
          )
        }}</For>
      </MenuContent>
    </Menu>
  )
}

export function mountDocsMenu(container: HTMLElement, locale: Locale) {
  container.replaceChildren()
  render(() => <DocsMenu locale={locale} />, container)
  return container.querySelector<HTMLButtonElement>('[data-slot="menu-trigger"]')!
}
