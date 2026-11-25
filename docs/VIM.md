<p align="center">
  English · <a href="VIM.zh-CN.md">简体中文</a>
</p>

# Vim keys in vl

Vim mode is enabled by default. Press `Ctrl+P` and choose **Preferences: Toggle Vim Mode** to switch immediately, or set `features.vimMode` to `false` in `~/.config/vl/vl.config.json`.

In Vim mode, text files open in Normal. Press `i` for Insert, `v` for Visual, and `Esc` or `Ctrl+[` to return to Normal. Read-only diffs share the same motions, selections, yanks, and jumps. See [Keyboard shortcuts](KEYBINDINGS.md) for workbench controls.

## Where vl differs from Vim

| Key | In vl | In stock Vim |
| --- | --- | --- |
| `H` / `L` | First nonblank / end of the current line. Think `^` / `$`. | Top / bottom of the window |
| `f`, `df`, `cf`, `yf` | Flash jump, or an operator up to a labeled target in the viewport | `f{char}` finds a character on the current line |
| `U` | Redo. `u` still undoes. | Restore the last changed line. Redo is `Ctrl+R`. |
| `:` | Enter a save or quit command, or jump to a line or column. Try `:42` or `:42:8`. | Enter an Ex command |
| `/`, `n` | Open the Find widget and jump to the next match. | Enter a search pattern and repeat the search. `N` goes in the opposite direction. |
| `Space` | Leader in an editor tab. Try `Space →` to switch to the next tab. | Move right in Normal mode |
| `Ctrl+V` | Paste from the system clipboard in the text editor | Start Visual block in Normal mode |
| `Ctrl+R` | Open Recent Workspace | Redo in Normal mode |
| `Ctrl+F` / `Ctrl+W` | Find / Close Editor | Page forward / start a window command in Normal mode |
| `Tab` / `Shift+Tab` | Indent / outdent in Normal and Visual | `Tab` jumps forward in the jumplist in Normal mode |
| Backtick | Move back to a word start and toggle case. In Visual, extend the selection with that motion and toggle its case. | Jump to a mark |
| `[` / `]` in a diff | Previous / next hunk | Prefixes for bracket commands |

For Vim's defaults, see [motions](https://vimhelp.org/motion.txt.html),
[undo and redo](https://vimhelp.org/undo.txt.html), and the
[command index](https://vimhelp.org/index.txt.html).

**Clipboard.** `y` yanks to the unnamed register. `d`, `c`, `x`, and `s` also store deleted text there. `p` / `P` paste from that register, which is shared across editor tabs. `Ctrl+C` / `Ctrl+X` / `Ctrl+V` use the [system clipboard](KEYBINDINGS.md#text-editing). Explorer's `yy` / `p` copy and paste files.

Visual block, macros, named registers, mark jumps, and Ex commands beyond those listed below are unsupported.

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

The `w`, `b`, and `e` motions and the `iw` and `aw` text objects use Vim's default UTF-8 word classes. For example, `foo_中文かな。🙂` has five words (`foo_`, `中文`, `かな`, `。`, and `🙂`). An empty line is a stop for `w` and `b`.

In Normal mode, `/` opens Find and `n` jumps to the next match and restores hidden highlights. Jumping centers the match in the viewport. See [Find and Replace](KEYBINDINGS.md#find-and-replace) for more shortcuts.

### Chinese word segmentation

Set `features.chineseWordSegmentation` in `~/.config/vl/vl.config.json` to use Chinese word boundaries for Vim word motions, `Ctrl+Left` / `Ctrl+Right`, and double-click selection. The latter two also work with Vim mode off.

```json
{
  "features": {
    "chineseWordSegmentation": true
  }
}
```

Download one Jieba dictionary into `~/.config/vl/extra/` and reopen vl. If more than one dictionary is present, vl checks the rows from top to bottom and uses the first usable file.

| Dictionary | Download size | Vocabulary |
| --- | ---: | --- |
| [`dict.txt`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/jieba/dict.txt) | 4.84 MiB | Jieba's standard dictionary. |
| [`dict.txt.small`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.small) | 1.48 MiB | The fewest entries and the smallest download. |
| [`dict.txt.big`](https://raw.githubusercontent.com/fxsjy/jieba/v0.42.1/extra_dict/dict.txt.big) | 8.19 MiB | The most entries, with broader coverage of Traditional Chinese. |

For unknown-word recognition, also download [CppJieba's `hmm_model.utf8`](https://raw.githubusercontent.com/yanyiwu/cppjieba/v5.6.7/dict/hmm_model.utf8) into that directory.

## Editing in Normal mode

| Key | Action |
| --- | --- |
| `i` / `a` | Insert before / append after the cursor |
| `I` / `A` | Insert before the first nonblank / append at end of line |
| `o` / `O` | Open a line below / above and enter Insert |
| `x` / `s` | Delete the character / substitute it and enter Insert. Both take a count. |
| `r{char}` | Replace a character in place. `3rX` writes `XXX`. |
| `dd` / `cc` / `yy` | Delete / change / yank a line |
| `gcc` / `3gcc` | Toggle comments on the current line / three lines starting at the cursor |
| `gc{motion}` / `gcap` | Toggle comments on lines covered by a motion or text object |
| `gcu` / `gcgc` | Remove the current block comment or adjacent line comments |
| `D` / `C` | Delete / change to end of line |
| `J` | Join the current line with the next. `3J` joins three lines. |
| `p` / `P` | Put after / before the cursor. Linewise text goes below / above. |
| `u` / `U` | Undo / redo. Both take a count. |
| `.` / `3.` | Repeat the last change once / three times |
| `Tab` / `Shift+Tab` | Indent / outdent the current line |

A single line uses a line comment and multiple lines use one block comment when the language supports it. For languages with only one comment style, both use that style.

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
| `gc` | Toggle comments on the selected lines and exit Visual |
| `Tab` / `Shift+Tab` | Indent / outdent and keep the selection |
| `>` / `<` | Indent / outdent and exit Visual |
| `S{delimiter}` | Surround the selection |
| `Esc` / `Ctrl+[` | Clear the selection and return to Normal |

## Insert mode

Press `Esc` / `Ctrl+[` to return to Normal. See [Text editing shortcuts](KEYBINDINGS.md#text-editing) for cursor movement, selection, and clipboard operations.

Type `./` or `../` to complete a path relative to the current file. Accepting a directory adds `/` and lists its contents. Quote paths containing spaces. [Selecting a completion](KEYBINDINGS.md#move-through-lists)

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

## Save and quit commands

In Normal mode, press `:`, type the command, then press `Enter`.

| Command | Action |
| --- | --- |
| `:w` / `:w!` | Save the current file |
| `:q` | Close the current editor if it has no unsaved changes |
| `:q!` | Close the current editor and discard its unsaved changes |
| `:wq` / `:wq!` | Save the current file and close its editor |

Save commands with `!` still check for read-only files and changes on disk. To open a file named after one of these commands, prefix its name with `./`. In single-file mode, closing the last tab exits vl.

Use `Space` sequences to switch tabs, save, and close. See [Workbench shortcuts](KEYBINDINGS.md#use-space-for-tabs-saving-and-closing).
