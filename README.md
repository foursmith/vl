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

Requires Go **1.26.7+**, Git on `PATH`, and an interactive terminal. A Nerd Font
is recommended. Git authentication uses your existing credentials/SSH agent;
terminal password prompts are disabled.

```sh
make build
bin/elegit /path/to/repository

# Override automatic theme detection (also available via F1 → Preferences: Color Theme):
bin/elegit -theme light /path/to/repository

# Plain-text icons when the terminal font lacks Nerd Font glyphs:
bin/elegit -icons=false /path/to/repository
```

With no path, elegit opens the repository containing the current directory.
Subdirectories and file paths resolve to their enclosing repository.

## Available now

- Centered block-letter welcome screen with clickable command, file-search, and Source
  Control shortcuts; compact branding on small terminals.
- Open elegit's recent repositories with `Ctrl+R`, the welcome page, or
  **File: Open Recent Folder** in the command palette. Successful opens move to
  the front of the searchable list, which keeps the last 20 repositories in
  `recent.json` beside `layout.json` in the elegit configuration directory.
  Failed opens preserve the current workspace; unsaved editor changes and
  active Git operations block switching.
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
- Unified and side-by-side text diffs, line numbers, addition/deletion colors,
  hunk navigation and mouse selection, current-view search.
- Compact diff controls show worktree/index context, nonzero change counts,
  and a clickable layout toggle. File-mode and rename details use readable
  context labels; hunk ranges use subdued separators instead of raw patch headers.
- Stage/unstage a file, stage all, and stage/unstage individual modification
  hunks. Added/deleted/renamed/copied files are staged as a whole.
- Commit staged changes and inspect the latest 100 commits per repository. History shows branch
  and tag labels and supports multiple expanded commits, each with its own
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
  counts, conflicts, cursor position, tab width, UTF-8, line ending and language.
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
- Single-child folder chains display as one compact path in Explorer, Source
  Control, and commit history; branching folders and folders containing files
  retain their own rows.
- Explorer folders start collapsed. Ignored paths remain visible in gray; files
  show Git status letters and folders show aggregate change dots. New file,
  new folder, refresh, and collapse-all buttons work from the repository header.
  `Ctrl+P` opens the searchable file picker.
- Search sidebar tab (`Ctrl+Shift+F` / `Ctrl+4`) with live workspace text search,
  collapsible file groups, highlighted matches, and click/Enter to reveal a match.
  File rows show a Material icon, filename, muted parent path, and a count badge.
  Matches on the same source line share a compact snippet with every visible
  match highlighted. The active editor also highlights all matches in the file,
  preserving the current selection. Row actions keep a fixed slot so hover and
  selection never reflow snippets or count badges. Hover or select a row to
  remove it from the current results with the close button or `Delete`; removal
  leaves file contents unchanged and
  excludes those results from replacement until the next search.
  Search includes tracked and untracked files across initialized submodules and
  nested repositories, respects each repository's Git ignore rules, skips symlinks
  and binary files, and uses open editor buffers including unsaved changes.
  Toggle **Include Ignored Files** (`[ig]` / `Alt+I`) to also search ignored files;
  Git metadata is always excluded.
- Expand Replace with `Ctrl+Shift+H` or the chevron. Match Case (`Alt+C`),
  Whole Word (`Alt+W`), and Regular Expression (`Alt+R`) work for search and
  replacement. Regex uses Go syntax (RE2); replacements support `$1` and `${name}`.
  Replace a selected match or file with `Ctrl+Enter`; replace all results with
  `Ctrl+Alt+Enter`, or use the corresponding buttons. Changes open as unsaved
  editor tabs with one undo step per file; save each edited file with `Ctrl+S`.
  Search scans the full workspace without a cumulative byte or match-count cutoff,
  including text files larger than the editor's 4 MiB editing limit. Such files
  appear in results but retain that limit when opened or replaced. Unreadable files
  are reported and block bulk replacement. Changed targets trigger a new search
  before replacement so an outdated result cannot overwrite edits.
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
- Automatically follow terminal color scheme notifications (DEC 2031), with immediate
  updates while running. Terminals without this protocol keep the dark default.
  Choose Auto, Light Modern or Dark Modern from **F1 → Preferences: Color Theme**.
  The choice is saved on exit in `layout.json` and retained when opening another repository.
  `-theme auto` / `-theme light` / `-theme dark` overrides the saved choice at startup.
- TextMate syntax highlighting with VS Code Dark Modern / Light Modern token rules, including font styles, for recognized languages up to 256 KiB.
- Drag the sidebar divider to resize it. Use **View: Move Side Bar Right/Left**
  in the command palette (**F1**) to change its position, or **View: Reset Side Bar Width**
  to restore automatic sizing. Width and position are saved on exit in
  `elegit/layout.json` under the OS user configuration directory. Narrow terminals
  keep the single-surface sidebar/content layout.
- Changes uses the same collapsible repository section for ordinary
  repositories, parents and initialized submodules, including clean repositories. Each section shows
  its branch, its own staged/worktree file trees, and refresh,
  commit and stage buttons. Child file paths are relative to that repository.
- Commit/History uses the same repository sections. Each repository has its own
  latest 100 commits, branch/tag labels and independently expandable file trees.
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
- Searchable commands, disabled-command explanations, and Git output panel.

Search files with **Ctrl+P**. Commands are available from **Ctrl+Shift+P** or **F1**.
Clean tabs close immediately; unsaved files offer **Save / Discard / Cancel**, using either the
buttons or **S / D / Esc**. File mutations and Git operations run only when requested; refreshing
the view does not stage, commit, fetch, or change branches.

## Keys

| Context | Key | Action |
|---|---|---|
| Global | Ctrl+1 / 2 / 3 | Explorer / Changes / History |
| Global | Ctrl+P | Search files |
| Global | Ctrl+Shift+P or F1 | Commands |
| Global | F6 / Esc | Switch focus / focus sidebar |
| Global | Ctrl+B / Ctrl+J | Toggle sidebar / output |
| Global | Ctrl+D | Toggle unified / side-by-side diff |
| Global | F5 | Refresh repository |
| Global | Ctrl+Shift+F / Ctrl+4 | Search workspace sidebar |
| Global | Ctrl+Shift+H | Expand workspace replacement |
| Search | Tab / Shift+Tab | Move between search, replace, and results |
| Search | Alt+C / Alt+W / Alt+R / Alt+I | Toggle case / whole word / regex / ignored files |
| Search | Ctrl+Enter / Ctrl+Alt+Enter | Replace selected match or file / all results |
| Global | Ctrl+F / F3 | Find / next match in current content |
| Global | Ctrl+G | Go to line or line:column |
| Global | Ctrl+S / Ctrl+W | Save / close editor tab |
| Global | Alt+Left / Alt+Right | Previous / next editor tab |
| Global | Ctrl+Q | Quit, with unsaved-changes confirmation |
| Sidebar | Up/Down or K/J | Select entry |
| Sidebar | Enter or Right | Open file/diff, toggle folder, or switch branch |
| Changes | E | Edit selected file |
| Changes | Space | Toggle file staging |
| Changes | S / U / A / C | Stage / unstage / stage all / commit |
| File / Diff | Horizontal wheel or Shift+wheel | Scroll horizontally |
| Diff | Left / Right | Scroll horizontally |
| Diff | [ / ] or P / N | Previous / next hunk |
| Diff | S / U | Stage / unstage selected hunk |
| Diff | E / Tab | Edit file / toggle diff layout |
| Editor | Ctrl+Z / Ctrl+Y | Undo / redo |
| Editor | Shift+arrows | Select text |
| Editor | Ctrl+A / C / X / V | Select all / copy / cut / paste |
| Editor | Tab | Insert a literal tab |

Printable keys remain text input while an editor has focus. Below 70 columns,
the sidebar and content share one surface; use Enter and Esc/F6 to switch.
Side-by-side diffs fall back to unified when the content is too narrow.

## Current limits

The editor opens regular UTF-8 text files up to 4 MiB. Binary, mixed LF/CRLF,
and legacy CR-only files are rejected rather than silently converted. It does
not currently offer Save As, LSP, multi-cursor, or session restoration.
File and diff tabs share one opening order and the same switch/close controls.
Single-clicking files or diffs reuses one preview tab; double-clicking the row or
tab, pressing Enter, or running Keep Preview Open keeps it open. Kept working-tree,
staged and commit diffs retain independent tabs, including for the same file.
Reopening the same source selects its existing tab.
File tabs open live worktree files; commit diffs are read-only patches.

Material Icon Theme uses SVG artwork. This terminal version uses a curated set
of its associations/colors with Nerd Font equivalents, not the original SVG
shapes or its entire association catalog. `-icons=false` keeps the colors and
uses plain text markers. No GUI/WebView or terminal image protocol is required.

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
