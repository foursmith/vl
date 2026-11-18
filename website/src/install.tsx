import { createSignal, onSettled } from "solid-js"

const platforms = [
  { id: "macos", name: "macOS" },
  { id: "linux", name: "Linux" },
  { id: "windows", name: "Windows" },
] as const

type Platform = (typeof platforms)[number]["id"]

const releaseAssets = "https://github.com/foursmith/vl/releases/latest/download"
const brewInstall = "brew install foursmith/tap/vl"
const shellInstall = `curl -fsSL ${releaseAssets}/install.sh | sh`
const powershellInstall = `Invoke-RestMethod -UseBasicParsing ${releaseAssets}/install.ps1 | Invoke-Expression`

function Command(props: { text: string; label: string }) {
  const [status, setStatus] = createSignal("")

  async function copy() {
    try {
      await navigator.clipboard.writeText(props.text)
      setStatus("Copied")
    } catch {
      setStatus("Select the command to copy it manually.")
    }
  }

  return (
    <div class="mt-2.5">
      <div class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-sm border border-line bg-inset p-2">
        <pre class="min-w-0 px-2 py-1.5 text-sm leading-relaxed wrap-anywhere whitespace-pre-wrap"><code>{props.text}</code></pre>
        <button
          type="button"
          class="grid size-8 cursor-pointer place-items-center rounded-sm text-muted hover:bg-line hover:text-accent focus-visible:text-accent"
          aria-label={`Copy ${props.label}`}
          title={`Copy ${props.label}`}
          onClick={copy}
        >
          <span class="copy-icon" aria-hidden="true" />
        </button>
      </div>
      <p class="text-sm text-accent" role="status">{status()}</p>
    </div>
  )
}

function ShellInstaller() {
  return (
    <>
      <p class="mt-1 leading-relaxed text-muted">
        Run the release installer in your terminal. It detects your architecture and installs <code>vl</code> to <code>~/.local/bin</code>.
      </p>
      <Command label="shell install command" text={shellInstall} />
      <p class="mt-4 leading-relaxed text-muted">
        If <code>vl</code> is missing from your <code>PATH</code>, run this in Bash or Zsh. Add the same line to <code>~/.bashrc</code> or <code>~/.zshrc</code> to keep it for new sessions.
      </p>
      <Command label="PATH command" text={'export PATH="$HOME/.local/bin:$PATH"'} />
    </>
  )
}

export function InstallSection() {
  const [platform, setPlatform] = createSignal<Platform>("macos")
  const tabs: HTMLButtonElement[] = []

  onSettled(() => {
    const agent = navigator.userAgent
    if (/Windows NT/i.test(agent)) setPlatform("windows")
    else if (/Linux/i.test(agent) && !/Android/i.test(agent)) setPlatform("linux")
  })

  function navigateTabs(event: KeyboardEvent, index: number) {
    let next: number
    switch (event.key) {
      case "ArrowRight": next = (index + 1) % platforms.length; break
      case "ArrowLeft": next = (index + platforms.length - 1) % platforms.length; break
      case "Home": next = 0; break
      case "End": next = platforms.length - 1; break
      default: return
    }
    event.preventDefault()
    setPlatform(platforms[next].id)
    tabs[next].focus()
  }

  return (
    <section id="install" class="landing-section" aria-labelledby="install-title">
      <h2 id="install-title" class="section-title">Install vl</h2>
      <p class="mt-2 text-muted">Choose your platform. Release binaries are available for x86-64 and ARM64.</p>

      <div class="mt-6 overflow-hidden rounded-md border border-line bg-panel">
        <div class="flex border-b border-line bg-inset font-mono text-xs" role="tablist" aria-label="Installation platform">
          {platforms.map(({ id, name }, index) => (
            <button
              ref={(element) => { tabs[index] = element }}
              id={`install-tab-${id}`}
              type="button"
              role="tab"
              aria-selected={platform() === id ? "true" : "false"}
              aria-controls={`install-panel-${id}`}
              tabindex={platform() === id ? 0 : -1}
              class="-mb-px cursor-pointer border-e border-b border-line border-b-transparent px-4 py-3 text-muted hover:text-ink focus-visible:-outline-offset-4 aria-selected:border-b-accent aria-selected:bg-panel aria-selected:text-ink sm:px-6"
              onClick={() => setPlatform(id)}
              onKeyDown={(event) => navigateTabs(event, index)}
            >{name}</button>
          ))}
        </div>

        <div class="p-4 sm:p-6">
          <div id="install-panel-macos" role="tabpanel" aria-labelledby="install-tab-macos" hidden={platform() !== "macos"}>
            <div class="grid gap-7 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
              <div class="min-w-0">
                <h3 class="text-lg font-semibold">Homebrew</h3>
                <p class="mt-1 leading-relaxed text-muted">If you have Homebrew, run this command. It adds the Foursmith tap and installs vl.</p>
                <Command label="Homebrew command" text={brewInstall} />
              </div>
              <div class="min-w-0 lg:border-s lg:border-line lg:ps-7">
                <h3 class="text-lg font-semibold">Install script</h3>
                <ShellInstaller />
              </div>
            </div>
          </div>

          <div id="install-panel-linux" role="tabpanel" aria-labelledby="install-tab-linux" hidden={platform() !== "linux"}>
            <h3 class="text-lg font-semibold">Install script</h3>
            <ShellInstaller />
          </div>

          <div id="install-panel-windows" role="tabpanel" aria-labelledby="install-tab-windows" hidden={platform() !== "windows"}>
            <h3 class="text-lg font-semibold">PowerShell install script</h3>
            <p class="mt-1 leading-relaxed text-muted">
              Open PowerShell and run the command below. The installer places <code>vl.exe</code> in <code class="wrap-anywhere">%LOCALAPPDATA%\vl\bin</code> and updates your user <code>PATH</code>.
            </p>
            <Command label="PowerShell install command" text={powershellInstall} />
            <p class="mt-4 leading-relaxed text-muted">You can run vl in the same PowerShell session. Reopen other terminal windows to pick up the updated <code>PATH</code>.</p>
          </div>

          <div class="mt-6 border-t border-line pt-5">
            <h3 class="text-lg font-semibold">Open a workspace</h3>
            <p class="mt-1 leading-relaxed text-muted">With Git on your <code>PATH</code>, open a repository in your terminal. Check the installation, then launch vl in the current directory.</p>
            <Command label="launch commands" text={"vl --version\nvl ."} />
          </div>
        </div>
      </div>
    </section>
  )
}
