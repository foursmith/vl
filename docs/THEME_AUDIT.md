# VS Code theme application audit

Reference: the installed VS Code revision
`08d4889f9ec4a1685d257b9b95de036c8e1ce1e5`, verified from its `product.json`.
This audit follows consumers as well as the JSON theme format. Source links
below use this revision, so defaults do not silently change with upstream main.

| Layer / component | Upstream rule and evidence | vl implementation |
| --- | --- | --- |
| Theme category | Extension contribution `uiTheme` determines the base scheme; JSON includes merge colors/rules, not extension identity. [ColorThemeData](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/services/themes/common/colorThemeData.ts) | Imports use the manifest category, falling back to the root theme's `type` when absent. Includes contribute colors and token rules. |
| Registry defaults | `getColor` checks user color customizations, then the theme map, then `resolveDefaultColor`; registered defaults may reference another ID, transform its alpha, select a scheme variant, or be null. [Color utilities](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colorUtils.ts) | `defaults.json` contains the source-extracted dependency graph for supported roles. `Theme.Color` preserves references, RGBA, and undefined values. No fallback to a second theme's palette. |
| Inherited text | `input.foreground` and `editorWidget.foreground` reference `foreground`; input placeholder and dark description defaults derive alpha from it. [Input](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/inputColors.ts), [base colors](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/baseColors.ts) | Derived defaults follow this theme's overridden foreground, including its existing alpha. |
| Sidebar list states | Focus, selection, hover, and inactive selection are distinct; nullable foregrounds do not emit an overriding color. [Registrations](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/listColors.ts), [list renderer](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/base/browser/ui/list/listWidget.ts) | Empty foregrounds remain empty; component inheritance and decoration precedence are resolved separately from the chosen background. |
| Explorer decorations | An explicit Git label color overrides inherited row text. Only a focused selected list clears the label/badge color with `color: inherit !important`. Text badges use opacity .75; propagated folder bubbles use .4. [Icon label CSS](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/base/browser/ui/iconLabel/iconlabel.css), [decoration renderer](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/services/decorations/browser/decorationsService.ts) | Hover and inactive selection preserve explicit Git decorations. Active selection clears them for label/badge inheritance. Badge opacity is composed against the actual row background. File icons remain independent. |
| Changes / SCM | Resource labels use `fileDecorations: { colors: false, badges: ... }`; the resource status decoration is rendered separately. [SCM renderer](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/contrib/scm/browser/scmViewPane.ts) | Normal/selected label text and the separately colored status marker are different runs. Git status IDs come from [Git Resource.getStatusColor](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/extensions/git/src/repository.ts). |
| Quick Input | Quick Input supplies list overrides for both active and inactive focus/selection, using `quickInputList.focus*` and `contrastActiveBorder`. The input retains input-box styles. [Service](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/quickinput/browser/quickInputService.ts), [registrations](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/quickpickColors.ts) | Picker body, focused row, and input use separate roles. A selection tint makes focus visible when the theme provides only an outline. |
| Menus | `menu.*` has its own registration/fallback graph through dropdown and list colors. [Menu colors](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/platform/theme/common/colors/menuColors.ts) | Context menus no longer reuse Quick Input background or editor foreground. |
| Tabs | Active tab, inactive tab, group header, and border are distinct. Inactive default text alpha derives from overridden active-tab text. Active editor group is not the same as sidebar keyboard focus. [Registrations](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/common/theme.ts), [tab renderer](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/browser/parts/editor/multiEditorTabsControl.ts) | The single editor group uses active-group tab roles even when the sidebar has keyboard focus. Active tab text is not borrowed from generic strong text. |
| Status bar | VS Code registers status bar colors. [Workbench registrations](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/common/theme.ts) | vl shares the sidebar's base background and foreground with the status bar. |
| Token alpha | Theme normalization preserves non-FF alpha; it only removes an opaque FF suffix. [normalizeColor](https://github.com/microsoft/vscode/blob/08d4889f9ec4a1685d257b9b95de036c8e1ce1e5/src/vs/workbench/services/themes/common/colorThemeData.ts) | Token RGBA survives theme resolution and highlighting. Editor/diff painting composes against the actual line/token/selection background. |

## Terminal boundary and scope

The TUI cannot render browser widget shadows. Picker/menu outer frames therefore
remain an explicit terminal adaptation, with a visible edge when the registered
widget/menu border is absent. Horizontal navigation uses `activityBarTop.*`.
Current-line borders become gutter markers, while search and diff borders
become underlines. Code and control foregrounds are checked for contrast after
the cell background and its overlays are resolved. `Theme.Color` retains the
theme's source values.

The renderer covers vl's workbench surfaces and TextMate syntax. VS Code
user-settings overrides, native window effects, font metrics, CSS layout,
and language-server semantic tokens are outside that scope.

`internal/theme/tools/generate-defaults.cjs` uses TypeScript's parser to extract
only the needed registrations and their referenced defaults. It does not execute
VS Code or extension source. The checked-in graph includes a revision and a
source path for every color. Normal builds/tests require only Go.

Validation covers overridden default references, nullable colors, four base
schemes, explicit/manifest type ownership, alpha on multiple surfaces, list
state/decoration precedence, token scope resolution, and distinct component
colors. This audit supersedes the earlier assumption that an inactive selected
Explorer row must discard its Git label color.
