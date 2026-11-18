# VS Code color themes

Builtin themes use unmodified VS Code theme files. The reference
is the installed VS Code build `08d4889f9ec4a1685d257b9b95de036c8e1ce1e5`.

## Source files

- [dark_modern.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_modern.json): workbench colors, including editor background `#1F1F1F`. Includes Dark+.
- [dark_plus.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_plus.json): function, type, variable and control-flow token colors. Includes Dark (Visual Studio).
- [dark_vs.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_vs.json): base token colors for declarations, strings, comments, numbers and operators.
- [editorColors.ts](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/editorColors.ts#L267): default diff overlays. Dark Modern does not override them.
- [go.tmLanguage.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/go/syntaxes/go.tmLanguage.json): Go token scopes, including blue `keyword.function.go` and purple `keyword.control.import.go`.

- `light_modern.json`, `light_plus.json`, and `light_vs.json` from the same pinned
  VS Code revision provide Light Modern workbench and inherited token colors.
  The unmodified files are embedded beside the dark themes.

## Switching

vl remembers one dark and one light theme (Dark Modern and Light Modern by
default). It selects the matching theme at startup and on external light/dark
transitions. A deliberate selection that differs from the external appearance
stays active across transitions and restarts. Automatic switching resumes when
the external appearance matches the selection again.

Terminal appearance uses
[DEC mode 2031 color scheme notifications](https://contour-terminal.org/vt-extensions/color-palette-update-notifications/).
At startup we probe mode 2031, enable it if supported and currently disabled,
and request the current light/dark scheme with `CSI ? 996 n`. Notifications
(`CSI ? 997 ; 1 n` / `CSI ? 997 ; 2 n`) update the appearance immediately.
Until a terminal report arrives, macOS falls back to its system appearance,
checked every two seconds with `defaults read -g AppleInterfaceStyle` outside
the UI loop. Other platforms require terminal reports for automatic changes.
Without either source, vl uses the saved selection.

The terminal's default background follows the selected theme's sidebar/chrome
background so padding outside the cell grid blends with the app. Editor/diff cells
paint their own backgrounds. Appearance detection uses terminal scheme reports
or the system appearance, rather than sampling this app-controlled background.
On exit, errors and repository switches we disable only subscriptions we enabled,
preserving modes already set by the caller.

Ctrl+Shift+P contains one **Preferences: Color Theme** command. It opens a separate searchable
theme picker with the current selection marked and dark themes before light themes.
Arrow keys preview the highlighted theme immediately. Enter confirms it and
remembers it for that appearance; Escape or switching to another picker restores
the committed theme. Preview does not change either saved theme. External changes
can update the committed theme while previewing; cancellation restores that latest
selection. Builtin themes are Dark Modern, Dark+, Dark (Visual Studio), Light Modern,
Light+, and Light (Visual Studio). CLI identifiers are `dark_modern`, `dark_plus`,
`dark_vs`, `light_modern`, `light_plus`, and `light_vs`.

Place local VS Code themes in `~/.config/vl/themes/` on every platform,
including macOS. JSON, JSONC, and XML `.tmTheme` files appear in
**Ctrl+Shift+P → Preferences: Color Theme**, labelled Local and Light/Dark. Subdirectories
are scanned, so relative include files or a full extension directory can stay
together. Non-theme JSON files such as `package.json` are skipped. A bad theme
reports an error without preventing valid themes from appearing. Reopen the
theme picker after adding or updating a theme; discovery runs in the
background, outside rendering.

Selections from this directory are saved as stable relative identifiers such as
`local:my-theme.json` or `local:my-pack/themes/day.json`. Display names may change
without losing the saved selection. Builtin IDs remain separate even if a local
file has the same name. You can also select a local ID or an external file at
startup:

```sh
vl -theme local:my-theme.json
vl -theme /path/to/theme.json
# If a detached theme has no type declaration:
vl -theme local:day.json -theme-type vs
```

JSON/JSONC comments, trailing commas, relative recursive `include`, and
`tokenColors` arrays are supported. `tokenColors` may also reference a relative
XML `.tmTheme` file. Legacy JSON `settings` and standalone XML `.tmTheme` files
are converted to the same internal theme representation. Includes apply first;
the child overrides color keys and appends token rules. A color value `default`
removes an inherited override. Invalid files and missing includes return errors;
cyclic includes and excessive nesting are rejected.

The loader checks ancestor extension `package.json` manifests for a matching
`contributes.themes[].path`, label, and `uiTheme`. The extension contribution
owns the name/category; a standalone root JSON `type` declaration (`light`,
`dark`, `hc`, `hcLight`) is an import convenience only when no contribution or
explicit override exists. Includes do not change category. Without either source
the fallback is `vs-dark`. `-theme-type` supplies an explicit extension type
(`vs`, `vs-dark`, `hc-light`, `hc-black`) for a detached file. There is no automatic
scan of installed extensions or Marketplace download.

The committed selection (`theme`, `themeType`) and remembered pair (`darkTheme`,
`lightTheme`, with optional `darkThemeType` / `lightThemeType` overrides) are saved
in `layout.json` alongside sidebar preferences. Confirming a theme writes this
snapshot immediately, independently of terminal notification support. A
mismatched manual selection is saved with `themePinned`. A startup or live
appearance change that selects a different theme also writes the snapshot.
Previewing a theme in the picker is temporary until confirmed. Missing
remembered files report an error and fall back to Modern for that appearance.

Each model owns one immutable theme containing its UI palette and compiled token
rules. Switching, including between two dark themes, cancels pending syntax jobs, drops
colored row caches, and requests new editor/diff syntax with the selected theme.
Old asynchronous results are rejected by cancellation guards.

## Workbench color resolution and mapping

The [source audit](THEME_AUDIT.md) traces registrations through the components
that consume them. `internal/theme/defaults.json` is extracted from the pinned
VS Code source: it contains only supported registrations and their dependency
graph, including light/dark/high-contrast variants, references, alpha transforms,
and null defaults. `Theme.Color(id)` first checks this theme's overrides, then
resolves the registration against the same theme. Null stays undefined, rather
than becoming an editor foreground or another theme's color.

The terminal adapter then maps component roles: sidebar lists, input boxes,
Quick Input, menus, tab/group headers, status bar, output panel, Git decorations,
and editor/diff surfaces. Text colors retain RGBA until actual painting. This
matters when the same text appears on a selection, a token background, or a diff
line. Inactive tab/placeholder/description defaults follow overridden source
colors rather than hardcoded RGB approximations.

Quick Input uses its own focus colors even when keyboard focus is in its input;
menus use `menu.*`; active tabs use `tab.activeForeground`, not generic strong
text. The single editor group's active state does not change merely because
sidebar keyboard focus changes. A folder without Git is still an open folder
for status bar styling.

The single-row tab strip adapts `tab.activeBorderTop` to a thin leading marker.
File tabs share Explorer's Git decoration colors. Breadcrumbs use
`breadcrumb.foreground` and `breadcrumb.background`; pinned code rows use
`editorStickyScroll.background`, inheriting the editor background when absent.
The final pinned row uses `editorStickyScroll.border` for its bottom rule,
with the panel border as the terminal default. The rule shares the row cache.

The TUI cannot paint browser shadows. Picker/menu frames therefore remain a
separate terminal adaptation when the registered border is absent or too faint.
This does not rewrite registered colors or brighten the theme's selected-row
background. File icon artwork/color remains independent of the color theme.
High-contrast types are resolved with their own defaults, but terminal cells do
not reproduce every VS Code high-contrast outline or native window effect.
Unsupported UI IDs, font families/sizes, CSS layouts, and extension code are not
rendered. The resolver covers vl's surfaces, not the complete VS Code UI.

## File tree decorations

Explorer's Git label color is explicit; Changes uses normal inherited label
text and independently colored status decorations (`fileDecorations.colors` is
false in VS Code's SCM renderer). Added, untracked, modified, renamed/copied,
deleted, conflicting, staged modified/deleted, and submodule IDs remain separate.
Ignored labels are muted; file/folder icons retain their icon-theme colors.

Hover and inactive selection do not erase an explicit Explorer Git color.
Only focused selection clears label/badge colors to inherit the selected row's
foreground, following `iconlabel.css`. Text badges have opacity .75 and
propagated folder bubbles .4, composited against the actual row background.
Changes status icons keep their own color independently of label selection.

## Terminal diff colors

Terminals need opaque cell colors. Composite VS Code's overlays in the order
`overlay * alpha + background * (1 - alpha)`, rounding RGB channels. Text
emphasis is composited over the corresponding line background.

| Role | VS Code source | Terminal color |
| --- | --- | --- |
| Inserted line | rgba(155,185,85,.2) over editor background | `#383E2A` |
| Inserted text | `#9CCC2C33` over inserted line | `#4C5A2A` |
| Removed line | `#FF000033` over editor background | `#4C1919` |
| Removed text | `#FF000033` over removed line | `#701414` |
| Diagonal fill | `#CCCCCC33` over editor background | `#424242` |

Light Modern composites the same line overlays over white: inserted line
`#EBF1DD`, inserted text (`#9CCC2C40`) `#D7E8B1`, removed line `#FFCCCC`,
removed text `#FFA3A3`, and diagonal fill (`#22222233`) `#D3D3D3`.
Blue selection, white editor/input, gray chrome, and darker Git status colors
keep text legible across the light surfaces.

Gutters retain the editor background. Whole inserted/deleted lines emphasize
their text and line ending; paired replacements emphasize the changed range.

## TextMate syntax

The editor and diff share `internal/highlight`, backed by `textmate-go v0.3.0`.
The six unmodified theme JSON files above are embedded under
`internal/theme/themes`; their `include` chain is resolved in order.
TextMate scope stacks match inherited `tokenColors` using a dot-scope trie
following [vscode-textmate's theme resolver](https://github.com/microsoft/vscode-textmate/blob/main/src/theme.ts):
scope strings/arrays, comma groups, ordered ancestors, direct-child `>` matching,
scope depth and ancestor specificity, later identical-rule overrides, inherited
foreground/background and explicit font-style resets. VS Code ignores unscoped
`tokenColors` defaults; editor defaults come from `colors` (legacy TextMate
settings are converted first). Scope exclusion/boolean expressions are not
implemented by this upstream theme resolver either.

Token RGBA survives scope resolution and highlighting. Painting composites token
backgrounds on the current editor/context surface, then selection overlays and
foreground alpha. Diff line/emphasis overlays take precedence over token
backgrounds. An unmatched token uses the enclosing
scope style or editor foreground, without category-based fallback colors.
Colors and bold, italic, underline and strikethrough styles reach both renderers.
`internal/theme/testdata/generate.cjs` generates reference scope/style cases with
the official `vscode-textmate@9.3.2` package; normal Go tests read the checked-in
fixture without requiring Node or Bun. The resolver's MIT notice is preserved
in `third_party/vscode-textmate-LICENSE`. ttt's centralized style mapping and
VS Code parity tests informed this design; no ttt terminal/style-slot runtime
is required.
There is no Chroma token-category mapping or Go keyword color special case.

The bundled grammar catalog supplies language detection and lazy grammar loading.
Line tokenization carries grammar state through comments, strings and embedded
languages. Work stays in cancellable background commands; each line has a soft
100 ms budget and a 20,000-rune / 80,000-byte cap. Transient time-limit
stops are retried at most twice from the same immutable start state, allowing
cold lazy compilation to finish without discarding multiline context. Files above
256 KiB remain plain. If a line cannot complete within its limits, subsequent lines stay plain
rather than using incomplete state. Diff sides tokenize separately and reset
state between hunks because the patch omits the intervening source.

This implements TextMate syntax highlighting, not language-server semantic
highlighting or bracket-pair colorization.

The current line uses `editor.lineHighlightBackground`, falling back to
`editor.lineHighlightBorder` as a cell background when the theme has no visible
line fill. Terminal cells cannot draw a line outline without consuming source space.
