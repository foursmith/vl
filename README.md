<div align="center">
<pre>
░▒▒▒▒░       ▒▒▒▒▒   ▒▒▒▒▒&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
░▒▓▓▓▓      ▓▓▓▓▓▓   ▒▓▓▓▓&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
░░▒▓▓▓▓    ▒▓▓▓▓▓▓   ▒▓▓▓▓&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
░░░▒▓▓▓▒  ░▓▓▓▓▓▓░   ▒▓▓▓▓&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
 ░░░▒▓▓▓░░▓▓▓▓▓▓░    ▓▓▓▓▓&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
  ░░░▓▓▓▓▓▓▓▓▓▓▒     ▓▓▓▓▓░░░░░░░░░
   ░░░▓▓▓▓▓▓▓▓▒      ▓▓▓▓▓▓▓▓▓▓▓▓▓▒
    ░░▒▒▒▒▒▒▓▒       ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
     ░▒▒▒▒▒▒▓        ▒▒▒▒▒▒▒▒▒▒▒▒▒▒
</pre>
<p>
  English · <a href="README.zh-CN.md">简体中文</a>
</p>
<p><strong>Read code, review Git diffs, and make quick edits right in your terminal, just like in VS Code.</strong></p>
</div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./website/src/assets/browse-dark.webp">
  <source media="(prefers-color-scheme: light)" srcset="./website/src/assets/browse-light.webp">
  <img src="./website/src/assets/browse-light.webp" alt="VL browsing a Markdown file with the file tree and outline open">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./website/src/assets/diff-dark.webp">
  <source media="(prefers-color-scheme: light)" srcset="./website/src/assets/diff-light.webp">
  <img src="./website/src/assets/diff-light.webp" alt="VL showing staged Git changes in a side-by-side diff">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./website/src/assets/edit-dark.webp">
  <source media="(prefers-color-scheme: light)" srcset="./website/src/assets/edit-light.webp">
  <img src="./website/src/assets/edit-light.webp" alt="VL editing a shell script in single-file mode">
</picture>

## Install

### Homebrew

```sh
brew install foursmith/tap/vl
```

On macOS, install the optional tools and a Nerd Font with this command.

```sh
brew install git ripgrep gh resvg font-jetbrains-mono-nerd-font
```

Select **JetBrainsMono Nerd Font Mono** in your terminal’s font settings to display file icons. For GitHub pull requests, run `gh auth login`. The built-in macOS `curl` can load remote images in Markdown.

### Go

Requires Go 1.27.3 or newer.

```sh
go install github.com/foursmith/vl/cmd/vl@latest
```

### macOS and Linux

```sh
curl -fsSL https://github.com/foursmith/vl/releases/latest/download/install.sh | sh
```

Installs to `~/.local/bin`. If `vl` is not found, add that directory to your `PATH`.

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

See the [optional dependencies](https://vl.foursmith.com/docs/install#optional-dependencies) for the tools each feature uses.

## Quickstart

Run `vl .` in your project directory to open the workbench.

Select a Nerd Font in your terminal to display file icons, or run `vl --icons=false .` to use plain-text icons.

Text files open in **Vim Normal mode** by default. Press `i` to type and `Esc` to return to Normal mode. To turn off Vim keybindings, press `Ctrl+P` and run **Preferences: Toggle Vim Mode**.

- `Ctrl+G` finds a file by name or path.
- `Ctrl+Shift+F` searches file contents across the workspace.
- `Ctrl+3` opens the Changes view in a Git repository.

## Documentation

- [Quickstart](https://vl.foursmith.com/docs/) for browsing code and reviewing Git changes.
- [Command line](https://vl.foursmith.com/docs/cli) for startup options, file positions, and `vl preview`.
- [Keyboard shortcuts](https://vl.foursmith.com/docs/shortcuts) for default bindings and customization.
- [Configuration](https://vl.foursmith.com/docs/configuration) for themes, input methods, and integrations.

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

vl is licensed under the [MIT License](LICENSE). See
[Third-party notices](THIRD_PARTY_NOTICES.md) for component licenses.
