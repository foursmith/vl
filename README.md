# elegit

A terminal Git workbench with diff review and basic text editing. The target is a
complete Git and diff workspace that can replace the Git workflow in VS Code.
The editor stays deliberately smaller than a language IDE.

The workbench aligns sidebar navigation and editor tabs in a compact single-row
header, followed by content controls, an optional output panel, and a status bar.
Colors follow **VS Code Modern Dark**. File associations and colors follow
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

# Plain-text icons when the terminal font lacks Nerd Font glyphs:
bin/elegit -icons=false /path/to/repository
```

With no path, elegit opens the repository containing the current directory.
Subdirectories and file paths resolve to their enclosing repository.

## Available now

- Staged, unstaged, untracked, and conflicted file status; automatic refresh.
- Source Control groups use folder trees with independent folding for staged
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
- Commit staged changes, inspect the latest 100 commits and their patches.
- List local branches, switch branches, create and switch to a new branch.
- Click the status-bar branch to search local/remote branches and tags, create
  a branch from HEAD or a selected reference, or check out detached. Remote
  checkout creates a tracking branch; existing local names are never replaced.
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
- Explorer folders start collapsed. Ignored paths remain visible in gray; files
  show Git status letters and folders show aggregate change dots. New file,
  new folder, refresh, and collapse-all buttons work from the repository header.
  The navigation search icon opens a searchable file picker.
- Visible tab close buttons, middle-click close, close others/all, and overflow
  arrows that keep the active tab and its close button accessible.
- UTF-8 editing, Tab insertion, Unicode cursor movement, selection, clipboard
  copy/cut/paste, undo/redo, find, and save. Existing BOM, uniform CRLF/LF, final
  newline presence, and file permissions survive saves. External modifications
  are checked before replacing a file.
- Syntax colors through Chroma for recognized languages up to 256 KiB.
- Searchable commands, disabled-command explanations, and Git output panel.

Commands are available from **Ctrl+P**, **F1**, or the sidebar's **···** menu. Clean tabs close
immediately; unsaved files offer **Save / Discard / Cancel**, using either the
buttons or **S / D / Esc**. File mutations and Git operations run only when requested; refreshing
the view does not stage, commit, fetch, or change branches.

## Keys

| Context | Key | Action |
|---|---|---|
| Global | Ctrl+1 / 2 / 3 / 4 | Explorer / Source Control / History / Branches |
| Global | Ctrl+P or F1 | Commands |
| Global | F6 / Esc | Switch focus / focus sidebar |
| Global | Ctrl+B / Ctrl+J | Toggle sidebar / output |
| Global | Ctrl+D | Toggle unified / side-by-side diff |
| Global | F5 | Refresh repository |
| Global | Ctrl+F / F3 | Find / next match in current content |
| Global | Ctrl+G | Go to line or line:column |
| Global | Ctrl+S / Ctrl+W | Save / close editor tab |
| Global | Alt+Left / Alt+Right | Previous / next editor tab |
| Global | Ctrl+Q | Quit, with unsaved-changes confirmation |
| Sidebar | Up/Down or K/J | Select entry |
| Sidebar | Enter or Right | Open file/diff, toggle folder, or switch branch |
| Source Control | E | Edit selected file |
| Source Control | Space | Toggle file staging |
| Source Control | S / U / A / C | Stage / unstage / stage all / commit |
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
File tabs open live worktree files; commit diffs are read-only patches.

Material Icon Theme uses SVG artwork. This terminal version uses a curated set
of its associations/colors with Nerd Font equivalents, not the original SVG
shapes or its entire association catalog. `-icons=false` keeps the colors and
uses plain text markers. No GUI/WebView or terminal image protocol is required.

The initial diff surface clips long lines to pane width and shows Git's default
three lines of context. Horizontal scrolling, full-file diff context, conflict
resolution, line staging, and the other remaining capabilities are in the roadmap.

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
