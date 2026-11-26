# Quickstart

[Install VL](INSTALL.md). To display file icons, install a [Nerd Font](https://github.com/ryanoasis/nerd-fonts) and select it in your terminal’s font settings. See [optional dependencies](INSTALL.md#optional-dependencies) for other features.

Run this from a project directory.

```sh
vl .
```

Use `vl --icons=false .` for plain-text icons. See the [command-line reference](CLI.md) for launch options, file positions, and preview output.

**Text files open in Vim Normal mode.** Press `i` to type and `Esc` to return to Normal mode. To turn off Vim keybindings, press `Ctrl+P` and run **Preferences: Toggle Vim Mode**.

1. Select a file in the tree to preview it. Press Enter to keep it open in its own tab.
2. Press `Ctrl+G` to find another file by name or path.
3. In a Git repository with changes, press `Ctrl+3` and select a changed file to see its diff.

Press `Ctrl+P` to find an action or `Ctrl+Q` to quit.

## Browse and search code

- Open Explorer with `Ctrl+1`. Move with `j` and `k` to preview files, or use Outline to jump between Markdown headings.
- Press `Ctrl+G` and type a filename or path. Add `:line[:column]` to jump to a position, such as `main.go:42:8`. In the picker, press `F4` to toggle ignored files.
- Press `Ctrl+F` to search the current file or `Ctrl+Shift+F` to search the workspace. Workspace search includes unsaved edits. Open **Search Details** for filters, or [install ripgrep](https://github.com/BurntSushi/ripgrep) to speed up workspace searches. See [find and replace](KEYBINDINGS.md#find-and-replace) for result navigation and replacement.

## Review Git changes

1. From a Git repository, press `Ctrl+3` and select a file in **Changes**. Press `F2` to focus its diff.
2. Press `[` or `]` to jump between hunks. To switch layouts, press `F7` and choose inline or side-by-side from the diff menu.
3. Press `s` to stage a hunk from an unstaged diff or unstage it from a staged diff. In Vim mode, press `V` first to select specific lines.

Press `Ctrl+4` to browse commits. To review GitHub pull requests, [install GitHub CLI](https://cli.github.com/), run `gh auth login`, then press `Ctrl+5` in a repository with a GitHub remote. See [diff editor shortcuts](KEYBINDINGS.md#diff-editor) for more actions.
