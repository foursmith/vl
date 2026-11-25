```
░▒▒▒▒░       ▒▒▒▒▒   ▒▒▒▒▒
░▒▓▓▓▓      ▓▓▓▓▓▓   ▒▓▓▓▓
░░▒▓▓▓▓    ▒▓▓▓▓▓▓   ▒▓▓▓▓
░░░▒▓▓▓▒  ░▓▓▓▓▓▓░   ▒▓▓▓▓
 ░░░▒▓▓▓░░▓▓▓▓▓▓░    ▓▓▓▓▓
  ░░░▓▓▓▓▓▓▓▓▓▓▒     ▓▓▓▓▓░░░░░░░░░
   ░░░▓▓▓▓▓▓▓▓▒      ▓▓▓▓▓▓▓▓▓▓▓▓▓▒
    ░░▒▒▒▒▒▒▓▒       ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
     ░▒▒▒▒▒▒▓        ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
```

<p align="center">
  English · <a href="README.zh-CN.md">简体中文</a>
</p>

A **V**SCode-**l**ike terminal workbench with **V**im-**l**ike keybindings.
Browse code, review diffs, and make quick edits.

![](./assets/workbench.webp)

Agents write all my code now. I still review their diffs before they commit.
VS Code with the Vim extension has been my go-to for years, so I built vl around
the bits I actually use. One ~10 MB binary, ready to run.

- **Omitted VS Code features**
  - Language server integration
    - Code completion
    - Diagnostics
    - Go to Definition
  - Debugging
  - Integrated terminal
  - Build and test tasks
  - Extension runtime and marketplace
- **Omitted Vim features**
  - Visual block mode
  - Macros
  - Named registers
  - Marks and mark jumps
  - Ex commands
  - Vimscript configuration and plugins


## Install

### Homebrew

```sh
brew tap foursmith/tap
brew install foursmith/tap/vl
```

### Go

Requires Go 1.27.3 or newer.

```sh
go install github.com/foursmith/vl/cmd/vl@latest
```

### macOS and Linux

```sh
curl -fsSL https://github.com/foursmith/vl/releases/latest/download/install.sh | sh
```

Installs to `~/.local/bin`.

### Windows (PowerShell)

```powershell
Invoke-RestMethod -UseBasicParsing https://github.com/foursmith/vl/releases/latest/download/install.ps1 | Invoke-Expression
```

Installs to `%LOCALAPPDATA%\vl\bin` and adds it to your user `PATH`.

### Build from source

Requires Go 1.27.3 or newer.

```sh
git clone https://github.com/foursmith/vl.git
cd vl
make build
```

Run `./bin/vl` from the checkout.

## Usage

### Optional dependencies

- For repository features, install Git. [Git](https://git-scm.com/)
- For faster workspace search, install ripgrep (`rg`). [ripgrep](https://github.com/BurntSushi/ripgrep)
- For GitHub pull requests, install `gh` and run `gh auth login`. [GitHub CLI](https://cli.github.com/)
- For remote images in Markdown, install `curl`.
- For SVG previews, install `resvg`. [resvg](https://github.com/linebender/resvg)
- For file icons, use a Nerd Font in your terminal. [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts)

### Modes

- `vl .` opens the workbench. Use `Ctrl+G` to find a file and `Ctrl+Shift+F` to search across files.
- `vl path/to/file` opens a single file in the editor.
- `vl preview path/to/file` prints syntax-highlighted code.

Click the language in the status bar or choose **Change Syntax Highlighting**
from the command palette. The choice is saved per file and also applies to
`vl preview`. **Auto Detect** uses the filename or shebang. **Plain Text**
disables highlighting.

See the [Keyboard shortcuts](docs/KEYBINDINGS.md).

### Auto Switch IM

Press `Ctrl+,` or choose **Preferences: Open Configuration File** from the
command palette. The configuration accepts JSONC comments and trailing commas.
Add this field, save with `Ctrl+S`, and run **Reload VL** from the command palette.

```json
{
  "features": {
    "autoSwitchInputMethod": {
      "enable": true,
      "cmd": ["im-select", "com.apple.keylayout.ABC"]
    }
  }
}
```

vl runs the command when navigation resumes after using a text field, leaving Insert mode, or returning to the terminal. It also runs at startup if navigation has focus. Set `enable` to `false` to disable switching.

The example selects ABC on macOS using [im-select](https://github.com/daipeihust/im-select). Use the executable and arguments for your input method setup, such as `["im-select.exe", "1033"]` on Windows or `["fcitx5-remote", "-c"]` with Fcitx 5 on Linux. The executable must be in `PATH` or given by its absolute path.

### Yazi integration

#### Preview code

Requires Yazi 26.9.1 or newer. Install the preview plugin.

```sh
ya pkg add foursmith/vl:vl-preview
```

Enable code previews in `yazi.toml`.

```toml
[[plugin.prepend_previewers]]
mime = "text/*"
run = "vl-preview"

[[plugin.prepend_previewers]]
mime = "application/{mbox,javascript,wine-extension-ini}"
run = "vl-preview"
```

#### Open text files

Add this to `yazi.toml` to open text files in vl with Enter or `o`.

```toml
[[opener.vl]]
run = "vl -- %s1"
block = true
desc = "Edit in vl"

[[open.prepend_rules]]
mime = "text/*"
use = ["vl", "reveal"]

[[open.prepend_rules]]
mime = "application/{json,ndjson,mbox,javascript,wine-extension-ini}"
use = ["vl", "reveal"]
```

## Development

```sh
make build  # updates bin/vl
make test
make check  # go vet + race-enabled tests
```

## Acknowledgments

- [TTT](https://github.com/eugenioenko/ttt) and [textmate-go](https://github.com/eugenioenko/textmate-go) for syntax highlighting and implementation inspiration.
- [VS Code](https://github.com/microsoft/vscode) for the workbench design and [Vim](https://github.com/vim/vim) for the keybindings.

## License

This Source Code Form is subject to the terms of the Mozilla Public License,
v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain
one at https://mozilla.org/MPL/2.0/.

See [LICENSE](LICENSE) for the full text. Third-party components retain their
respective licenses; see [Third-party notices](THIRD_PARTY_NOTICES.md).
