package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestLanguageAndFallback(t *testing.T) {
	for _, tc := range []struct {
		value    string
		language Language
		valid    bool
	}{{"", English, true}, {"en", English, true}, {"zh-CN", Chinese, true}, {"zh", English, false}, {"zh_CN", English, false}, {"zh-Hans", English, false}, {"en-US", English, false}, {"fr", English, false}} {
		if language, valid := Parse(tc.value); language != tc.language || valid != tc.valid {
			t.Errorf("Parse(%q) = %q, %v", tc.value, language, valid)
		}
	}
	for _, language := range []Language{English, Chinese} {
		if got := Text(language, "unknown message %s", "filename.go"); got != "unknown message filename.go" {
			t.Fatal(got)
		}
	}
	if got := Text(Chinese, "Save changes to %s?", "Changes.go"); got != "是否保存对 Changes.go 的更改？" {
		t.Fatal(got)
	}
	if got := Text(English, " %.0f%% · 0 Fit ", 50.0); got != " 50% · 0 Fit " {
		t.Fatal(got)
	}
}

// Catalog changes must preserve formatting, especially warnings with paths and
// counts. Every explicit app message must be present in the Chinese catalog.
func TestCatalogCoverageAndFormats(t *testing.T) {
	format := regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)
	for source, translated := range chinese {
		if strings.TrimSpace(translated) == "" {
			t.Errorf("empty translation: %q", source)
		}
		if strings.Join(format.FindAllString(source, -1), " ") != strings.Join(format.FindAllString(translated, -1), " ") {
			t.Errorf("format mismatch: %q => %q", source, translated)
		}
	}
	paths, err := filepath.Glob("../app/*.go")
	if err != nil || len(paths) == 0 {
		t.Fatal("app sources unavailable", err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "tr" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			message, _ := strconv.Unquote(literal.Value)
			if _, ok := chinese[message]; !ok {
				t.Errorf("%s: missing translation for %q", fs.Position(literal.Pos()), message)
			}
			return true
		})
	}
}
