# Keyboard shortcuts

Use `Ctrl+P` to jump to a file, `Ctrl+Shift+P` to run a command, and `F2` to move
focus between the sidebar and the editor. Files open in Normal mode. See the
[Vim guide](VIM.md#where-vl-differs-from-vim) for the bindings that behave
differently in vl.

Keys are case-sensitive. `Ctrl+P` is a chord. `Space w` and `gg` are sequences.

## Workbench

| Shortcut | Action |
| --- | --- |
| `Ctrl+P` | Quick Open (Go to File) |
| `Ctrl+Shift+P` / `F1` | Show Command Palette |
| `Ctrl+H` | Open Recent Workspace |
| `Ctrl+1` / `Ctrl+2` / `Ctrl+3` / `Ctrl+4` / `Ctrl+5` | Focus Explorer / Search / Changes / History / Pull Requests |
| `Ctrl+Shift+F` | Find in Files |
| `Ctrl+L` | Toggle Sidebar |
| `F2` | Focus the sidebar or the editor |
| `Ctrl+S` | Save |
| `Ctrl+W` | Close Editor. Prompts if the file has unsaved changes. |
| `Ctrl+Shift+W` | Close the other tabs. Prompts for unsaved changes. |
| `Space 1` … `Space 9` | Switch to a tab by number from a focused editor outside Insert mode |
| `Space ←` / `Space →` | Switch to the adjacent tab from a focused editor outside Insert mode |
| `F5` | Refresh the active view |
| `Shift+F10` / right-click | Open the context menu |
| `Ctrl+Q` | Quit. Prompts if there are unsaved changes. |

Quick Open searches files as you type. Prefix the input with `>` for commands
or `:` to go to a line or column. In a picker, use `Up` / `Down` to select a
result, `PageUp` / `PageDown` to page, and `Enter` to accept. `Esc` dismisses
the picker.

`Ctrl+C` copies selected text when the text or diff editor has focus. In the
main workbench outside those editors and text inputs, it can quit vl.

## Sidebar

| Shortcut | Action |
| --- | --- |
| `j` / `k` or `Down` / `Up` | Select and preview the next / previous item |
| `gg` / `G` | Select the first / last item |
| `3j`, `5gg`, `5G` | Move down three items or jump to the fifth item |
| `PageUp` / `PageDown` | Move by a page |
| `h` / `Left` | Select the visible parent. Keep the current fold state. |
| `l` / `Right` / `Enter` | Expand or collapse a folder, or open the selected item |
| `e` | Open the selected file for editing, where available |
| `r` | Refresh the active view |
| `o` (Pull Requests) | Open the selected PR in your browser |

Moving the selection or single-clicking a file reuses the preview tab and keeps
focus in the sidebar. Opening the file with `Enter` or `l`, switching to the
editor with `F2`, or clicking the editor or its tab keeps that tab open. Scrolling
and image zoom leave it in preview. On narrow terminals, use `Enter` to open
the item and `F2` to switch back to the sidebar.

### Explorer

| Shortcut | Action |
| --- | --- |
| `L` | Split the selected compact folder chain |
| `dd` / `Delete` | Delete the selected file or folder after confirmation |
| `yy` | Yank the selected file or folder into the Explorer's file clipboard |
| `p` | Paste into the selected folder or next to the selected file. Confirm the destination path. |

`dd` and `yy` act on one item at a time, including its children if it's a folder.
Save edits to the source before copying. Paste stays within the current
workspace, refuses symlinks and special files, and skips `.git` metadata.
Existing paths are protected. Copying into the source's parent suggests a
name ending in `-copy` if the default destination is the source itself.

### Changes

| Shortcut | Action |
| --- | --- |
| `s` | Stage a working-tree row or unstage a staged row. Applies to the file, folder, or group selected. On the repository header, stages the repository. |
| `Shift+S` | Stage all in the selected repository from a working-tree row, or unstage all from a staged row. On the repository header, stages first when there are stageable changes. |
| `c` | Commit |
| `d` | Open Discard Changes for the selected unstaged item. Confirm with `x` or the dialog. |
| `Shift+D` | Discard all unstaged changes in the selected repository, with confirmation. Available from working-tree rows and repository headers. |
| `t` | Open Stash Changes, including untracked files |
| `f` | Fetch |
| `p` / `P` | Pull (fast-forward only) / Push |

The row buttons, context menu, and Command Palette use the same scope.
Repository-wide actions include collapsed files and act on one Git index.

## Find and Replace

| Shortcut | Action |
| --- | --- |
| `Ctrl+F` / Normal mode `/` | Open Find in the editor |
| Normal mode `n` | Find Next |
| `↑` / `↓` in Find | Jump to the previous / next match |
| `Enter` in Find | Jump to the match and return focus to the editor |
| `Tab` / `Shift+Tab` | Switch between Find and Replace, or return to the editor when Replace is collapsed |
| `Enter` in Replace | Replace the current match |
| `Esc` | Close Find and hide highlights |

Find opens as an overlay in the upper-right corner. It shares the query,
replacement, and match options with workspace Search. Find Next and Find
Previous wrap around. Use Match Case, Match Whole Word, or Use Regular
Expression to change how the query matches. Replace also supports Preserve
Case. Use the chevron to toggle the Replace field and the Replace All button to
replace every match in the file. Replacement text can include capture groups
like `$1` and `${name}`. In-file replacements stay in the buffer until you save.
Read-only diffs support Find.

### Search across files

| Shortcut | Action |
| --- | --- |
| `Ctrl+Shift+F` / `Ctrl+2` | Find in Files |
| `Tab` / `Shift+Tab` | Cycle through the visible fields and results |
| `Enter` in the search field | Run the search and focus the results |
| `Esc` from a field | Focus the results |
| `/` / `Ctrl+F` in the results | Focus the search field |
| `j` / `k`, `gg` / `G`, `PageUp` / `PageDown` in the results | Move through matches and file groups |
| `h` / `l` in the results | Select the parent file group / expand it or open the selected result |
| `Enter` in the results | Open the selected file or match and keep the tab |
| `dd` / `Delete` in the results | Remove a match or file group from the results. This leaves the file alone. |
| `Enter` in Replace | Replace the selected match, or all retained matches in the selected file group |

Search runs as you type and includes unsaved edits. Selecting a result previews
it. Opening it focuses the editor at the match and highlights the other matches
in that file. Use `n` to keep jumping. Use the chevron to toggle the
Replace field and the Replace All button to replace all retained results. The
search details button shows the include and exclude fields.

Include and exclude fields take comma-separated globs such as
`*.go, **/*.{ts,tsx}`. Use `./` to anchor a pattern at the workspace root.
Excludes win over includes. You can combine the open-file and changed-file
filters with globs and ignore rules. A result you remove with `dd` is skipped
during replacement.

**Replace in Files saves to disk immediately**, including any edits already in
the affected open buffers. Open files keep an undo step for the replacement.
If a result goes stale, run the search again before replacing.

## Diff editor

The diff editor is read-only. Use the same motions, Visual selections, yanks,
Find, and Flash bindings as in a text tab.

| Shortcut | Action |
| --- | --- |
| `[` / `]` in Normal mode | Previous / Next Hunk |
| `s` | Stage or unstage selected changed lines, or the current hunk when there is no selection |
| `Shift+S` | Stage or unstage the entire file |
| `d` | Discard selected changes or the current hunk, with confirmation |
| `Shift+D` | Discard all unstaged changes to this file, with confirmation |
| `Ctrl+D` | Toggle Inline View (inline / side by side) |
| `Shift+F10` / right-click | Open diff actions, including staging, unstaging, and discarding unstaged hunks or selected changes |
| `Ctrl+C` | Copy selected source text to the system clipboard |
| `Space 1` … `Space 9` / `Space q` | Switch tabs / close the tab |

In a side-by-side diff, keyboard navigation uses the right pane. You can
select text with the mouse in either pane.
Yanks and copies leave out line numbers and diff markers. Added or deleted
files open inline, and side-by-side falls back to inline when space runs out.

Hover a working-tree hunk to reveal `→` (discard, with confirmation) and
`+` (stage). Hover a staged hunk to reveal `−` (unstage). If the hunk
contains selected changes, its buttons act on those changes within the hunk.
Use the whole-file action for binary diffs. History and pull request diffs
are read-only.

After staging or unstaging, a Diff tab follows the changes when its current
side becomes empty. If both sides are open, the tab for the resulting side is
reused.

## Image preview

| Shortcut | Action |
| --- | --- |
| `+` / `=` / `-` | Zoom in / out around the viewport center |
| Mouse wheel | Zoom around the pointer |
| `h j k l` / arrow keys / drag | Pan. The keys take a count. |
| `0` | Fit to window |
| `Enter` | Keep the preview tab open |
| `Space 1` … `Space 9` / `Space q` | Switch tabs / close the tab |

## Menus and dialogs

| Shortcut | Action |
| --- | --- |
| `k` / `j`, `Up` / `Down`, `Shift+Tab` / `Tab` | Select the previous / next menu item |
| `Home` / `End` | Select the first / last menu item |
| `l` / `Enter` / `Space` | Run the selected action |
| `h` / `Esc` / `Shift+F10` | Close the context menu |

In confirmation dialogs, `h` / `l` switch buttons, `j` / `k` scroll the details,
`Enter` activates the focused button, and `Esc` cancels.
