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

## Syntax mapping

The editor and diff share `internal/highlight`. Chroma token categories map to
Dark Modern's inherited token palette. Declarations are blue `#569CD6`, control
flow purple `#C586C0`, functions yellow `#DCDCAA`, types teal `#4EC9B0`, variables
light blue `#9CDCFE`, strings `#CE9178`, comments `#6A9955`, numbers `#B5CEA8`,
operators `#D4D4D4`, constants `#4FC1FF` and escapes `#D7BA7D`.

Go import needs an explicit scope correction because Chroma groups it with
package declarations. Chroma is a lexical tokenizer, not VS Code's TextMate
and language-server semantic token pipeline: sharing a palette does not imply
identical classification of every identifier. Diff hunks also omit surrounding
source, so lexical state is reset between hunks.
