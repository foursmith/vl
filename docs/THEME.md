# VS Code color themes

The 10 bundled JSON files in `internal/theme/themes/` come from
`contributes.themes[].path` in the `vscode.theme-defaults` extension of VS Code 1.135.0
(`08d4889f9ec4a1685d257b9b95de036c8e1ce1e5`). The corresponding contribution
IDs, labels, and theme types are recorded in `internal/theme/builtin.go`.

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
theme picker with the current selection marked. Themes are grouped into dark,
light, dark high contrast, and light high contrast.
Arrow keys preview the highlighted theme immediately. Enter confirms it and
remembers it for that appearance; Escape or switching to another picker restores
the committed theme. Preview does not change either saved theme. External changes
can update the committed theme while previewing; cancellation restores that latest
selection. The 10 themes from `vscode.theme-defaults` are bundled. Use a
builtin ID such as `dark_modern`, `light_2026`, or `hc_light` to
select one from the command line.

## Import from VS Code

Choose **Import Themes from VS Code** at the top of **Preferences: Color Theme**.
vl finds the local VS Code installation and scans its built-in and installed
extensions for color themes. It reads the extension contribution's label,
`id`, and `uiTheme`. A contribution without an `id` uses its label as its ID.
Imported IDs include the extension identity, for example
`vscode:antfu.theme-vitesse/Vitesse Dark`. You can pass that ID to `-theme`.

The importer resolves JSON/JSONC includes and TextMate references into
self-contained JSON files under `~/.config/vl/themes/vscode/`. Includes apply
first. The child overrides color keys and appends token rules. Missing includes,
cycles, and excessive nesting are reported. Each contribution's `uiTheme`
sets its color category. Saved themes load directly from the import catalog.

Importing skips bundled themes and existing imported IDs. The picker
refreshes after import and preserves the current selection. If some themes fail
to import, vl reports the error and keeps the successful imports.

## Saved selections

The committed selection (`theme`) and remembered pair (`darkTheme`,
`lightTheme`) are saved by ID in `~/.config/vl/vl.json` alongside sidebar
preferences. Confirming a theme writes this snapshot immediately, independently
of terminal notification support. A
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
Quick Input, menus, tab/group headers, status bar, Git decorations,
and editor/diff surfaces. Text colors retain RGBA until actual painting. This
matters when the same text appears on a selection, a token background, or a diff
line. Inactive tab/placeholder/description defaults follow overridden source
colors rather than hardcoded RGB approximations.

Quick Input uses its own focus colors even when keyboard focus is in its input;
menus use `menu.*`; active tabs use `tab.activeForeground`, not generic strong
text. The single editor group's active state does not change merely because
sidebar keyboard focus changes. The status bar uses the sidebar background and
text color by default. While editing text, **Standard** applies
VSCodeVim's mode colors to the whole bar.
**Pure** applies the same mode colors to the mode label and uses the
sidebar colors for the other items.

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
The bundled theme JSON files and their dependencies are embedded under
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

The editor uses TextMate syntax highlighting. Within the syntax highlighting
size limit, recognized files show guides at their indentation levels. Once
syntax highlighting is ready, the innermost bracket pair at the cursor gets a
vertical guide between the two bracket rows. Matched brackets use foreground
colors based on nesting depth. TextMate scopes exclude brackets in strings
and comments.

The current line uses `editor.lineHighlightBackground`. When the theme supplies
a border instead, the editor draws horizontal rules across the text area along
the current line and the preceding visible text line. The gutter and row spacing
stay fixed.
The current-line highlight is hidden while text is selected.
Search and diff use colored underlines for their border colors. Sidebar
navigation uses the horizontal activity bar's foreground roles and a selection
tint when needed. The picker and context menu use the focused row's background
color, with a marker in monochrome mode. Code and control text use a 4.5:1
contrast floor against the final cell background.
