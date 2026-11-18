package theme

import (
	"encoding/json"
	"slices"
	"strings"
)

type FontStyle uint8

const (
	Bold FontStyle = 1 << iota
	Italic
	Underline
	Strikethrough
)

type TokenStyle struct {
	Color, Background string
	Font              FontStyle
}
type scopeRule struct {
	scope    string
	parents  []string // nearest ancestor first, like vscode-textmate
	depth    int
	style    TokenStyle
	setsFont bool
}
type scopeTrie struct {
	main     scopeRule
	parents  []scopeRule
	children map[string]*scopeTrie
	matches  []scopeRule
}

// This trie follows microsoft/vscode-textmate's MIT-licensed theme.ts algorithm:
// inherit rules while descending dot scopes, merge identical selectors, then
// choose one rule at each scope level by depth and ancestor specificity.
// See third_party/vscode-textmate-LICENSE.
func compileTokens(tokens []rawToken) *scopeTrie {
	var rules []scopeRule
	for _, entry := range tokens {
		var selectors []string
		var single string
		if json.Unmarshal(entry.Scope, &single) == nil {
			selectors = strings.Split(strings.Trim(single, ","), ",")
		} else {
			_ = json.Unmarshal(entry.Scope, &selectors)
		}
		for _, selector := range selectors {
			parts := strings.Fields(selector)
			if len(parts) == 0 {
				continue
			}
			r := scopeRule{scope: parts[len(parts)-1], parents: slices.Clone(parts[:len(parts)-1]), setsFont: entry.Settings.FontStyle != nil}
			slices.Reverse(r.parents)
			r.depth = strings.Count(r.scope, ".") + 1
			r.style.Color = normalizeHex(entry.Settings.Foreground)
			r.style.Background = normalizeHex(entry.Settings.Background)
			if entry.Settings.FontStyle != nil {
				for _, s := range strings.Fields(*entry.Settings.FontStyle) {
					switch s {
					case "bold":
						r.style.Font |= Bold
					case "italic":
						r.style.Font |= Italic
					case "underline":
						r.style.Font |= Underline
					case "strikethrough":
						r.style.Font |= Strikethrough
					}
				}
			}
			rules = append(rules, r)
		}
	}
	slices.SortStableFunc(rules, func(a, b scopeRule) int {
		if n := strings.Compare(a.scope, b.scope); n != 0 {
			return n
		}
		return slices.Compare(a.parents, b.parents)
	})
	root := &scopeTrie{}
	for _, rule := range rules {
		root.insert(strings.Split(rule.scope, "."), rule)
	}
	root.finish()
	return root
}
func (r *scopeRule) merge(in scopeRule) {
	r.depth = in.depth
	if in.style.Color != "" {
		r.style.Color = in.style.Color
	}
	if in.style.Background != "" {
		r.style.Background = in.style.Background
	}
	if in.setsFont {
		r.style.Font = in.style.Font
		r.setsFont = true
	}
}
func (t *scopeTrie) insert(parts []string, r scopeRule) {
	if len(parts) > 0 {
		if t.children == nil {
			t.children = map[string]*scopeTrie{}
		}
		child := t.children[parts[0]]
		if child == nil {
			child = &scopeTrie{main: t.main, parents: slices.Clone(t.parents)}
			t.children[parts[0]] = child
		}
		child.insert(parts[1:], r)
		return
	}
	if len(r.parents) == 0 {
		t.main.merge(r)
		return
	}
	for i := range t.parents {
		if slices.Equal(t.parents[i].parents, r.parents) {
			t.parents[i].merge(r)
			return
		}
	}
	inherited := t.main
	inherited.parents = r.parents
	inherited.merge(r)
	t.parents = append(t.parents, inherited)
}
func (t *scopeTrie) finish() {
	t.matches = append(slices.Clone(t.parents), t.main)
	slices.SortStableFunc(t.matches, func(a, b scopeRule) int {
		if a.depth != b.depth {
			return b.depth - a.depth
		}
		i, j := 0, 0
		for {
			if i < len(a.parents) && a.parents[i] == ">" {
				i++
			}
			if j < len(b.parents) && b.parents[j] == ">" {
				j++
			}
			if i >= len(a.parents) || j >= len(b.parents) {
				break
			}
			if n := len(b.parents[j]) - len(a.parents[i]); n != 0 {
				return n
			}
			i++
			j++
		}
		return len(b.parents) - len(a.parents)
	})
	for _, child := range t.children {
		child.finish()
	}
}
func parentsMatch(scopes, parents []string) bool {
	pos := len(scopes) - 1
	for i := 0; i < len(parents); i++ {
		direct := parents[i] == ">"
		if direct {
			i++
			if i == len(parents) {
				return false
			}
		}
		for pos >= 0 && !scopeMatches(scopes[pos], parents[i]) {
			if direct {
				return false
			}
			pos--
		}
		if pos < 0 {
			return false
		}
		pos--
	}
	return true
}
func scopeMatches(scope, selector string) bool {
	return scope == selector || strings.HasPrefix(scope, selector+".")
}
func (t *Theme) Resolve(scopes []string) TokenStyle {
	style := TokenStyle{Color: t.palette.Foreground}
	for level, scope := range scopes {
		node := t.syntax
		for _, part := range strings.Split(scope, ".") {
			child := node.children[part]
			if child == nil {
				break
			}
			node = child
		}
		for _, rule := range node.matches {
			if !parentsMatch(scopes[:level], rule.parents) {
				continue
			}
			if rule.style.Color != "" {
				style.Color = rule.style.Color
			}
			if rule.style.Background != "" {
				style.Background = rule.style.Background
			}
			if rule.setsFont {
				style.Font = rule.style.Font
			}
			break
		}
	}

	return style
}
