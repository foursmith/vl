# Performance audit

Measured on Apple M4 / macOS arm64, Go 1.27.4. Fixtures are local temporary
repositories and generated text. Setup is excluded from benchmark timing.
Results cover the model/service stage named below; terminal transport, screen
painting and remote-network latency require separate acceptance.

## Original audit measurements (commit `5d72709`)

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
cancellable background command. At that commit, whole-file Chroma tokenization took about
68 ms / 91 MB allocated for that Go fixture; it is measured separately by
`BenchmarkSyntaxHighlight`. Highlighting waits 30 ms after small-file edits and
200 ms for files above 32 KiB. Clean-file opening schedules it immediately.
The current TextMate implementation retains the 256 KiB cap and observes cancellation
between lines and bounded retries.

## TextMate migration

With `textmate-go v0.3.0` and the embedded Dark Modern theme chain,
`BenchmarkSyntaxHighlight` on the same 5,000-line Go fixture measured
46–53 ms and 23.0 MB allocated per complete pass (`-benchtime=500ms -count=3`,
run separately from the race suite). This measures warmed grammar/background
work, not a foreground frame or cold startup. The original Chroma measurement
above was about 68 ms / 91 MB; this is a recorded baseline, not a claim of a
speedup for every grammar or input.

Each editor tab now retains a serialized TextMate `Document` across immutable
text snapshots. The background task finds the common line prefix/suffix and
applies `ReplaceLines`; tokenizer state and unchanged tail results are reused
when lexical state converges. Token-to-span and scope-to-theme mappings are
cached, bounded to 20,000 entries and reset when their grammar/theme changes.
The existing debounce, cancellation, stale-result rejection, 256 KiB input cap,
and per-line limits remain. This still splits and compares the snapshot's lines;
it does not add an editor change log or change text storage and undo.

Diff highlighting uses one task-local session for both sides and subsequent
hunks. TextMate caches results by line text and canonical start state; each
snapshot starts its own lexical state, so removed code cannot contaminate the
new side, nor can state cross omitted hunk ranges.

On Apple M4 / Go 1.27.4, two 500 ms runs measured:

| Highlighter workload | Time | Allocation |
| --- | ---: | ---: |
| Fresh full pass, 5,000 Go lines (`BenchmarkSyntaxHighlight`) | 49.5–50.0 ms | 24.2 MB |
| Warm session, one changing line in 5,000 Go lines (`BenchmarkSessionEdit`) | 0.244–0.247 ms | 0.365 MB |
| Old/new 2,000-line diff, separate fresh passes | 38.6–39.4 ms | 15.5 MB |
| Same diff, shared task-local session including initial pass | 21.1–22.8 ms | 9.08 MB |

The edit benchmark includes new snapshot construction, line comparison, and span
lookup; each iteration uses a different replacement. The diff changes one line
and uses unique source lines. These measure warmed-grammar highlighter work,
not opening latency, debounce time, terminal rendering or process footprint.
Edits changing multiline lexical state can require retokenizing the entire tail.
The uncached reference tokenizer is compiled only into tests; production uses
the incremental session exclusively. Run:

```sh
go test ./internal/highlight -run '^$' \
  -bench 'Benchmark(SyntaxHighlight|SessionEdit|DiffHighlightShared)$' \
  -benchmem -benchtime=500ms -count=2
```

## Standalone editor memory

Direct file invocations use a GC growth target of 50 instead of Go's default
100. This trades some allocation throughput for less heap headroom in the
single-file editor. An explicitly set `GOGC` (including `off`) takes precedence;
folder workspaces retain their existing GC policy. This is not a hard memory
limit and does not remove syntax highlighting, undo history or file watching.

On macOS / Apple M4, the before/after executable comparison used the same saved
preferences, 80 × 24 PTYs, a 30 KiB Go file copied from `internal/app/model.go`
and a tiny UTF-8 text file. `vmmap -summary` reported physical footprint:

| Workload | Before | Editor GC target 50 |
| --- | ---: | ---: |
| Text file, idle for 10 seconds | 12.5 MiB | 11.8 MiB |
| Go file, idle for 10 seconds | 19.5 MiB | 17.7 MiB |
| Text file, insert then undo, settle 10 seconds | 13.4 MiB | 12.5 MiB |
| Go file, insert then undo, settle 10 seconds | 24.5 MiB | 19.7 MiB |

These are short local samples, not a memory ceiling. File size, language grammar,
editing history, terminal size and user GC settings affect the result. A separate
heap profile attributed most retained application allocations to TextMate grammar
scanners and regexes, rather than workspace model state.

`BenchmarkEditorEditHighlight` includes an edit/frame/background-highlight cycle
and an undo/frame/background-highlight cycle, including the edit debounce timer.
Three 1-second runs measured 75.3–82.2 ms per cycle at `GOGC=100` and 76.4–79.4 ms
at `GOGC=50`; these samples do not establish a latency improvement. Run the two
settings separately, without concurrent test workloads:

```sh
GOGC=100 go test ./internal/app -run '^$' -bench '^BenchmarkEditorEditHighlight$' -benchtime=1s -count=3
GOGC=50 go test ./internal/app -run '^$' -bench '^BenchmarkEditorEditHighlight$' -benchtime=1s -count=3
```

## Changes and checks

- Wheel input scrolls the sidebar viewport without changing selection or
  requesting diffs. Background refresh preserves the scrolled viewport.
- Text, rune/byte line offsets and plain-ASCII metadata are indexed per text
  change. Cursor and dirty-state queries avoid whole-file rune/byte copies.
- Editor style runs are batched. Rendered rows are retained only for the visible
  viewport and invalidated by text, current line, selection, layout or new
  highlight results.
- Diff search uses the shared source document; the viewport stores only row
  geometry and only visible rows are painted. Hunk
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
TextMate still performs whole-file work in a background command; it does not yet
reuse incremental document state across edits. Diff parsing/layout keeps
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

## File watching and grapheme editing

Repository events are debounced and coalesced while a snapshot/mutation is in
flight. Watch configuration and file reads run outside the UI loop. Existing
watches use a 30-second reconciliation interval instead of unconditional
three-second snapshots; only visible/open worktree directories and Git metadata
are monitored, so unobserved paths can wait for that reconciliation. Creation
failure retains the three-second fallback.

A 200 ms benchmark smoke on the same machine measured cursor right/left at
116 ns / zero allocation, insert+undo at 58 microseconds, the 1 MiB ASCII line
frame at 0.26 ms, editor scrolling at 0.41 ms and sidebar scrolling at 0.36 ms.
These are workload-specific regression checks, not terminal-paint timings.
Grapheme boundaries use a lazy 128-line cache; extremely long Unicode lines
still require segmentation when first visited. ASCII edits use a local boundary
check, avoiding whole-document indexing solely to place the insertion cursor.

Rex smoke checks used a temporary repository: a family ZWJ emoji was removed as
one editing unit and restored by undo; atomic external replacement refreshed the
clean editor and newly created files appeared in the explorer. A second external
write left locally edited buffer text intact. Model tests separately cover
undo-to-clean reconciliation of a deferred external edit.
