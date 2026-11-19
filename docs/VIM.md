# Vim keys in vl

Text files open in **Normal mode**. Use `i` to enter Insert mode, `v` to enter
Visual mode, and `Esc` or `Ctrl+[` to get back to Normal. The diff editor is
read-only and shares the motions, selections, yanks, and jumps. See
[Keyboard shortcuts](KEYBINDINGS.md) for the rest of the workbench.

## Where vl differs from Vim

| Key | In vl | In stock Vim |
| --- | --- | --- |
| `H` / `L` | First nonblank / end of the current line. Think `^` / `$`. | Top / bottom of the window |
| `f`, `df`, `cf`, `yf` | Flash jump, or an operator up to a labeled target in the viewport | `f{char}` finds a character on the current line |
| `U` | Redo. `u` still undoes. | Restore the last changed line. Redo is `Ctrl+R`. |
| `:` | Quick Open with the line prompt prefilled. Try `:42` or `:42:8`. | Enter an Ex command |
| `/`, `n` | Open the Find widget and jump to the next match. | Enter a search pattern and repeat the search. `N` goes in the opposite direction. |
| `Space` | Leader in an editor tab. Try `Space w` to save. | Move right in Normal mode |
| `Ctrl+V` | Paste from the system clipboard in the text editor | Start Visual block in Normal mode |
| `Ctrl+H` | Open Recent Workspace | Move left in Normal mode |
| `Ctrl+F` / `Ctrl+W` | Find / Close Editor | Page forward / start a window command in Normal mode |
| `Tab` / `Shift+Tab` | Indent / outdent in Normal and Visual | `Tab` jumps forward in the jumplist in Normal mode |
| Backtick | Move back to a word start and toggle case. In Visual, extend the selection with that motion and toggle its case. | Jump to a mark |
| `[` / `]` in a diff | Previous / next hunk | Prefixes for bracket commands |

For Vim's defaults, see [motions](https://vimhelp.org/motion.txt.html),
[undo and redo](https://vimhelp.org/undo.txt.html), and the
[command index](https://vimhelp.org/index.txt.html).

**Know which clipboard you're using.** `y` yanks into vl's unnamed register.
`d`, `c`, `x`, and `s` also put removed text there. `p` and `P` read that
register, which is shared across editor tabs. For example, `ddp` moves a line
down. In Normal, Visual, and Insert modes, use `⌘C`, `⌘X`, and `⌘V` on macOS
or `Ctrl+C`, `Ctrl+X`, and `Ctrl+V` to copy, cut, and paste using the system
clipboard. Copy and cut require a selection. Cut and paste require an editable
text file. Explorer's `yy` and `p` have their own file clipboard.

vl supports a subset of Vim. Use Visual characterwise or linewise selection.
For saving and closing, use the Leader or workbench bindings below. Visual
block, macros, named registers, Ex commands, and mark jumps are unsupported.

## Motions and search

These motions work in Normal and Visual. Use a count to repeat a motion,
like `3j` or `2w`.

| Key | Motion |
| --- | --- |
| `h j k l` / arrow keys | Left, down, up, right |
| `w` / `b` / `e` | Next word start / previous word start / end of word |
| `0` / `Home` | Start of line, including indentation |
| `^` / `H` | First nonblank on the line |
| `$` / `L` / `End` | End of line |
| `gg` / `G` | First / last line |
| `5gg` / `5G` | Go to line 5 |
| `PageUp` / `PageDown` | Move up / down a page |
| `Ctrl+U` / `Ctrl+D` | Scroll up / down half a screen |

In Normal mode, `/` opens the same Find widget as `Ctrl+F`. Matches are
highlighted and the current match is centered when you jump. `n` advances and
brings back hidden highlights. See [Find and Replace](KEYBINDINGS.md#find-and-replace)
for the full set of bindings.

## Editing in Normal mode

| Key | Action |
| --- | --- |
| `i` / `a` | Insert before / append after the cursor |
| `I` / `A` | Insert before the first nonblank / append at end of line |
| `o` / `O` | Open a line below / above and enter Insert |
| `x` / `s` | Delete the character / substitute it and enter Insert. Both take a count. |
| `r{char}` | Replace a character in place. `3rX` writes `XXX`. |
| `dd` / `cc` / `yy` | Delete / change / yank a line |
| `D` / `C` | Delete / change to end of line |
| `J` | Join the current line with the next. `3J` joins three lines. |
| `p` / `P` | Put after / before the cursor. Linewise text goes below / above. |
| `u` / `U` | Undo / redo. Both take a count. |
| `.` / `3.` | Repeat the last change once / three times |
| `Tab` / `Shift+Tab` | Indent / outdent the current line |

An Insert session groups typing into one undo step. Dot repeat covers ordinary
edits, completed Insert sessions, Visual edits, puts, indentation, case
changes, Flash edits, and Surround. Moving the cursor or using undo during
Insert preserves the previous repeatable change.

### Operators and text objects

Combine `d` (delete), `c` (change), or `y` (yank) with a motion or text object.
Change drops you into Insert. Counts before and after the operator multiply.

| Example | Action |
| --- | --- |
| `dw` / `de` | Delete to the next word start / through the end of the word |
| `cw` | Change through the end of the word. From whitespace, change up to the next word. |
| `d0` / `d^` / `d$` | Delete to the start of the line / first nonblank / end of the line |
| `dj` / `dk` | Delete the current line and the line below / above |
| `2dd` / `d2w` / `2d3w` | Delete two lines / use two `w` motions / use six `w` motions |
| `diw` / `daw` | Delete the word / the word plus adjacent whitespace |
| `ci"` / `da(` / `yi{` | Change inside quotes / delete a pair of parentheses and its contents / yank inside braces |

Use `i` for inner and `a` for around. Text objects also work directly in Visual.

| Object | Inner / around | Details |
| --- | --- | --- |
| Word | `iw` / `aw` | Uses word and punctuation boundaries |
| WORD | `iW` / `aW` | Runs up to whitespace, punctuation included |
| Quotes | `i"` / `a"`, `i'` / `a'`, `i` / `a` followed by a backtick | Scoped to the current line |
| Parentheses | `i(` / `a(` | `)` and `b` are aliases for `(` |
| Square brackets | `i[` / `a[` | `]` is an alias for `[` |
| Braces | `i{` / `a{` | `}` and `B` are aliases for `{` |
| Angle brackets | `i<` / `a<` | `>` is an alias for `<` |

Parentheses, square brackets, and braces can nest and span lines.

## Visual mode

| Key | Action |
| --- | --- |
| `v` / `V` | Characterwise / linewise Visual. Press the same key again to exit. |
| Motion or text object | Extend the selection or select the object |
| `o` | Swap the active end of the selection |
| `y` | Yank and exit Visual |
| `d` / `x` | Delete and exit Visual |
| `c` | Change the selection and enter Insert |
| `p` / `P` | Replace the selection with text from the unnamed register |
| `J` | Join the selected lines, at least two, and exit Visual |
| `Tab` / `Shift+Tab` | Indent / outdent and keep the selection |
| `>` / `<` | Indent / outdent and exit Visual |
| `S{delimiter}` | Surround the selection |
| `Esc` / `Ctrl+[` | Clear the selection and return to Normal |

## Insert mode

| Key | Action |
| --- | --- |
| Arrow keys, `Home` / `End` | Move the cursor |
| `Ctrl+Left` / `Ctrl+Right` | Move by word |
| `Ctrl+Home` / `Ctrl+End` | Move to the start / end of the file |
| `Shift` + a movement key above | Extend the selection |
| `PageUp` / `PageDown` | Move up / down a page |
| `Ctrl+U` / `Ctrl+D` | Scroll up / down half a screen |
| `Ctrl+A` | Select All |
| `⌘C` / `⌘X` / `⌘V` (macOS), or `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Copy / Cut / Paste using the system clipboard |
| `⌘Z` / `⌘⇧Z` (macOS), or `Ctrl+Z` / `Ctrl+Shift+Z` | Undo / redo |
| `Backspace` / `Delete` | Delete the selection or the character before / after the cursor |
| `Enter` / `Tab` | Insert a newline / tab |

Type `./` or `../` to complete a path relative to the current file. Pick a
suggestion with `Up` / `Down` and accept it with `Tab`. Accepting a directory
adds `/` and shows its contents. Quote paths that contain spaces.

## Flash

Press `f`, type a literal query, and hit the label at the target.
Flash searches fully visible text, including pinned context lines. In Visual
mode, the jump extends the selection. The interaction follows
[flash.nvim](https://github.com/folke/flash.nvim).

| Key | Action |
| --- | --- |
| Keep typing | Narrow the matches |
| Target label | Jump to that match |
| `Enter` | Jump to the first target on the current label page |
| `Tab` / `Shift+Tab` | Show labels or page through them |
| `Esc` | Cancel and keep the original cursor and selection |
| `df` / `cf` / `yf` | Delete / change / yank through the first character of the target |
| `ysf` | Pick a target, then type a delimiter to surround the range |

Flash operators use the unnamed register. Dot repeat for `df` and `cf` looks
for the same literal target again.

## Surround

| Key | Action |
| --- | --- |
| `ysiw"` | Surround the word with double quotes |
| `ysaw]` | Surround the word and adjacent whitespace with square brackets |
| `ysw)` / `yse)` | Surround a motion with parentheses |
| `yss)` / `2yss]` | Surround the current line / the current line and the next |
| `ysL]` / `ysG]` / `ysgg]` | Surround through end of line / through the last line / back to the first line |
| `S"` / `S]` in Visual | Surround the selection with quotes / square brackets |
| `cs([` / `cs"'` | Change parentheses to square brackets / double quotes to single quotes |
| `ds(` / `ds"` | Delete the delimiters and keep the contents |
| `2cs([` / `2ds(` | Change / delete the next enclosing pair of parentheses |

Surround takes quotes, backticks, and `() [] {} <>`. Either end of a bracket
pair works. It keeps the inner whitespace and the unnamed register intact.
Undo and dot repeat work on the entire change.

## Folds

| Key | Action |
| --- | --- |
| `za` | Toggle the current fold |
| `zc` / `zo` | Close / open the current fold |
| `zC` / `zO` | Close / open the current fold recursively |
| `zM` / `zR` | Close / open all folds in the file |

Folds follow brace-delimited blocks and Python indentation. Moving skips
folded lines. A search or line jump opens the fold when its target is inside.
Editing clears the fold state. Reopening a file restores saved folds whose
block boundaries still match.

## Leader and tabs

`Space` is the Leader in editor tabs outside Insert mode. Press the keys in
sequence. `Esc` cancels a pending sequence.

| Key | Action |
| --- | --- |
| `Space 1` … `Space 9` | Go to a tab by its position from the left |
| `Space ←` / `Space →` | Go to the adjacent tab on the left or right |
| `Space w` / `⌘S` (macOS) / `Ctrl+S` | Save |
| `Space q` / `Ctrl+W` | Close Editor |
| `Ctrl+Shift+W` | Close the other tabs |
| `Space z` | Save and close |

Tab navigation and closing also work from diffs and image previews. Save
commands apply to editable text files.
