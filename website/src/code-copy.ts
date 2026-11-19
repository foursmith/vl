import { onCleanup } from "solid-js"

export function createCodeCopyHandler() {
  const resets = new Map<HTMLButtonElement, ReturnType<typeof setTimeout>>()

  onCleanup(() => {
    for (const reset of resets.values()) clearTimeout(reset)
  })

  return async function copyCode(event: MouseEvent) {
    if (!(event.target instanceof Element)) return
    const button = event.target.closest<HTMLButtonElement>("button[data-copy-code]")
    const block = button?.closest(".code-block")
    const code = block?.querySelector("pre code")
    const status = block?.querySelector('[role="status"]')
    if (!button || !code || !status) return

    let state: "copied" | "failed"
    try {
      await navigator.clipboard.writeText(code.textContent.replace(/\r?\n$/, ""))
      state = "copied"
    } catch {
      state = "failed"
    }
    if (!button.isConnected) return

    const feedback = state === "copied" ? "Copied" : "Select the text to copy it manually."
    button.dataset.copyState = state
    button.title = feedback
    status.textContent = feedback

    clearTimeout(resets.get(button))
    resets.set(button, setTimeout(() => {
      delete button.dataset.copyState
      button.title = button.getAttribute("aria-label") ?? ""
      status.textContent = ""
      resets.delete(button)
    }, 2000))
  }
}
