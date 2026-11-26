import type { JSX } from "@solidjs/web"

export function SearchIcon() {
  return (
    <svg class="size-[1.125rem] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" aria-hidden="true">
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  )
}

export function SearchTriggerContent(props: { label: string; shortcut?: JSX.Element }) {
  return (
    <>
      <SearchIcon />
      <span class="hidden flex-1 text-start md:inline">{props.label}</span>
      <kbd class="hidden md:inline" aria-hidden="true">{props.shortcut ?? "Ctrl+K"}</kbd>
    </>
  )
}

export function ChevronDown() {
  return (
    <svg class="size-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="m3 4.5 3 3 3-3" />
    </svg>
  )
}
