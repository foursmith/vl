# Install

Choose an installation method for your system.

## Homebrew

```sh
brew install foursmith/tap/vl
```

On macOS, install the optional tools and a Nerd Font with this command.

```sh
brew install git ripgrep gh resvg font-jetbrains-mono-nerd-font
```

Select **JetBrainsMono Nerd Font Mono** in your terminal’s font settings to display file icons. For GitHub pull requests, run `gh auth login`. The built-in macOS `curl` can load remote images in Markdown.

## Go

Requires Go 1.27.3 or newer.

```sh
go install github.com/foursmith/vl/cmd/vl@latest
```

## macOS and Linux

```sh
curl -fsSL https://github.com/foursmith/vl/releases/latest/download/install.sh | sh
```

Installs to `~/.local/bin`. If `vl` is not found, add that directory to your `PATH`.

## Windows (PowerShell)

```powershell
Invoke-RestMethod -UseBasicParsing https://github.com/foursmith/vl/releases/latest/download/install.ps1 | Invoke-Expression
```

Installs to `%LOCALAPPDATA%\vl\bin` and adds it to your user `PATH`.

## Build from source

Requires Go 1.27.3 or newer.

```sh
git clone https://github.com/foursmith/vl.git
cd vl
make build
```

Run `./bin/vl` from the checkout.

## Optional dependencies

- Install a [Nerd Font](https://github.com/ryanoasis/nerd-fonts) and select it in your terminal settings to display file icons.
- Install [Git](https://git-scm.com/) to review diffs and browse commit history.
- Install [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for faster workspace searches.
- Install the [GitHub CLI](https://cli.github.com/) and run `gh auth login` to review GitHub pull requests.
- Install [resvg](https://github.com/linebender/resvg) to preview SVG files and `curl` to load remote images in Markdown.
