# Performance audit

Measured on Apple M4 / macOS arm64, Go 1.27.4. Fixtures are local temporary
repositories and generated text. Setup is excluded from benchmark timing.
Results cover the model/service stage named below; terminal transport, screen
painting and remote-network latency require separate acceptance.

## Measurements

| Workload | Before | After |
|---|---:|---:|
| Editor scroll frame, 10,000 lines, 180×55 | 5.94 ms / 11.7 MB allocated | 0.44 ms / 0.68 MB |
| File-tree scroll frame, 10,000 items | 6.08 ms / 11.7 MB | 0.40 ms / 0.53 MB |
| Explorer Git decorations, 5,000 changed files | 47.0 ms | 0.67 ms |
| Diff layout, 200 hunks / 8,000 source lines | 34.5 ms / 45.2 MB | 1.92 ms / 1.15 MB |
| Hunk navigation plus frame | 35.2 ms | 1.24 ms |
| Typing, frame and undo, 5,000 Go lines: foreground work | 132.6 ms / 179 MB | 0.46 ms / 0.70 MB |
| Cursor right/left, roughly 0.9 MB document | 2.05 ms / 4.16 MB | below 0.001 ms / zero allocation |
| Insert and undo, roughly 0.9 MB document | 2.75 ms | 0.055 ms |
| 1 MiB ASCII line: cursor movement plus frame | 5.80 ms / 8.71 MB | 0.26 ms / 0.28 MB |
| Quick-open filtering, 10,000 paths | 0.81 ms | 0.22 ms |
| Change-tree construction, 10,000 changed paths | 2.81 ms / 7.58 MB | 1.70 ms / 2.42 MB |
| Output panel, command with 50,000 output lines | 0.85 ms / 1.79 MB per frame | 0.19 ms / 0.15 MB |
| Git snapshot, 1,000 files / 100 changes / 5 remotes | 104 ms | 59 ms |

The typing row measures foreground work after moving tokenization into a
cancellable background command. Whole-file Chroma tokenization remains about
68 ms / 91 MB allocated for that Go fixture; it is measured separately by
`BenchmarkSyntaxHighlight`. Highlighting waits 30 ms after small-file edits and
200 ms for files above 32 KiB. Clean-file opening schedules it immediately.
Tokenization is capped at 256 KiB and observes cancellation between tokens.

## Changes and checks

- Wheel input scrolls the sidebar viewport without changing selection or
  requesting diffs. Background refresh preserves the scrolled viewport.
- Text, rune/byte line offsets and plain-ASCII metadata are indexed per text
  change. Cursor and dirty-state queries avoid whole-file rune/byte copies.
- Editor style runs are batched. Rendered rows are retained only for the visible
  viewport and invalidated by text, cursor, selection, focus, layout or new
  highlight results.
- Diff layout/search content stays plain; only visible rows are painted. Hunk
  navigation reuses the layout. Identical Git diff results reuse parsed data.
- Replaced diff/snapshot requests are cancelled. Snapshot status, metadata and
  file reads run as three independent tasks. Remote URLs use at most four Git
  subprocesses concurrently and retain Git's `insteadOf` behavior and ordering.
  Status polling disables optional index refresh writes.
- File-tree status projection uses path indexes. Folded change groups retain
  counts without constructing their hidden trees. Plain-text language detection
  is shared by the breadcrumb and status bar instead of rematching every frame.
- Output is bounded to 200 logical lines detached from large command buffers.
  Removed undo/redo entries are cleared so their backing arrays release text.

Tests cover scroll bounds, selection/focus, refresh position, Unicode byte/rune
coordinates, bounded render and undo history, cancellation, stale syntax/diff
results, diff search/hunk mapping, remote URL rewrites, and output retention. Rex smoke acceptance used a 5,001-line
Go file and a 10,000-line / 100-hunk diff: 40 characters inserted and undone
returned to the clean state, and 40 successive hunk jumps reached the selected
hunk while preserving the open editor tab.

## Remaining cost boundaries

The basic editor keeps immutable text snapshots. Inserting/deleting text and
rebuilding line indexes still scale with document size; the open-file limit is
4 MiB. Very long Unicode/tab-heavy lines can still require visual-prefix scans.
Chroma's background work retains the cost shown above. Diff parsing/layout keeps
raw and parsed patch data proportional to patch size. Atomic saves retain fsync
and external-change checks; a 1 MB local-save smoke benchmark took about 8 ms,
with filesystem-dependent latency.

## Reproduce

```sh
go test -p 1 ./internal/app ./internal/editor ./internal/git ./internal/highlight \
  -run '^$' \
  -bench 'Benchmark(ScrollFrame|ExplorerDecorations|DiffRender|HunkNavigation|EditorTyping|CursorMove|InsertUndo|QuickOpenFilter|ChangeTree|LongLineFrame|OutputPanelFrame|Snapshot|SyntaxHighlight)$' \
  -benchtime=500ms -benchmem

go test ./internal/editor -run '^$' -bench '^BenchmarkSave$' -benchtime=3x -benchmem
make check
```
