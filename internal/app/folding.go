package app

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Sticky context and folding share one scope cache. Edits invalidate source
// ranges, so reopen folds instead of hiding unrelated text at stale coordinates.
func (t *tab) syncCodeScopes() {
	if t.scopeText != t.doc.Text {
		t.scopeText = t.doc.Text
		t.scopes = codeScopes(t.doc.Path, t.doc.Text)
		t.folds, t.foldRows = nil, nil
		t.renderedRows = nil
	}
}

func (t *tab) visibleRow(source int) int {
	t.syncCodeScopes()
	if t.foldRows == nil {
		return source
	}
	i := sort.SearchInts(t.foldRows, source)
	if i == len(t.foldRows) || t.foldRows[i] != source {
		i--
	}
	return max(0, i)
}

func (t *tab) sourceRow(visible int) int {
	t.syncCodeScopes()
	if t.foldRows == nil {
		return max(0, visible)
	}
	if visible >= len(t.foldRows) {
		return len(t.doc.Lines()) + visible - len(t.foldRows)
	}
	return t.foldRows[max(0, visible)]
}

func (t *tab) rebuildFoldRows() {
	t.foldRows = nil
	t.renderedRows = nil
	if len(t.folds) == 0 {
		return
	}
	slices.SortFunc(t.folds, compareCodeScopes)
	t.folds = slices.Compact(t.folds)
	row := 0
	for _, fold := range t.folds {
		if fold.start < row {
			row = max(row, fold.end+1) // Nested or sharing a closing/opening line.
			continue
		}
		for row <= fold.start {
			t.foldRows = append(t.foldRows, row)
			row++
		}
		row = fold.end + 1
	}
	for ; row < len(t.doc.Lines()); row++ {
		t.foldRows = append(t.foldRows, row)
	}
}

func (t *tab) revealFold(row int) {
	t.syncCodeScopes()
	before := len(t.folds)
	t.folds = slices.DeleteFunc(t.folds, func(f codeScope) bool { return row > f.start && row <= f.end })
	if before != len(t.folds) {
		t.rebuildFoldRows()
	}
}

// Folding includes the closing line; Python's indentation scopes end before
// the next sibling. Keep the shared sticky scopes unchanged.
func editorFoldScopes(t *tab) []codeScope {
	scopes := slices.Clone(t.scopes)
	ext := strings.ToLower(filepath.Ext(t.doc.Path))
	for i := range scopes {
		if ext == ".py" || ext == ".pyw" {
			scopes[i].end--
		}
		scopes[i].end = min(scopes[i].end, len(t.doc.Lines())-1)
	}
	return slices.DeleteFunc(scopes, func(s codeScope) bool { return s.end <= s.start })
}

func (m *Model) editorFold(command string) {
	t := &m.tabs[m.activeTab]
	t.syncCodeScopes()
	row, col := t.doc.Position()
	scopes := editorFoldScopes(t)
	switch command {
	case "M":
		t.folds = scopes
	case "R":
		t.folds = nil
	default:
		// Closed folds are sorted outermost first, including shared opening lines.
		closed := slices.IndexFunc(t.folds, func(f codeScope) bool { return f.start == row })
		var target codeScope
		if closed >= 0 && command != "c" {
			target = t.folds[closed]
		} else {
			found := false
			for i := len(scopes) - 1; i >= 0; i-- {
				scope := scopes[i]
				if scope.start <= row && row <= scope.end && (command != "c" || !slices.Contains(t.folds, scope)) {
					target, found = scope, true
					break
				}
			}
			if !found {
				return
			}
		}
		switch {
		case command == "o", command == "a" && closed >= 0:
			t.folds = slices.DeleteFunc(t.folds, func(f codeScope) bool { return f == target })
		case command == "O":
			t.folds = slices.DeleteFunc(t.folds, func(f codeScope) bool {
				return f.start >= target.start && f.end <= target.end
			})
		default:
			t.folds = append(t.folds, target)
			if command == "C" {
				for _, scope := range scopes {
					if scope.start >= target.start && scope.end <= target.end {
						t.folds = append(t.folds, scope)
					}
				}
			}
		}
	}

	t.rebuildFoldRows()
	// Closing an enclosing block moves the cursor to its visible header rather
	// than allowing ensureEditor to reopen the newly hidden cursor line.
	if visible := t.sourceRow(t.visibleRow(row)); visible != row {
		row, col = visible, 0
	}
	t.doc.SetPosition(row, col)
	m.ensureEditor()
}

// Vim moves in source lines; translate vertical motions into visible lines only
// when folds exist. Operators keep their source-based ranges.
func (m *Model) moveAcrossFolds(before int) {
	if !m.editing() {
		return
	}
	t := &m.tabs[m.activeTab]
	t.syncCodeScopes()
	if len(t.folds) == 0 {
		return
	}
	after, col := t.doc.Position()
	t.doc.SetPosition(t.sourceRow(t.visibleRow(before)+after-before), col)
	if t.vim.VisualMode {
		t.vim.SyncVisual(t.doc)
	}
}
