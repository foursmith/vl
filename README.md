# elegit

A terminal Git workbench with diff review and basic text editing. The target is a
complete Git and diff workspace that can replace the Git workflow in VS Code.
The editor stays deliberately smaller than a language IDE.

The workbench aligns sidebar navigation and editor tabs in a compact single-row
header, followed by content controls, an optional output panel, and a status bar.
Colors follow **VS Code Dark Modern / Light Modern**, automatically selected
from terminal color scheme notifications by default. File associations and colors follow
**Material Icon Theme**, adapted to Nerd Font glyphs for terminal display.
Interface controls use Codicons from Nerd Fonts with fixed cell widths;
`-icons=false` uses ASCII controls and file/folder markers.

This is the first working foundation. Advanced Git parity is tracked explicitly
in [docs/ROADMAP.md](docs/ROADMAP.md); it is not yet a complete Git client.

## Run

Requires Go **1.27.3+**, Git on `PATH`, and an interactive terminal. A Nerd Font
is recommended. Git authentication uses your existing credentials/SSH agent;
terminal password prompts are disabled.

```sh
make build
bin/elegit /path/to/folder

# Select a theme (also available via F1 → Preferences: Color Theme):
bin/elegit -theme light_modern
# Use another builtin or a theme from ~/.config/elegit/themes/:
bin/elegit -theme dark_plus
bin/elegit -theme local:my-theme.json /path/to/repository

# Select the interface language (also available via F1 → Preferences: Configure Display Language):
bin/elegit -lang zh-CN /path/to/repository
bin/elegit -lang en /path/to/repository

# Plain-text icons when the terminal font lacks Nerd Font glyphs:
bin/elegit -icons=false /path/to/repository
```

With no path, elegit opens the most recently used workspace. If there is no
recent workspace, it asks you to provide a path. Use `elegit .` to explicitly open
the current directory. Subdirectories and file paths inside a repository resolve
to its root. Ordinary folders support file browsing, quick open, and
editing. Changes and History offer **Initialize Repository** when the folder has
no Git repository; initialization keeps open editor tabs and unsaved edits.

English is the default display language. Use **F1 → Preferences: Configure Display Language**
(search `language` in either language) to switch between **English** and **简体中文**
without restarting. The selection is saved immediately as `language` in `layout.json`
and retained across workspace switches. `-lang en` or `-lang zh-CN` overrides the
saved choice at startup. File contents, names, commit messages, and raw Git output
keep their original text. English strings are the fallback for missing translations;
the Simplified Chinese catalog lives in `internal/i18n/zh-CN.json`.

## Available now

- Centered block-letter welcome screen with clickable command, file-search, and
  Changes shortcuts; compact branding on small terminals.
- Open elegit's recent folders with `Ctrl+R`, the welcome page, or
  **File: Open Recent Folder** in the command palette. Successful opens move to
  the front of the searchable list, which keeps the last 20 folders in
  `recent.json` beside `layout.json` in the elegit configuration directory.
  The same folders appear above the sidebar navigation tabs for direct switching.
  Click the WORKSPACES header to collapse or expand the list; the state is saved
  in `layout.json` and restored on restart.
  Names stay compact, duplicate names include a parent directory, and hovering
  shows the full path. Longer lists scroll independently of the file tree.
  Failed opens preserve the current workspace; unsaved editor changes and
  active Git operations block switching.
- Open tabs are saved per folder when quitting or switching folders and restored
  on the next open, including file/image tabs, worktree/index/commit diffs, tab
  order, active tab, preview state, and text cursor/scroll positions. Sessions
  live in `~/.config/elegit/workspaces/` on all platforms. Missing files are
  skipped; unsaved edits still require the existing discard confirmation.
- Staged, unstaged, untracked, and conflicted file status; automatic refresh.
- Changes groups use folder trees with independent folding for staged
  and worktree changes. Group headers also collapse independently and retain
  their file counts and batch actions while collapsed. Mouse-wheel scrolling
  moves the sidebar viewport without selecting files or requesting diffs. Click or press Enter to
  toggle a group or folder; Left collapses
  it or selects its parent, and Right expands it. File status and staging actions
  retain their worktree/index identities across refreshes.
- Hover a changed file for inline open, discard and stage buttons. Index rows
  offer open and unstage. Hovering never changes selection; disabled actions
  explain their reason in the status bar. Folder rows offer recursive stage,
  unstage and confirmed discard within that folder. Group headers offer the same
  batch actions and open-all; collapsed descendants are included.
- Sticky scroll keeps enclosing code block headers above the editor and parent
  folders above Explorer/Changes trees (up to five rows, leaving room for content).
  Code context follows braces in common Go/C/JS-style files and indentation in
  Python; it is a lightweight lexical outline, not a language-server symbol tree.
  Pinned code keeps line numbers and syntax colors; click it to return to its
  source. Pinned folders retain folding, hover, and context-menu actions.
- Unified and side-by-side text diffs, line numbers, addition/deletion colors,
  hunk navigation and mouse selection, current-view search.
- Compact diff controls show worktree/index context, nonzero change counts,
  and a clickable layout toggle. File-mode and rename details use readable
  context labels; hunk ranges use subdued separators instead of raw patch headers.
- Stage/unstage a file, stage all, and stage/unstage individual modification
  hunks. Added/deleted/renamed/copied files are staged as a whole.
- Commit staged changes and browse repository history, loading older commits
  automatically as you scroll. History shows branch
  labels and highlights lightweight and annotated tags before commit titles.
  It supports multiple expanded commits, each with its own
  collapsible changed-file tree. Select a file for a read-only parent-to-commit
  diff (first parent for merges). Initial commits compare against an empty tree,
  and renames keep their original path.
- List local branches, switch branches, create and switch to a new branch.
- Click the status-bar branch to search local/remote branches and tags, create
  a branch from HEAD or a selected reference, or check out detached. Remote
  checkout creates a tracking branch; existing local names are never replaced.
- Status-bar Git indicators and actions follow the active file or diff, including
  nested submodules; with no active file they retain the most recent file's
  repository, falling back to the workspace root before any file is opened.
- Status bar shows staged/unstaged/untracked markers, upstream ahead/behind
  counts, conflicts, cursor position, UTF-8, line ending and language.
  Click sync to pull with `--ff-only` then push; without an upstream, choose a
  remote to publish the current branch. Counts use locally fetched tracking refs.
- Click the GitHub status icon to open the repository in the system browser.
  SSH and HTTPS GitHub remotes are normalized to a credential-free HTTPS URL;
  the active branch's upstream remote is preferred, followed by `origin`.
- Fetch, fast-forward pull, push, stash with untracked files, pop latest stash,
  and confirmed discard of non-conflicted worktree changes. Discarding an
  untracked file requires confirmation and deletes only that file; Git cannot
  restore it. Directories are never recursively deleted by this action.
- Expandable repository file tree and text-file tabs with VS Code-style preview
  behavior: a single click reuses one clean preview, double-click/Enter keeps a
  file open, and editing permanently keeps its tab even after undo.
- Read-only PNG, JPEG, GIF (first frame), and SVG previews in file tabs.
  Image diffs show the old and new versions side by side for working-tree,
  staged, and commit changes, with dimensions and absent sides labeled.
  Image diffs fit each version to its half of the view; stage or unstage the
  whole file from Changes.
  Previews use the [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
  with Unicode placeholders, requiring a compatible terminal with 256-color support.
  The terminal is probed on startup; unsupported terminals show an explanation.
  Images fit the available space, retain transparency, follow the same
  preview/keep/close controls, and reload on external changes. Closing a preview
  or exiting releases its terminal image resources.
  Scroll over an image to zoom around the pointer; drag with the left mouse
  button to pan. With the image focused, **+ / = / -** zoom around the viewport
  center, **arrow keys** pan, and **0** restores fit. Each tab retains its zoom and
  position. The toolbar/status bar show the actual pixel scale. Zoom ranges
  from 0.1× to 32× the initial fit; only the visible region is rasterized.
  SVG renders in pure Go, including text, gradients, clipping, masks and filters,
  and rasterizes again at the display resolution when resized. External image
  references are skipped; embedded images and installed system fonts are supported.
  Files are limited to 32 MiB and 16 megapixels, with transfers bounded to a
  4096-pixel edge.
- Single-child folder chains display as one compact path in Explorer, Changes,
  and commit history; branching folders and folders containing files
  retain their own rows.
- Explorer folders start collapsed. Ignored paths remain visible in gray; files
  show Git status letters and folders show aggregate change dots. New file,
  new folder, refresh, and collapse-all actions are available in the context menu.
  `Ctrl+P` opens the searchable file picker.
- Visible tab close buttons, middle-click close, close others/all, and overflow
  arrows that keep the active tab and its close button accessible.
- Parent-directory file monitoring refreshes visible explorer directories and
  open files, including atomic replacement. Clean buffers reload without moving
  the viewport; dirty buffers retain edits and report external changes. Git
  metadata watches resolve linked worktrees. Events are merged for 150 ms with
  a 1-second maximum wait. A 30-second reconciliation covers unwatched paths and
  missed notifications; watcher creation failure retains the 3-second polling fallback.
- UTF-8 editing, Tab insertion, grapheme-aware cursor movement/deletion, selection, clipboard
  copy/cut/paste, undo/redo, find, and save. Existing BOM, uniform CRLF/LF, final
  newline presence, and file permissions survive saves. External modifications
  are checked before replacing a file.
- Remember separate dark and light themes. When the current theme matches the
  terminal appearance, its next light/dark change selects the remembered counterpart.
  First reports after startup preserve the saved theme.
  A mismatched manual choice stays selected. Terminal notifications (DEC 2031)
  take precedence; macOS system appearance is the fallback.
  Open **F1 → Preferences: Color Theme** for the searchable picker: arrows preview,
  Enter confirms, Esc cancels. Put VS Code JSON/JSONC or TextMate themes in
  `~/.config/elegit/themes/`; see [theme compatibility](docs/THEME.md).
  Confirmed themes are saved immediately in `layout.json`, even without terminal
  appearance notifications. Previews are not saved. Other layout preferences are
  saved on exit; all preferences are retained across repository switches.
  `-theme light_modern` / `-theme dark_modern` or any builtin/local ID selects a theme at startup.
- TextMate syntax highlighting with VS Code Dark Modern / Light Modern token rules, including font styles, for recognized languages up to 256 KiB.
- Drag the sidebar divider to resize it. Use **View: Move Sidebar Right/Left**
  in the command palette (**F1**) to change its position, or **View: Reset Sidebar Width**
  to restore automatic sizing. Width and position are saved on exit in
  `elegit/layout.json` under the OS user configuration directory. Narrow terminals
  keep the single-surface sidebar/content layout.
- Changes uses the same collapsible repository section for ordinary
  repositories, parents and initialized submodules, including clean repositories. Each section shows
  its branch, its own staged/worktree file trees, and refresh,
  commit and stage buttons. Child file paths are relative to that repository.
- Commit/History uses the same repository sections. Each repository has its own
  paged commit history, branch/tag labels and independently expandable file trees.
  Historical file diffs read from that repository, including commits absent from
  the parent's object database. Repository folds are independent between the
  Changes and History pages. Click a repository header's branch name to open
  the existing branch/tag picker for that repository, including create-from and
  detached checkout. The target is retained throughout the picker flow.
- Commit messages are kept independently per repository until a successful commit.
  Click the commit button or press C to edit a draft. The commit prompt names
  the repository and branch; only that repository's staged files are committed.
  A failed commit keeps its draft.
- C and A in Changes commit or stage all in the selected repository.
  File/folder/group actions, diffs, hunk staging, unstaging and discard also stay
  within their owning repository. Parent submodule summaries remain visible with
  an S marker; internal child edits are staged and committed in the child section.
  Committing a child never stages or commits its parent's new pointer.
- Repository discovery and refresh are read-only. Explorer retains its normal
  directory interactions; status-bar branch and remote controls use the workspace root.
- Right-click context menus for Explorer, Changes, History, editor/diff/image content,
  and editor tabs. Menus retain text selections and target the clicked file or tab.
  Use Up/Down, Enter and Esc, or click outside to dismiss; Shift+F10 opens the
  menu for the focused sidebar row or editor. Long menus scroll in small terminals.
  Existing discard and unsaved-file confirmations also apply to menu actions.
- Searchable commands, disabled-command explanations, and Git output panel.

Search files with **Ctrl+P**. Commands are available from **Ctrl+Shift+P** or **F1**.
Both use the same input: a leading `>` searches commands; remove it to search files.
All pickers support Up/Down, Page Up/Down and Home/End; typing searches directly.
Lists scroll one row at a time and highlight hovered rows without changing the
keyboard selection. Command availability hints appear in the status bar.
Append `:line[:column]` to a filename to jump to that position, or enter
`:line[:column]` alone to jump within the active text file.
Commands sort alphabetically by name, with the last used command first; the choice
is remembered across restarts and repository switches.

Clean tabs close immediately; unsaved files offer **Save / Don't Save / Cancel**, using either the
buttons or **S / D / Esc**. File mutations and Git operations run only when requested; refreshing
the view does not stage, commit, fetch, or change branches.
Confirmation dialogs wrap long paths; when the message exceeds the screen, use
**Up/Down**, **Page Up/Down**, or **Home/End** to read it. The actions stay visible.
Use **Left/Right** or **Tab/Shift+Tab** to select a button, **Enter/Space** to activate it,
and **Esc** to cancel. Empty diff tabs close automatically when their changes disappear.
With `NO_COLOR` set to a nonempty value, lists mark the focused row with `>`;
the active navigation section and editor tab use underlining.

## Keys

| Context | Key | Action |
|---|---|---|
| Global | Ctrl+1 / 2 / 3 | Explorer / Changes / History |
| Global | Ctrl+P | Search files |
| Global | Ctrl+Shift+P or F1 | Commands |
| Global | F6 / Esc | Switch focus / focus sidebar |
| Global | Ctrl+L / Ctrl+J | Toggle sidebar / output |
| Global | Ctrl+D | Toggle unified / side-by-side diff |
| Global | F5 | Refresh repository |
| Sidebar / Content | Right-click / Shift+F10 | Open context menu |
| Global | Ctrl+F / F3 | Find / next match in current content |
| Global | Ctrl+G | Go to line or line:column |
| Global | Ctrl+S / Ctrl+W | Save / close editor tab |
| Global | Alt+Left / Alt+Right | Previous / next editor tab |
| Global | Ctrl+Q | Quit, with unsaved-changes confirmation |
| Sidebar | Up/Down or K/J | Select entry |
| Sidebar | Enter or Right | Open file/diff, toggle folder, or switch branch |
| Explorer | Del | Delete selected file or folder from disk, with confirmation |
| Changes | E | Edit selected file |
| Changes | Space | Toggle file staging |
| Changes | S / U / A / C | Stage / unstage / stage all / commit |
| File / Diff | Horizontal wheel or Shift+wheel | Scroll horizontally |
| Diff | Left / Right | Scroll horizontally |
| Diff | [ / ] or P / N | Previous / next hunk |
| Diff | S / U | Stage / unstage selected hunk |
| Diff | E / Tab | Edit file / toggle diff layout |
| Image | Wheel up / down | Zoom in / out around the pointer |
| Image | + / = / - | Zoom in / out around the viewport center |
| Image | Arrow keys | Pan the image |
| Image | Left drag / 0 | Pan / restore fit |
| Editor | Ctrl+Z / Ctrl+Y | Undo / redo |
| Editor | Shift+arrows | Select text |
| Editor | Ctrl+A / C / X / V | Select all / copy / cut / paste |
| Editor / Diff | Left drag / Shift+click, Ctrl+C | Select and copy source text; split diffs select one side |
| Editor | Tab | Insert a literal tab |

Printable keys remain text input while an editor has focus. Below 70 columns,
the sidebar and content share one surface; use Enter and Esc/F6 to switch.
Added and deleted files always use a full-width unified text diff.
Side-by-side diffs fall back to unified when the content is too narrow.

## Current limits

The editor opens regular UTF-8 text files up to 4 MiB; supported raster images and
SVGs open in a read-only image tab. Other binary, mixed LF/CRLF,
and legacy CR-only files are rejected rather than silently converted. It does
not currently offer Save As, LSP, or multi-cursor.
File and diff tabs share one opening order and the same switch/close controls.
Single-clicking files or diffs reuses one preview tab; double-clicking the row or
tab, pressing Enter, or running **View: Keep Editor** keeps it open. Kept working-tree,
staged and commit diffs retain independent tabs, including for the same file.
Reopening the same source selects its existing tab.
File tabs open live worktree files; commit diffs are read-only patches.

Material Icon Theme uses SVG artwork. This terminal version uses a curated set
of its associations/colors with Nerd Font equivalents, not the original SVG
shapes or its entire association catalog. `-icons=false` keeps the colors and
uses plain text markers. No GUI/WebView is required. Raster and SVG file previews
require the Kitty graphics protocol with Unicode placeholder support.

All file diffs share the same unified/side-by-side renderer and Git's default
three lines of context. File previews and diffs support horizontal scrolling
with a trackpad/horizontal wheel or Shift+mouse wheel. Focused diffs also support
Left/Right arrows; side-by-side panes scroll together with fixed line numbers.
Full-file diff context, conflict resolution, line staging, and the other
remaining capabilities are in the roadmap.

## Development

```sh
make build
make test
make check  # go vet + race-enabled tests
go test ./internal/app -run '^$' -bench 'Benchmark(ScrollFrame|ExplorerDecorations)$' -benchmem
```

Tests use temporary repositories for real Git operations, plus editor and diff
unit tests and deterministic model/view tests. Interactive terminal acceptance is
separate from these checks. Performance measurements, reproduction commands and
remaining cost boundaries are in [docs/PERFORMANCE.md](docs/PERFORMANCE.md).

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
