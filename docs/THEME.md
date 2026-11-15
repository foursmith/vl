# Dark Modern color sources

Colors follow VS Code's **Dark Modern**, not a screenshot sample. The reference
is the installed VS Code build `08d4889f9ec4a1685d257b9b95de036c8e1ce1e5`.

## Source files

- [dark_modern.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_modern.json): workbench colors, including editor background `#1F1F1F`. Includes Dark+.
- [dark_plus.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_plus.json): function, type, variable and control-flow token colors. Includes Dark (Visual Studio).
- [dark_vs.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/theme-defaults/themes/dark_vs.json): base token colors for declarations, strings, comments, numbers and operators.
- [editorColors.ts](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/editorColors.ts#L267): default diff overlays. Dark Modern does not override them.
- [go.tmLanguage.json](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/go/syntaxes/go.tmLanguage.json): Go token scopes, including blue `keyword.function.go` and purple `keyword.control.import.go`.

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

Gutters retain the editor background. Whole inserted/deleted lines emphasize
their text and line ending; paired replacements emphasize the changed range.

## TextMate syntax

The editor and diff share `internal/highlight`, backed by `textmate-go v0.3.0`.
The three unmodified theme JSON files above are embedded under
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
