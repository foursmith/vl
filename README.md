# vl

A lightweight, VSCode-like Git workbench and Vim-like editor.

It brings Explorer, Changes, History, PR and editor tabs into one workspace, with VS Code
Dark Modern / Light Modern colors and Material Icon Theme associations adapted
to Nerd Font glyphs.

Advanced Git capabilities are still in progress; see the [roadmap](docs/ROADMAP.md).

## Run

Requires Go **1.27.3+** and an interactive UTF-8 terminal.
Folder workspaces also require Git on `PATH`.
A Nerd Font is recommended; use `-icons=false` for plain-text controls and markers.
Git uses your existing credentials or SSH agent; terminal password prompts are disabled.

```sh
make build
bin/vl /path/to/folder
bin/vl /path/to/file.go  # editor-only mode
bin/vl /path/to/file.go:30:5  # jump to line 30, column 5 (column is optional)

# Startup overrides; themes and language can also be changed through Ctrl+Shift+P.
bin/vl -theme light_modern
bin/vl -theme "vscode:antfu.theme-vitesse/Vitesse Dark" /path/to/repository
bin/vl -lang zh-CN /path/to/repository
bin/vl -icons=false /path/to/repository
```

With no path, vl opens the most recent workspace, or asks for a path if none
exists. Use `bin/vl .` to open the current directory. Paths inside a Git
repository resolve to its root. Ordinary folders support browsing and editing;
Changes and History offer **Initialize Repository**.

Passing an existing text file opens only the editor, without tabs, breadcrumbs, the sidebar, Git
controls, repository discovery or workspace session restoration. Git is not
required for this mode; Git history, commit state and commands are not initialized.
**Ctrl+S** saves;
**Ctrl+W** closes the file and exits (with a save/discard/cancel prompt for edits).
**Ctrl+Q** quits. Search, clipboard, undo/redo, themes and Vim editing
remain available. Direct file editing does not change recent workspaces or their
saved tabs.

File locations use one-based line and character column numbers; positions beyond
the file are clamped to its end or the line's end. Existing paths containing
colons take precedence over location suffixes.

### Code preview

```sh
vl preview cmd/vl/main.go
vl preview --color=always --skip=20 --lines=30 --width=80 -- cmd/vl/main.go
vl preview --line-numbers --wrap --width=80 cmd/vl/main.go
```

`vl preview` writes syntax-highlighted source to the terminal using the
editor's TextMate grammars and saved theme. Redirected output is plain text by
default. Use `--color=always` for consumers of ANSI output and `--theme` to select
a syntax theme for one invocation. `--line-numbers` shows the original source
line numbers. `--wrap` wraps long lines instead of clipping them and requires
`--width`, which includes the line-number gutter. `--skip` and `--lines` count
source lines before wrapping. Preview reads incrementally, caps long source
lines at 32 KiB, and continues in plain text after the highlighting budget
is exhausted.

For Yazi 26.9.1 or newer, copy [the previewer](contrib/yazi/vl-code.yazi) into
`~/.config/yazi/plugins/`, put `vl` on your `PATH`, and change `run = "code"`
to `run = "vl-code"` in the code entries under `[plugin]` → `previewers` in
`yazi.toml`. The plugin passes the pane dimensions and scroll offset to
`vl preview` for local files. Yazi's built-in code preview handles virtual
URLs and clamps scroll offsets beyond the end of a file.

## Features

- **Git:** staged, unstaged, untracked and conflicted status; file and hunk staging;
  commits; branch/tag selection; fetch, fast-forward pull, push, stash and discard.
- **Diff review:** unified and side-by-side views, syntax colors, hunk navigation,
  source-text selection and image comparisons. Commit diffs are read-only.
- **PR:** the 50 most recent pull requests for the workspace repository,
  including open, draft, merged and closed PRs, with per-file read-only diffs.
  Requires GitHub CLI (`gh`) authenticated with `gh auth login`.
- **History:** paged commits with branch/tag labels and expandable changed-file trees.
  Merge commits compare against their first parent.
- **Multiple repositories:** Changes and History have independent sections for
  parent repositories and initialized submodules. Actions target the owning
  repository; committing a submodule does not commit its parent's pointer.
- **Explorer and tabs:** compact folder trees, file search, preview/kept tabs,
  context menus, recent workspaces and per-folder session restoration.
  Right-click a compact folder chain and choose **Split Folders** to show each
  directory separately for the current workspace session, including across
  refreshes. Closing or switching workspaces resets the split state.
- **Workspace search:** second sidebar tab, grouped file results with line numbers and match highlights;
  case, whole-word, regex and ignored-file options, including unsaved open text buffers.
  Use Replace below the search field or Ctrl+Shift+H for selected-match/file or workspace replacement,
  with regex capture groups and stale-result checks. Workspace replacements save affected files directly
  without opening tabs, including any pending edits in those files. Replacements in open files are undoable.
  Full-width search and replacement inputs keep the sidebar focused on results.
  The Options picker provides preserve-case replacement, include/exclude globs, changed-file and
  open-editor scopes, and ignore files. The same editor find/replace component handles Ctrl+F, Vim / and n, context menus and
  workspace-result handoff, including match counts and per-file replacement.
  Scope filters compose and stay active after closing the picker; see [search controls](docs/KEYBINDINGS.md#全文搜索与替换).
- **Text editing:** grapheme-aware movement, selection, system clipboard, undo/redo,
  shared find/replace with match counts and next/previous navigation, line jumps, syntax highlighting and save. Vim is the only interaction
  mode, with Normal/Insert/Visual editing, Flash jumps, Surround and repeat commands.
- **Markdown:** visually aligned tables and inline local or HTTP(S) images. The
  Markdown source stays editable. Syntax highlighting refreshes on returning to
  Normal mode after typing in Insert mode.
- **Images:** read-only PNG, JPEG, GIF (first frame), WebP and SVG previews, with zoom and pan.
- **Workspace UI:** sticky code/folder context, resizable left/right sidebar,
  output panel, English/简体中文, and built-in and VS Code color themes.

File and Git metadata changes refresh automatically. Clean files reload;
unsaved edits are retained and external modifications are reported.

## Using the workspace

Search workspace text with **Ctrl+2** or **Ctrl+Shift+F**.
Sidebar shortcuts **Ctrl+1/2/3/4/5** open Explorer, Search, Changes, History and PR.
Explorer is the default. Each workspace remembers its selected sidebar tab.
The badge below PR shows the number of open pull requests, including drafts.
The PR list shows open pull requests by default. Use the filter and refresh
buttons in the repository row to change the state filter or reload the list.
Scroll to the bottom to load the next page, or press **Enter** on **Load more pull requests…**.
Loaded pages and the open count are saved locally per filter and shown while
GitHub refreshes the loaded range. The count and list refresh once per minute.
Hover over or focus a commit or PR to read its full title in the status bar.
In History and PR, press **l** or **Enter** on a commit or PR to toggle its
file tree. Moving onto a file previews its diff and keeps focus in the sidebar.
Clicking a commit or PR toggles its tree, and clicking a file previews its diff.
Each workspace keeps its 32 most recently used PR diffs in `vl/pr-patches` under
the local cache directory. Expanding a PR reuses its cached diff if the source
and target revisions match. Press **F5** in the PR tab to refresh the list,
count and current diff. Press **o** on a PR or file to open the PR on GitHub.

Open files with **Ctrl+P**, commands with **Ctrl+Shift+P**, and recent workspaces with **Ctrl+H**.
The file/command picker accepts `>` for commands, `:line[:column]` for a position
in the current file, or `filename:line[:column]` to open a file at that position.

A single click in a sidebar list shows a reusable preview and leaves focus in the list.
Moving focus into the editor keeps the displayed tab open. You can do this with
Enter, F6, a double-click in the list, or a click in the editor or its tab.
Scrolling the content leaves the preview reusable. The active tab has a thin accent marker; file tabs
share Explorer's Git colors and status letters. An unsaved dot takes precedence
over the Git status letter. Tabs and text cursor/scroll positions are restored
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

- [快捷键总览](docs/KEYBINDINGS.md): workspace, sidebar, editor, Diff, images and pickers.
- [文本编辑详解](docs/VIM.md): Normal/Insert/Visual, operators, text objects, Flash and Surround.

vl uses one Vim keybinding set throughout. Text tabs open in Normal; press `i` to
insert and `Esc` to return to Normal. In content tabs, `Space 1`–`Space 9`
switch tabs and `Space q` closes the current tab. Diff is a read-only editor: it shares
Vim motions, Visual selection, yanking, search and Flash while blocking text changes.
Diff hunk actions live in the context menu and command palette; `[`/`]` navigate hunks.
Workbench shortcuts remain available.

## Appearance and preferences

Use **Ctrl+Shift+P → Preferences: Color Theme** to preview and select a theme, or
**Preferences: Configure Display Language** to switch between English and 简体中文.
Use **Preferences: Status Bar Style** to choose **Standard** (the default,
colors the whole bar by Vim mode) or **Pure** (colors the mode label only
and uses the sidebar colors for the rest of the bar).
Theme, language and layout preferences persist across restarts and workspace switches.

The 10 color themes from VS Code's `vscode.theme-defaults` are bundled. Dark/light
terminal notifications select the remembered matching theme at startup and when the
terminal appearance changes. macOS system appearance is the fallback. A manually
selected mismatched theme stays selected.

Choose **Import Themes from VS Code** at the top of the color theme picker
to add color themes from VS Code. The picker skips themes that are already
bundled or imported. See [theme compatibility](docs/THEME.md).

Preferences use `~/.config/vl/vl.json`.
Runtime state uses `$XDG_STATE_HOME/vl/`, defaulting to `~/.local/state/vl/` when
the variable is unset, empty or relative. It contains: `recent.json` for recent
folders, `workspaces/` for tab sessions, and `files/` for cursor and scroll positions.
Each file has its own `files/<SHA-256 of the real absolute file path>.json` record.
Code folds are saved with each file's position and restored when their first and last lines still match at the saved line numbers.
File-tree opens, direct file invocations and different workspaces share the same record, including
symlink aliases. Changed positions are saved when switching tabs, replacing previews,
closing files and exiting vl; opening a file without moving its cursor, scrolling or changing folds
does not create or rewrite its position record. Workspace sessions only save the tab list and active tab.
State is read only from the configured state directory.
`-theme` and `-lang` override saved choices at startup.

## Requirements and limits

- Text files must be UTF-8 and at most 4 MiB. Mixed LF/CRLF, legacy CR-only files
  and other binary files are rejected. Saves preserve BOM, uniform line endings,
  final newline presence and permissions, and check for external changes.
- TextMate syntax highlighting applies to recognized languages up to 256 KiB.
  Sticky code context uses lexical braces/indentation rather than a language server.
- Image rendering requires the [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
  with Unicode placeholders and a compatible terminal with 256-color support.
  Standalone previews explain when terminal graphics are unavailable. Image files are limited to 32 MiB
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
make build  # updates bin/vl
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

## License

This Source Code Form is subject to the terms of the Mozilla Public License,
v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain
one at https://mozilla.org/MPL/2.0/.

See [LICENSE](LICENSE) for the full text. Third-party components retain their
respective licenses; see [Third-party notices](THIRD_PARTY_NOTICES.md).
