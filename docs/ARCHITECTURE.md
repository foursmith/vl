# Architecture

## Product boundary

vl owns a complete Git/diff workflow and basic text editing. ttt is a UI
and interaction reference, not a copied runtime. Features such as language-server
hosting, Lua extensions, and embedded terminals are outside the initial scope.

## Ownership

One Bubble Tea program owns the terminal for the application lifetime.
`app.Runtime` replaces workspace models without leaving the alternate screen.
Before replacement, the runner saves the current tabs and file positions;
Runtime cancels the old model's background work and deletes its Kitty images.
Terminal dimensions, capabilities and appearance subscriptions carry into the
new model. Command results carry a workspace generation, checked before Bubble
Tea handles terminal effects or delivers messages to the active model.

```text
cmd/vl                     flags, process lifetime, concrete composition
  └── internal/app         Bubble Tea model/update/view, commands, task delivery
        ├── internal/git        system Git adapter, status and repository results
        ├── internal/diff       patch parsing, line pairing, single-hunk extraction
        ├── internal/imagepreview bounded bitmap decoding, optional SVG conversion, and Kitty image encoding
        ├── internal/editor     document bytes, cursor, selection, undo and save
        ├── internal/files      lazy visible-directory reads and entry creation
        ├── internal/watcher    bounded/coalesced directory notifications
        ├── internal/highlight  TextMate scopes → Dark Modern theme spans
        ├── internal/platform    system browser opener
        ├── internal/i18n       English message keys and Simplified Chinese catalog
        ├── internal/theme      Modern Dark workbench colors
        └── internal/fileicon   Material-style terminal icon associations
```

Git, diff, and editor packages do not import Bubble Tea or own widgets. The
application owns input routing, dialogs, focus, layout, result identity, and
operation serialization. Highlighting remains presentation data; it cannot
change document contents. The status-bar reference picker owns branch and tag
selection through a single checkout path. Repository-header branch labels reuse
the same picker with an explicit repository target, retained when selecting a
create-from reference or entering a new branch name. File search, commands and references
share one picker layout for rendering, hover and click targets. A selected index
and scroll offset provide continuous scrolling and page navigation, with a shared
input for search and file-position syntax.
Global shortcuts use Bubbles key bindings, also read by command/menu labels.
Dialogs, pickers and context menus compose through one Lip Gloss layer helper.
Changes file, folder and group buttons share action availability rules.
Changes and History share repository headers. History state and asynchronous
file-list results use repository plus commit ID as their identity, since the same
commit can appear in more than one repository. Child history files retain workspace
paths for display and an owning repository for Git queries; branch/tag labels come
from that repository's refs. History, branch pickers and status controls read the
same repository snapshot, including metadata errors, without a separate history map.

Display language belongs to presentation. `internal/i18n` translates complete
messages and formats their arguments; command IDs, operation identities, paths,
file contents and raw Git output remain unchanged. The language picker and
`-lang` accept `en` and `zh-CN`. The selected language is saved with layout
preferences and retained when switching workspaces.

## Dependencies

Use Charm's compatible v2 stack: Bubble Tea for event/render lifecycle, Bubbles
for text input, key bindings and the diff viewport, Lip Gloss for styling/layout,
and Charm's ANSI utilities for cell-width-aware clipping. `textmate-go` supplies stateful
syntax tokens, matched against the embedded VS Code Dark Modern theme inheritance
chain; `atotto/clipboard` supplies system clipboard access. `fsnotify` monitors
parent directories, and `uax29/graphemes` defines Unicode editing boundaries.

The Bubbles textarea is not the document engine: its input sanitizer expands
tabs and it imposes a line-count cap. Files instead use a small explicit document
model, preserving their bytes at the save boundary.

System Git is the source of truth. Pass arguments directly through `os/exec`,
never shell strings. Use NUL-delimited status/file records, literal pathspecs,
disabled color/external-diff/textconv, and bounded command lifetimes. The service
does not install hooks or alter a repository's configuration.

## State and concurrency

- Only `Update` changes product state. Commands capture immutable request data
  and return typed messages.
- Snapshot generations and each diff tab's own generation reject obsolete
  results and cancel replaced requests. Snapshot reads are bounded parallel
  tasks. Working-tree, index and commit diffs share rendering and request code;
  each tab owns its patch, viewport, hunk selection, syntax and cancellation.
  Async replies carry their owning diff state, so switching or closing a tab
  cannot redirect its results into another tab. Background refresh reads each
  open working-tree/index tab's source and preserves the selected tab.
- Each expanded commit owns its file cache, directory folds and file-list request
  identity. Late lists can populate their own tree but cannot replace another
  commit's active diff. Commit rows never carry mutable Changes targets.
- Only one Git mutation runs at a time. Background polling pauses during it;
  success or failure is followed by a refresh.
- Hunk application first checks the current diff against the displayed diff.
- Terminal input and filenames are escaped before rendering; ANSI styling is
  generated by the application.
- Save uses a same-directory temporary file, mode preservation, fsync, external
  content checks, and replacement. It is not a lock against other editors: a
  concurrent write can still occur between the final check and rename.
- Document line offsets are indexed once per text change. Dirty checks compare
  saved text and encoding flags without serializing the file on every frame.
- Syntax tokenization runs in cancellable, debounced commands with document/text
  identity checks. Its completion never changes scroll position or editor focus.
- Diff layout maps rendered rows to a read-only source document; the shared Vim
  engine owns navigation, selection and search. The viewport tracks row geometry,
  and only visible rows receive ANSI painting.
  Hunk navigation reuses the layout, and unchanged Git output reuses parsed data.
- Editor rendering groups equal styles into runs and caches only visible rows.
  Text, current line, selection and horizontal layout changes invalidate rows;
  scrolling evicts rows outside the viewport. The terminal renders the cursor:
  a blinking bar for insertion and a steady block for Vim navigation.
- Sidebar wheel events scroll the viewport without selection or diff requests.
  Git decorations use one path index instead of a nested file/change scan.
- Undo uses bounded text snapshots (128 entries / approximately 8 MiB retained),
  suitable for the basic editor rather than a large-file document engine.

## UI direction

Keep the workbench structure: horizontal sidebar navigation aligned with editor tabs,
file tabs, content controls, optional bottom output, and status bar. Use Modern
Dark semantic colors, with addition/deletion tints in diffs. Material Icon Theme
associations/colors are adapted to Nerd Font glyphs; native SVG rendering is not
part of the initial terminal backend.

Commands must remain discoverable in the palette. Preserve text-editing keys
while an editor owns focus. Disabled commands should explain the missing target.

The editor group uses one ordered `[]tab` and one active index, following ttt's
document/content-tab ownership. A tab holds a document, an image preview, or a
diff state. Each image tab has an image state owning its decoded source,
viewport and cached Kitty Unicode placeholder cells, separate from editable
documents. Image reload replies carry image identity and generation, so closing or
replacing a preview invalidates late results. Chunked PNG uploads ride on the
first placeholder cell through Ultraviolet's APC pass-through. Unchanged cells
do not retransmit. The placeholder grid follows normal clipping and modal overlays;
only owned image IDs are deleted on close or exit. Reported terminal cell dimensions
drive the fit calculation, with an 8×16 pixel fallback. SVG previews send file or
Git blob bytes to `resvg` on `PATH`, with a timeout and bounded PNG output.
Each image tab owns its fit-relative zoom and normalized center. Wheel zoom
preserves the source point beneath the pointer; dragging pans within source
bounds. Raster transforms render only the visible source rectangle into a
viewport-sized PNG, keeping high zoom independent of full-image allocation.
Blur, tab switches and geometry changes cancel an active image drag.
Startup probes the Kitty protocol; a terminal must also support Unicode placements
and 256 colors. Unsupported terminals show a message instead of placeholder glyphs.
All kinds append in opening order and share selection, keyboard cycling,
overflow, hit testing and close behavior. Diff identity includes repository,
path, staged versus working-tree source, and commit ID; reopening the same
source selects its existing tab. Different sources can remain open together.
The model's diff-state binding points at the selected tab for rendering, or an
empty surface while a file or image is selected; it is not a shared diff buffer.

Text, image and diff tabs share one preview slot and explicit preview versus kept state.
Single-click exploration replaces only a clean preview;
double-click/Enter keeps it. Editing a document also keeps its tab. Dirty
tabs can never be replaced by another preview. Resolve file identity before
deduplication, including symlinks. Tab rendering and mouse hit testing share one
layout calculation, including close buttons and overflow controls. Dirty close
requests retain document identity and offer save/discard/cancel; clean closes
preserve the active document when another tab is targeted. Closing a diff
invalidates pending replies and prevents periodic refresh from reopening it.

## Validation boundaries

- Temporary-repository tests establish actual index/worktree/hunk behavior.
- Editor tests establish byte preservation, undo, and external-change rejection.
- Model tests establish routing, stale-result handling, dirty-close guards, and
  exact frame dimensions across narrow/wide terminals and Unicode content.
- A real terminal is still required to assess font glyphs, theme appearance,
  pointer behavior, and terminal-specific key delivery.

Expand modules when a subsystem has a concrete state/lifecycle boundary. Avoid
repeating ttt's concentration of all product behavior in one application model.

Measured workloads and current cost boundaries are in [PERFORMANCE.md](PERFORMANCE.md).

## Filesystem reconciliation and Unicode coordinates

The watcher owns its fsnotify loop, watch reconciliation, one debounce timer and
bounded delivery channel. It watches the repository root, expanded explorer
folders, open files' canonical parent directories and actual Git metadata/ref
folders, including linked-worktree metadata. It avoids recursively watching the
entire worktree. Watch configuration is installed off the UI goroutine; a newly
installed directory emits one invalidation to cover the installation race.
Repeated installation failures are reported once per error rather than causing
a refresh loop. File events merge for 150 ms, capped at one second while writes
continue. A slow reconciliation poll covers missing/unwatched notifications;
if watcher creation fails, the original polling interval remains available.

Only Update replaces documents or schedules repository refreshes. An event
received during a snapshot or Git mutation queues a single follow-up refresh.
File reads capture a document pointer and per-tab generation; stale results are
ignored. Identical saved bytes preserve undo history, clean buffers reload while
retaining the viewport, and dirty buffers keep both local text and their original
save-conflict baseline. External deletion leaves the buffer open. Undoing back
to a clean state retries a previously deferred external change. Close cancels
watchers and outstanding requests.

Document and syntax offsets remain rune offsets. Editing, find selections and
mouse positions snap to UAX 29 extended grapheme boundaries. The document lazily
caches boundaries for at most 128 Unicode lines, invalidated by text changes;
ASCII uses a local fast path. Rendering, tab expansion and mouse hit testing
share complete-grapheme display widths. No Unicode normalization is applied, so
combining sequences, BOM and line-ending bytes retain their original form.
