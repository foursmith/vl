# Command line

## Open a workspace or file

| Command | What it does |
| --- | --- |
| `vl` | Reopen the most recent workspace that still exists. Run `vl .` if there is no recent workspace. |
| `vl .` | Open the current directory as a workspace. |
| `vl path/to/project` | Open another directory as a workspace. |
| `vl path/to/file` | Open a single file in the editor. |
| `vl path/to/file:42:8` | Open a file at line 42, column 8. Line and column numbers start at 1, and the column is optional. |

Put options before the path. Quote paths containing spaces, such as `vl "my project"`. Use `vl -- preview` to open a file or folder named `preview`.

## Startup options

| Option | What it does |
| --- | --- |
| `-h`, `--help` | Show command-line help. |
| `-v`, `--version` | Print the version. |
| `--lang en`, `--lang zh-CN` | Set and save the interface language. VL uses English until you choose a language. |
| `--icons=false` | Use plain-text icons when you haven’t selected a Nerd Font. |
| `--theme ID` | Start with a bundled or previously imported theme, such as `dark_modern` or `light_modern`. |

## Resume a workspace in Rex

On a Rex Server that supports OSC 7577, vl can reopen the current workspace in the same pane after the server restarts. vl restores saved tabs, the selected sidebar view, and file positions. The resumed process uses saved preferences and default startup options.

Add this rule to your Rex `init.lua` to allow workspaces under your home directory. For projects stored elsewhere, use their parent directory in place of `~`. Make sure `vl` is on your shell's `PATH`, then run `rex config reload`. [Rex resume rules](https://www.superlogical.com/rex/docs/customize/resume-rules#adding-a-rule)

```lua
rex.set("resume.trust.vl", {
  command = { "vl", rex.match.under("~") },
})
```

## Print a file preview

`vl preview` prints the file to standard output. To print lines 41 through 60 with their original line numbers, run the following from this repository’s root.

```sh
vl preview --skip 40 --lines 20 --line-numbers cmd/vl/main.go
```

Place preview options after `preview` and before the file path.

| Option | What it does |
| --- | --- |
| `-h`, `--help` | Show preview help. |
| `--skip N` | Skip the first `N` lines. Defaults to `0`. |
| `--lines N` | Print up to `N` source lines. The default, `0`, prints through the end of the file. |
| `--width N` | Limit the output to `N` columns, including line numbers. Defaults to `0` for unlimited width. |
| `--wrap` | Wrap long lines at the specified width. Requires `--width` of at least `2`. |
| `--line-numbers` | Show original source line numbers. |
| `--tab-size N` | Set the tab width for formatted output. Accepts `1` through `32` and defaults to `4`. |
| `--color MODE` | Use `auto` (the default), `always`, or `never`. Use `always` to keep ANSI colors in a pipe. |
| `--theme ID` | Use a syntax theme when color is enabled. Defaults to the saved theme for the current appearance. |

To override a file’s syntax, click the language in the editor’s status bar or run **Change Syntax Highlighting**. VL saves the choice per file and uses it for `vl preview`.
