# Git and diff parity

The goal is a complete Git/diff workbench with a basic editor. This list defines
the acceptance scope; unchecked capabilities are not implied by the first build.

## Foundation — implemented

- [x] Charm v2 runtime and ttt-style workbench structure.
- [x] Modern Dark colors; Material-style terminal file icons.
- [x] Repository discovery, NUL-safe status and file enumeration.
- [x] Staged/unstaged/untracked sections, event-driven refresh and low-frequency reconciliation.
- [x] Unified and paired side-by-side text patches.
- [x] File stage/unstage and stage all, including unborn repositories.
- [x] Modification-hunk stage/unstage with stale-diff rejection.
- [x] Commit staged changes and inspect the latest 100 commits per repository,
      with repository sections, independently expandable file trees, repository-scoped
      ref labels and per-file read-only diffs.
- [x] Local branches: list, create, switch.
- [x] Status-bar branch picker: local/remote branches, tags, detached checkout,
      create from a selected reference, and remote tracking checkout.
- [x] Upstream ahead/behind, fast-forward sync, publish branch, and GitHub jump.
- [x] Fetch, fast-forward pull, push using existing credentials.
- [x] Stash current changes and pop latest stash.
- [x] Confirmed discard of tracked non-conflicted unstaged changes.
- [x] File, folder and group hover actions, scoped batch staging/discard, and
      confirmed removal of individual untracked files.
- [x] File tree, tabs, grapheme-aware Unicode editing, selection, clipboard, undo/redo, save.
- [x] External file monitoring, clean-buffer reload and dirty-buffer conflict protection.
- [x] Palette, output, current-content find, responsive focus/layout.
- [x] Separate Source Control sections for submodules, with independent branches,
      staged/worktree trees, message drafts and commits; file actions use the owning repository.

## Diff completeness — next

- [ ] Horizontal scrolling, wrapping options, full-file context and expandable gaps.
- [ ] Diff directly against arbitrary refs, commits, files, and ranges.
- [ ] Editable worktree diff sharing the file tab's document and undo stack.
- [ ] Per-line and selected-range stage/unstage/discard, with explicit semantics
      for added/deleted/renamed files and files without final newlines.
- [ ] Renames/copies/mode changes and binary files as explicit presentations.
- [ ] Conflict-side views and a merge-resolution workflow.
- [ ] Word-level change highlighting and linked split-pane navigation.
- [ ] History full-file endpoints, pagination and commit topology graph.
- [ ] Patch export/import and clipboard copy.

## Git completeness

- [ ] Branch rename/delete and tracking/upstream reassignment.
- [ ] Commit amend/reword, reset/revert, cherry-pick and sequencer recovery.
- [ ] Merge/rebase, continue/abort/skip, interactive rebase editing.
- [ ] Stash list, inspect, apply/pop/drop selected stash.
- [ ] Tags, remotes, upstream/ahead/behind state, push options and authentication UX.
- [ ] Worktrees, submodule lifecycle management, ignored files, sparse checkouts and LFS behavior.
- [ ] Per-file history, blame, reflog, bisect.
- [ ] In-progress operation discovery and recovery after restarting elegit.

## Basic editor completion and polish

- [x] Explorer new-file/new-folder creation, ignored paths, Git decorations, and
      lazy folder expansion.
- [ ] Save As and reload changed-on-disk documents.
- [ ] Find/replace, go to line, indentation settings, word-aware navigation.
- [ ] Persisted settings, tabs, view state, configurable keybindings.
- [ ] Full Material file association catalog; optional original SVG icon rendering
      on terminals with a supported image protocol.
- [ ] Multi-platform real-terminal acceptance and representative large-repo budgets.

For each lane, validate actual repository/index/file contents, cancellation and
stale replies, plus real-terminal interaction. A placeholder menu item does not
count as implementation; passing a model test does not establish visual approval.
