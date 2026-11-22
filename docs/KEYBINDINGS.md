# Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| `Ctrl+G` | Go to File |
| `Ctrl+P` | Open the Command Palette |
| `Ctrl+F` | Find in the current file |
| `F2` | Switch focus between the sidebar and the editor |
| `Ctrl+S` | Save the current file |
| `Ctrl+W` | Close the current editor, with a prompt for unsaved changes |
| `Esc` | Cancel the current mode, menu, or input |

Files open in Normal mode. The [Vim guide](VIM.md) covers motions and
[save and quit compatibility shortcuts](VIM.md#save-and-quit-commands).

Use `Ctrl+U` / `Ctrl+D` to scroll up / down by half the height of the active
view. This works in text and diff editors (including Insert and Visual),
sidebar lists, pickers, menus, and scrollable confirmation details. In
Search, the keys navigate the results. In Find, they scroll the document
while keeping focus on the field. In a zoomed image preview, they pan vertically.

With a list focused, use `j` / `k` to move down / up, `gg` / `G` to jump to
the first / last row, and `PageUp` / `PageDown` to move by a page.

Keys are case-sensitive. Press `Ctrl+G` together and type sequences such as `gg`
one key at a time.

## Workbench

In confirmation dialogs, use `←`/`→` or `Tab` to select a button, `Enter` to activate it, and `Esc` to cancel.

| Shortcut | Action |
| --- | --- |
| `Ctrl+R` | Open Recent Workspace. Prompts if there are unsaved changes. |
| `Ctrl+1` / `Ctrl+2` / `Ctrl+3` / `Ctrl+4` / `Ctrl+5` | Focus Explorer / Search / Changes / History / Pull Requests |
| `F12` | Toggle Sidebar |
| `Ctrl+Shift+W` | Close the other tabs. Prompts for unsaved changes. |
| `Space 1` … `Space 9` | Switch to a tab by number from a focused editor outside Insert mode |
| `Space ←` / `Space →` | Switch to the adjacent tab from a focused editor outside Insert mode |
| `F5` | Refresh the active view |
| `F7` / right-click | Open the context menu |
| `Ctrl+Q` | Quit. Prompts if there are unsaved changes. |

Go to File filters by filename or path as you type. Prefix the input with `>` for
commands, or use `:line[:column]` to jump within the current file. See the
[Vim guide](VIM.md#save-and-quit-commands) for save and quit commands. In a picker,
use `Down` / `Up` or `Ctrl+J` / `Ctrl+K` to select the next / previous
result while typing. `PageUp` / `PageDown` move by a page, and `Home` / `End`
jump to the first / last result. `Enter` accepts and `Esc` dismisses the picker.
Path completion also supports arrow keys, `Ctrl+J` / `Ctrl+K`, and half- or
full-page navigation. `Tab` inserts the selected path.

`Ctrl+C` copies selected text when the text or diff editor has focus.

### Terminal compatibility

Some terminals use legacy input encoding for `Ctrl+1` through `Ctrl+5`. If
these shortcuts are unavailable, use the sidebar icons or open the Command
Palette with `Ctrl+P` and choose the view by name. For workspace search, choose
**Search: Find in Files**.

Windows Terminal can handle `Ctrl+V` itself. The editor accepts the text sent
by the terminal as a bracketed paste. If the terminal has its own text
selection, `Ctrl+C` may copy that selection. Use the editor's context menu to
copy the selection inside vl.

## Sidebar

| Shortcut | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle to the next / previous sidebar tab |
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

Cycling follows the order of the sidebar icons and wraps at both ends.
When cycling to Search, focus starts in the results. Press `/` to edit the query.

### Explorer

| Shortcut | Action |
| --- | --- |
| `o` | Switch focus between the file tree and Outline |
| `L` | Split the selected compact folder chain |
| `dd` / `Delete` | Delete the selected file or folder after confirmation |
| `yy` | Yank the selected file or folder into the Explorer's file clipboard |
| `p` | Paste into the selected folder or next to the selected file. Confirm the destination path. |

`dd` and `yy` act on one item at a time, including its children if it's a folder.
Save edits to the source before copying. Paste stays within the current
workspace, refuses symlinks and special files, and skips `.git` metadata.
Existing paths are protected. Copying into the source's parent suggests a
name ending in `-copy` if the default destination is the source itself.

Outline appears for the active Markdown file. Use `j` / `k` to select a section and
`Enter` to jump to it, or click the entry. Click the Outline heading to toggle
it. From within Outline, press `Left` to collapse it and `Right` to reopen it.

### Changes

| Shortcut | Action |
| --- | --- |
| `s` | Stage a working-tree row or unstage a staged row. Applies to the file, folder, or group selected. On the repository header, stages the repository. |
| `Shift+S` | Stage all in the selected repository from a working-tree row, or unstage all from a staged row. On the repository header, stages first when there are stageable changes. |
| `c` | Commit |
| `d` | Open Discard Changes for the selected unstaged item. |
| `Shift+D` | Discard all unstaged changes in the selected repository, with confirmation. Available from working-tree rows and repository headers. |
| `t` | Open Stash Changes, including untracked files |
| `f` | Fetch |
| `p` / `P` | Pull (fast-forward only) / Push |

The row buttons, context menu, and Command Palette use the same scope.
Repository-wide actions include collapsed files and act on one Git index.

Open **Manage Stashes…** from the Changes context menu or the Command Palette
to inspect stashes in the selected repository. Select a stash to list its files,
then select a file to view its diff. Use **Stash Actions…** in the file list to
apply, pop, or drop the stash. Apply and Pop can also restore the staged state.
Use **New Stash…** to choose what to save, including staged changes only,
tracked changes only, or tracked changes while keeping staged changes in place.

## Find and Replace

| Shortcut | Action |
| --- | --- |
| `Ctrl+F` / Normal mode `/` | Open Find in the editor |
| Normal mode `n` | Find Next |
| `↑` / `↓` or `Ctrl+K` / `Ctrl+J` in Find | Jump to the previous / next match |
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
| `Ctrl+2` | Find in Files |
| `Tab` / `Shift+Tab` from a field | Move through the visible fields and into the results |
| `Enter` in the search field | Run the search and focus the results |
| `Esc` from a field | Focus the results |
| `Ctrl+J` / `Ctrl+K` from a field | Select the next / previous result while keeping focus in the field |
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
| `F7` / right-click | Open diff actions, including staging, unstaging, and discarding unstaged hunks or selected changes |
| `Ctrl+C` | Copy selected source text to the system clipboard |
| `Space 1` … `Space 9` | Switch tabs |

Switch layouts from the diff toolbar or use **Toggle Inline View** in the
Command Palette or diff context menu. In a side-by-side diff, keyboard navigation uses the right pane. You can
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

Click the image or its tab to keep the preview open.

| Shortcut | Action |
| --- | --- |
| `+` / `=` / `-` | Zoom in / out around the viewport center |
| Mouse wheel | Zoom around the pointer |
| `h j k l` / arrow keys / drag | Pan. The keys take a count. |
| `0` | Fit to window |
| `Space 1` … `Space 9` | Switch tabs |

## Menus and dialogs

| Shortcut | Action |
| --- | --- |
| `k` / `j`, `Up` / `Down`, `Shift+Tab` / `Tab` | Select the previous / next menu item |
| `Home` / `End` | Select the first / last menu item |
| `l` / `Enter` / `Space` | Run the selected action |
| `h` / `Esc` / `F7` | Close the context menu |

In Changes, History, and Pull Requests, right-click a repository row or select
it and press `F7` to switch branches. The Changes and History menus also offer
remote actions such as Sync, Pull, Push, or Publish.

Publish sets the upstream for the current branch. With one remote it pushes
directly, and with multiple remotes it opens a picker. With no remotes, sign
in with `gh auth login` to create a private or public GitHub repository from the
publish picker.

In confirmation dialogs, `h` / `l` switch buttons, `j` / `k` scroll the details,
`Enter` activates the focused button, and `Esc` cancels.

## Adding a binding in code

Edit `workbenchKeybindings` in [keymap.go](../internal/app/keymap.go) to bind an
existing workbench command. Each rule declares the command ID, its display
label, the accepted key names, and any focus conditions. Add an alternative key
to the same rule, or add a rule for the same command if it needs a different
focus condition. The command palette and menus read their hints from this table.
Define new operations in [workbench_commands.go](../internal/app/workbench_commands.go).

The workbench compiles its default rules into a keymap shared by dispatch and
shortcut hints. Rule IDs default to their command IDs. Give a rule an explicit
ID when a command has more than one binding with different focus conditions.
The keymap builder accepts ordered assignments, replacements, and removals by
rule ID and preserves each rule's command and focus conditions. A rule with no
default key still gives the user a target to bind.

For a new list, use `listNavigationKey` when typing belongs to a text input,
or `navigationState.key` when the list has focus and supports Vim sequences.
Use `moveListSelection` for row and page movement, and keep component-specific
behavior such as previews and non-selectable headings in the caller.
