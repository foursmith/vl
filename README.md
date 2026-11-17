# elegit

A terminal Git workbench with diff review and basic text editing. It brings
Explorer, Changes, History and editor tabs into one workspace, with VS Code
Dark Modern / Light Modern colors and Material Icon Theme associations adapted
to Nerd Font glyphs.

Advanced Git capabilities are still in progress; see the [roadmap](docs/ROADMAP.md).

## Run

Requires Go **1.27.3+**, Git on `PATH`, and an interactive UTF-8 terminal.
A Nerd Font is recommended; use `-icons=false` for plain-text controls and markers.
Git uses your existing credentials or SSH agent; terminal password prompts are disabled.

```sh
make build
bin/elegit /path/to/folder

# Startup overrides; themes and language can also be changed through F1.
bin/elegit -theme light_modern
bin/elegit -theme local:my-theme.json /path/to/repository
bin/elegit -lang zh-CN /path/to/repository
bin/elegit -icons=false /path/to/repository
```

With no path, elegit opens the most recent workspace, or asks for a path if none
exists. Use `bin/elegit .` to open the current directory. Paths inside a Git
repository resolve to its root. Ordinary folders support browsing and editing;
Changes and History offer **Initialize Repository**.

## Features

- **Git:** staged, unstaged, untracked and conflicted status; file and hunk staging;
  commits; branch/tag selection; fetch, fast-forward pull, push, stash and discard.
- **Diff review:** unified and side-by-side views, syntax colors, hunk navigation,
  source-text selection and image comparisons. Commit diffs are read-only.
- **History:** paged commits with branch/tag labels and expandable changed-file trees.
  Merge commits compare against their first parent.
- **Multiple repositories:** Changes and History have independent sections for
  parent repositories and initialized submodules. Actions target the owning
  repository; committing a submodule does not commit its parent's pointer.
- **Explorer and tabs:** compact folder trees, file search, preview/kept tabs,
  context menus, recent workspaces and per-folder session restoration.
- **Text editing:** grapheme-aware movement, selection, system clipboard, undo/redo,
  literal search, line jumps, syntax highlighting and save. Optional Vim mode adds
  Visual selection, Flash jumps, Surround and repeat commands.
- **Images:** read-only PNG, JPEG, GIF (first frame) and SVG previews, with zoom and pan.
- **Workspace UI:** sticky code/folder context, resizable left/right sidebar,
  output panel, English/简体中文, and built-in or local themes.

File and Git metadata changes refresh automatically. Clean files reload;
unsaved edits are retained and external modifications are reported.

## Using the workspace

Open files with **Ctrl+P**, commands with **F1**, and recent folders with **Ctrl+R**.
The file/command picker accepts `>` for commands, `:line[:column]` for a position
in the current file, or `filename:line[:column]` to open a file at that position.

A single click opens a reusable preview tab. Double-click or Enter keeps it open;
editing also keeps a text tab. Tabs and text cursor/scroll positions are restored
per folder. Unsaved edits require confirmation before closing, quitting or
switching workspaces.

The status bar's branch picker and remote actions follow the active file's
repository, including submodules. Sync pulls with `--ff-only` before pushing;
a branch without an upstream can be published to a selected remote.
Ahead/behind counts use locally fetched tracking refs.

Refreshing never stages, commits, fetches or changes branches. Discard and deletion
require confirmation. Discarding an untracked file removes it from disk; Git
cannot restore it.

## Keyboard shortcuts

- [普通快捷键](docs/KEYBINDINGS.md): workspace, sidebar, editor, Diff, images and pickers.
- [Vim 快捷键](docs/VIM.md): modes, operators, text objects, Visual, Flash and Surround.

Enable Vim through **F1 → Preferences: Toggle Vim Mode** (search `vim`). It is off
by default and applies only to editable text tabs. Workbench shortcuts remain available.

## Appearance and preferences

Use **F1 → Preferences: Color Theme** to preview and select a theme, or
**Preferences: Configure Display Language** to switch between English and 简体中文.
Theme, language, Vim and layout preferences persist across restarts and workspace switches.

Dark/light terminal notifications select the remembered matching theme; macOS
system appearance is the fallback. A manually selected mismatched theme stays selected.
Local VS Code JSON/JSONC and TextMate themes belong in `~/.config/elegit/themes/`;
see [theme compatibility](docs/THEME.md).

Preferences and recent folders use `layout.json` and `recent.json` in the OS user
configuration directory's `elegit` folder. Sessions use `~/.config/elegit/workspaces/`
on all platforms. `-theme` and `-lang` override saved choices at startup.

## Requirements and limits

- Text files must be UTF-8 and at most 4 MiB. Mixed LF/CRLF, legacy CR-only files
  and other binary files are rejected. Saves preserve BOM, uniform line endings,
  final newline presence and permissions, and check for external changes.
- TextMate syntax highlighting applies to recognized languages up to 256 KiB.
  Sticky code context uses lexical braces/indentation rather than a language server.
- Image rendering requires the [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
  with Unicode placeholders and a compatible terminal with 256-color support.
  Unsupported terminals show an explanation. Image files are limited to 32 MiB
  and 16 megapixels. SVG external image references are skipped.
- Below 70 columns, sidebar and content share one surface; use Enter and Esc/F6
  to switch. Narrow diffs fall back to unified layout; added/deleted files always
  use full-width unified diffs.
- `NO_COLOR` enables non-color focus indicators. Nerd Font icons are terminal
  adaptations, not the original SVG artwork.
- Save As, LSP, multi-cursor, conflict resolution and line staging are not yet
  available. See the [roadmap](docs/ROADMAP.md) for remaining work.

## Development

```sh
make build  # updates bin/elegit
make test
make check  # go vet + race-enabled tests
```

Tests cover temporary Git repositories, editor operations and model/view behavior.
Interactive terminal acceptance is separate. Restart running instances after rebuilding.

- [Architecture](docs/ARCHITECTURE.md)
- [Performance and benchmarks](docs/PERFORMANCE.md)
- [Theme compatibility](docs/THEME.md)
- [Roadmap](docs/ROADMAP.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)
