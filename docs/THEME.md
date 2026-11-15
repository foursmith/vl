# Modern color sources

Colors follow VS Code's **Dark Modern** and **Light Modern**. The reference
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

The default is **Auto (Follow Terminal)**, using
[DEC mode 2031 color scheme notifications](https://contour-terminal.org/vt-extensions/color-palette-update-notifications/).
At startup we probe mode 2031, enable it if supported and currently disabled,
and request the current light/dark scheme with `CSI ? 996 n`. Notifications
(`CSI ? 997 ; 1 n` / `CSI ? 997 ; 2 n`) select Dark Modern / Light Modern
immediately. There is no polling or OSC 11 background-color detection.
Unsupported terminals keep the dark default; manual selection remains available.

Auto leaves the terminal's default background unchanged; cells still receive
explicit palette colors. On exit, errors and repository switches we disable
only subscriptions we enabled, preserving modes already set by the caller.

Use **F1 → Preferences: Color Theme — Auto / Light Modern / Dark Modern**, or
launch with `-theme auto` / `-theme light` / `-theme dark`. Explicit light/dark
choices ignore color scheme notifications. The preference is saved on exit as `theme`
in `layout.json` alongside sidebar preferences; detected colors are not saved
as a fixed choice.

Palettes belong to each model. Switching cancels pending syntax jobs, drops
colored row caches, and requests new editor/diff syntax with the selected theme.
Old asynchronous results are rejected by cancellation guards.

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
`internal/highlight/themes`; their `include` chain is resolved in order.
TextMate scope stacks match the inherited `tokenColors` rules, including
ancestor selectors, specificity, foreground inheritance and font-style resets.
Colors and bold, italic, underline and strikethrough styles reach both renderers.
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
