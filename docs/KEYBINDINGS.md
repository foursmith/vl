# Keyboard shortcuts

| To find | Press |
| --- | --- |
| A file by name or path | `Ctrl+G` for Go to File |
| Text in the current file | `Ctrl+F` for Find |
| Text across the workspace | `Ctrl+Shift+F` to widen the search |
| An action | `Ctrl+P` for the command palette |

Press keys joined with `+` together. Press sequences such as `gg` and `Space w` one key at a time.

## Workbench

| To | Press |
| --- | --- |
| Switch sidebar views | `Ctrl+1` through `Ctrl+5` for Explorer, Search, Changes, History, and Pull Requests. With sidebar focus, `Tab` / `Shift+Tab` cycle through the same views. |
| Move focus between sidebar and editor | `F2`. Focusing the editor keeps the current preview tab. |
| Show or hide the sidebar | `F12` |
| Cycle editor tabs | `Ctrl+T` for next, `Ctrl+Shift+T` for previous. |
| Save or close | `Ctrl+S` saves, `Ctrl+W` closes the current tab, and `Ctrl+Shift+W` closes the other tabs. |
| Switch workspace, open settings, or quit | `Ctrl+R` opens recent workspaces, `Ctrl+,` opens the configuration file, and `Ctrl+Q` quits. |
| Refresh the current view | `F5`. In Search it reruns the query and keeps focus. In the editor it checks for changes on disk and keeps unsaved edits. |
| Find actions for the current item | `F7` or right-click to open the context menu. |

Closing tabs, switching workspaces, and quitting prompt when there are unsaved changes. In single-file mode, `Ctrl+G` lists files in the current file's directory, and closing the last tab quits. The arrow keys rotate the welcome artwork when it is visible and the main area has focus.

### Use Space for tabs, saving, and closing

These sequences work in a focused editor in Vim Normal or Visual mode, and in image tabs.

| Sequence | To |
| --- | --- |
| `Space 1` through `Space 9` | Switch to a tab by its position from the left. |
| `Space ←` / `Space →` | Switch to the previous / next tab. |
| `Space w` / `Space q` / `Space z` | Save / close / save and close. Saving applies to editable files. |
| `Space Esc` | Cancel the sequence. |

Use `Space dd`, `Space cc`, or `Space yy` to write a line to the system clipboard. The operators also take motions and text objects, as in `Space yw` and `Space yiw`. In Visual, use `Space d`, `Space x`, `Space c`, or `Space y` for the selection. In either mode, `Space p` and `Space P` paste from the system clipboard.

## Move through lists

| How far | Press |
| --- | --- |
| One item | `↑` / `↓` or `Ctrl+K` / `Ctrl+J`. Sidebar lists and menus also accept `k` / `j`. |
| Half a page / a full page | `Ctrl+U` / `Ctrl+D` move half a page up / down. `PageUp` / `PageDown` move a full page. |
| First / last item | `Home` / `End`. Sidebar lists and menus also accept `gg` / `G`. |
| A specific distance or position | Sidebar lists and menus accept counts. `3j` moves down three items. `5gg` or `5G` selects the fifth item. |

**Go to File** filters by file name or path without requiring Git. Type `src/main.go:42:8` to open a file at line 42, column 8, or `:42` to jump to line 42 in the current file. In workspace mode, `F4` toggles ignored files. Type `>` to switch to the command palette.

**Path completion** appears when you type `./` or `../` in the editor. Use the arrow keys, `Ctrl+K` / `Ctrl+J`, or the paging keys to choose a candidate. Press `Tab` to insert it.

## Find and Replace

| To | In the current file | Across the workspace |
| --- | --- | --- |
| Start a search | `Ctrl+F`, or `/` in Normal mode. | `Ctrl+Shift+F` or `Ctrl+2`. Press `/` in the results to return to the query. |
| Go to the previous / next match | In the input, use `↑` / `↓` or `Ctrl+K` / `Ctrl+J`. In Normal mode, `n` goes to the next match. | Use the list navigation keys. From an input, `Ctrl+K` / `Ctrl+J` select results while keeping focus in the input. |
| Leave the query | `Enter` jumps to the match and returns focus to the editor. | `Enter` runs the search and focuses the results. In the list, `Enter` opens the selected result. |
| Move between inputs | `Tab` / `Shift+Tab` switch between Find and Replace. If Replace is collapsed, focus returns to the editor. | `↑` / `↓` move between visible inputs and the results. |
| Replace | In Replace, `Enter` replaces the current match in the editor buffer. | In Replace, `Enter` opens the Replace All confirmation. Use `dd` or `Delete` in the list to exclude a match or file group first. |
| Cancel input | `Esc` closes Find and hides highlights. | `Esc` returns to the results. |

Press `F7` in a search input and choose **Search Options** to set case sensitivity, whole-word matching, regular expressions, and other options. Use the disclosure arrow to show Replace. In file Find, `Ctrl+U` / `Ctrl+D` scroll the document while keeping focus in the input. In workspace search, paging keys move focus to the results and page through them.

Workspace search includes unsaved edits. Expand **Search Details** to limit the scope to open or changed files, or enter include and exclude globs. For example, `*.go, **/*.{ts,tsx}` lists multiple patterns and `./src` limits the scope to the root-level `src` directory. Excludes take precedence. Search and Go to File apply ignore rules in order of precedence: `.rgignore`, `.ignore`, then `.gitignore`. In Search, turn off **Respect Ignore Rules** or name an ignored directory in the include field to search it. Replacement text can refer to regex captures with `$1` or `${name}`.

## Sidebar

Use `h` / `←` to select the parent without changing its fold state. Use `l` / `→` or `Enter` to expand, collapse, or open the selected item. Moving through items reuses the preview tab. Opening a file or pressing `F2` keeps the preview. Press `e` in Explorer or Changes to open the editable file. Sidebar views other than Search also accept `r` or `R` to refresh.

### Explorer

| Press | To |
| --- | --- |
| `yy`, then `p` | Copy the selected file or folder, then paste into the selected folder or the selected file's directory. Confirm the destination path before pasting. |
| `dd` / `Delete` | Delete the selected item after confirmation. |
| `L` | Split the selected compact folder path. |
| `o` / `t` | Focus or leave the Markdown outline / file timeline. |

File operations act on one entry at a time, including its children if it is a folder. Save the source file before copying. Paste stays within the workspace, protects existing destinations, skips `.git`, and rejects symlinks and special files.

In the outline or timeline, `j` / `k` select a row, `Enter` / `l` / `→` jump to the heading or keep the history diff, `h` / `←` collapse the panel, and `Esc` returns focus to the tree. Expanding either panel collapses the other. The file menu and command palette also offer **View File History**.

### Changes

| Press | To |
| --- | --- |
| `s` / `S` | Stage or unstage the selected entry / repository. The selected row's worktree or staged section determines the direction. On a repository heading, staging takes priority when changes can be staged. |
| `d` / `D` | Confirm discarding unstaged changes in the selected entry / repository. |
| `c` / `t` | Commit / stash. The stash shortcut includes untracked files. |
| `f` / `p` / `P` | Fetch / pull with fast-forward only / push. |

Use the context menu or command palette to switch branches, sync, publish, or manage stashes. In Pull Requests, press `o` to open the selected PR in the browser.

## Diff editor

| Press | To |
| --- | --- |
| `[` / `]` | Jump to the previous / next hunk. |
| `Enter` | Expand or collapse nearby unchanged context. |
| `s` | Stage or unstage selected changed lines, or the current hunk when there is no selection. |
| `S` | Stage or unstage the whole file. |
| `d` | Confirm discarding selected changed lines or the current hunk. |
| `D` | Confirm discarding all unstaged changes in the file. |
| `F7` | Open the diff menu, which also lets you switch between inline and split layouts. |

Text diffs share the editor's movement, find, selection, yank, and Flash keys. Use `[`, `]`, and `Enter` outside Visual mode. Git editing keys apply to working-tree and staged diffs. In split view, keyboard navigation uses the right pane and the mouse can select text in either pane. Copies omit line numbers and diff markers. Binary diffs use whole-file operations. History and PR diffs are for review.

## Image preview

`+` or `=` zooms in, `-` zooms out, and `0` fits the image to the window. When zoomed in, use the arrow keys or `h j k l` to pan, with an optional count. `Ctrl+U` / `Ctrl+D` pan up / down by half a screen. Scroll the mouse wheel to zoom around the pointer, or drag to pan. Click the image or its tab to keep the preview.

## Text editing

These shortcuts apply to the focused text editor or input in any Vim mode. Text diffs support Select All and Copy.

| Press | To |
| --- | --- |
| `Ctrl+A` | Select all. |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Copy / cut / paste using the system clipboard. |
| `Ctrl+Z` / `Ctrl+Shift+Z` | Undo / redo. |
| `Ctrl+/` | Toggle comments on the current line or selected lines in the editor. |
| `Ctrl+Backspace` / `Ctrl+Delete` | Delete to the start / end of the line. |

In Insert mode or with Vim disabled, `Home` / `End` go to the start / end of the line, and `Ctrl+←` / `Ctrl+→` move by word. In the editor, `Ctrl+Home` / `Ctrl+End` go to the start / end of the file, and the same paging keys work as in lists. Hold `Shift` while moving to extend the selection. Drag to select text, double-click to select a word, or triple-click to select a line.

Use **Toggle Vim Mode** in the command palette to enable or disable Vim. See the [Vim guide](VIM.md) for more commands.

### Markdown editing

At the end of a list item, task item, or blockquote, press `Enter` to continue it. Ordered lists increment the number, and new tasks start unchecked. Press `Enter` again on an empty item or quote to exit it.

## Menus and dialogs

| Where | Move | Run or exit |
| --- | --- | --- |
| Picker | Use the list navigation keys. | `Enter` selects and `Esc` cancels. In Search Options, `Space` also toggles the selected option when the filter is empty. |
| Context menu | Use the list navigation keys or `Tab` / `Shift+Tab`. | `Enter` / `Space` / `l` / `→` runs the action. A displayed shortcut runs its action directly. `Esc` / `F7` / `h` / `←` closes the menu. |
| Confirmation | `←` / `→`, `h` / `l`, or `Tab` / `Shift+Tab` select a button. Vertical movement and paging keys scroll the details. | `Enter` / `Space` confirms and `Esc` cancels. In unsaved-changes prompts, `s` saves and `d` discards. |
| Text dialog, such as a commit message | Use the text editing keys. | `Enter` submits and `Esc` cancels. |

Pickers and context menus also accept `Ctrl+G`, `Ctrl+P`, `Ctrl+R` (workspace only), `Ctrl+,`, and `Ctrl+Q`. The app treats `Ctrl+[` as `Esc`.

## Terminal compatibility

Your terminal may intercept `Ctrl+Shift+T` or `Ctrl+Shift+F`, or fail to distinguish `Ctrl+1` through `Ctrl+5`. Use the named action in the command palette, click a sidebar icon, or cycle sidebar views with `Tab` / `Shift+Tab` while the sidebar has focus. Some terminals copy their own selection or send `Ctrl+V` as a paste event. Use the editor menu to copy a VL selection when needed.

## Shortcuts you can currently customize

Run **Preferences: Open Keyboard Shortcuts File** from the command palette to edit `~/.config/vl/keybindings.json`. Copy the example below to start with every configurable shortcut and its default keys.

```json
[
  {"command": "app.quit", "keys": "Ctrl+Q"},
  {"command": "diff.discardFile", "keys": "Shift+D"},
  {"command": "diff.discardSelectionOrHunk", "keys": "d"},
  {"command": "diff.nextHunk", "keys": "]"},
  {"command": "diff.previousHunk", "keys": "["},
  {"command": "diff.toggleContextLines", "keys": "Enter"},
  {"command": "diff.toggleLayout", "keys": []},
  {"command": "diff.toggleStageFile", "keys": "Shift+S"},
  {"command": "diff.toggleStageSelectionOrHunk", "keys": "s"},
  {"command": "edit.copy", "keys": "Ctrl+C"},
  {"command": "edit.cut", "keys": "Ctrl+X"},
  {"command": "edit.deleteToLineEnd", "keys": "Ctrl+Delete"},
  {"command": "edit.deleteToLineStart", "keys": "Ctrl+Backspace"},
  {"command": "edit.findInFile", "keys": "Ctrl+F"},
  {"command": "edit.paste", "keys": "Ctrl+V"},
  {"command": "edit.redo", "keys": "Ctrl+Shift+Z"},
  {"command": "edit.selectAll", "keys": "Ctrl+A"},
  {"command": "edit.toggleComment", "keys": "Ctrl+/"},
  {"command": "edit.undo", "keys": "Ctrl+Z"},
  {"command": "editor.closeAllTabs", "keys": []},
  {"command": "editor.closeOtherTabs", "keys": "Ctrl+Shift+W"},
  {"command": "editor.closeTab", "keys": "Ctrl+W"},
  {"command": "editor.nextTab", "keys": "Ctrl+T"},
  {"command": "editor.previousTab", "keys": "Ctrl+Shift+T"},
  {"command": "file.save", "keys": "Ctrl+S"},
  {"command": "preferences.openConfig", "keys": "Ctrl+,"},
  {"command": "preferences.openKeybindings", "keys": []},
  {"command": "sidebar.refresh", "keys": ["r", "Shift+R"]},
  {"command": "sidebar.showChanges", "keys": "Ctrl+3"},
  {"command": "sidebar.showCommitHistory", "keys": "Ctrl+4"},
  {"command": "sidebar.showExplorer", "keys": "Ctrl+1"},
  {"command": "sidebar.showPullRequests", "keys": "Ctrl+5"},
  {"command": "sidebar.showSearch", "keys": "Ctrl+2"},
  {"command": "sidebar.toggleFocus", "keys": "F2"},
  {"command": "sidebar.toggleVisibility", "keys": "F12"},
  {"command": "workbench.goToFile", "keys": "Ctrl+G"},
  {"command": "workbench.openCommandPalette", "keys": "Ctrl+P"},
  {"command": "workbench.openContextMenu", "keys": "F7"},
  {"command": "workbench.refresh", "keys": "F5"},
  {"command": "workspace.findInFiles", "keys": "Ctrl+Shift+F"},
  {"command": "workspace.openRecent", "keys": "Ctrl+R"}
]
```

After editing and saving the file, run **Reload VL** from the command palette.

Omit an entry to use its default. Set `keys` to a shortcut such as `F8` or `Alt+1`, an array of alternative shortcuts, or `[]` to leave the action without a shortcut. Key names are case-insensitive. Write `Shift+S` for the shifted key.

Bindings use the action's built-in focus rules. VL reports and skips entries with unknown commands, invalid keys, or a custom `when` condition.
