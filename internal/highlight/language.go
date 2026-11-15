package highlight

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
)

type LanguageInfo struct{ Name, ID string }

func Language(path string) LanguageInfo {
	name := strings.ToLower(filepath.Base(path))
	if name == "go.mod" || name == "go.sum" {
		return LanguageInfo{"Go Module", "gomod"}
	}
	byExtension := map[string]LanguageInfo{
		".go": {"Go", "go"}, ".js": {"JavaScript", "javascript"}, ".mjs": {"JavaScript", "javascript"}, ".cjs": {"JavaScript", "javascript"}, ".jsx": {"JSX", "jsx"}, ".ts": {"TypeScript", "typescript"}, ".tsx": {"TSX", "tsx"},
		".json": {"JSON", "json"}, ".jsonc": {"JSONC", "jsonc"}, ".md": {"Markdown", "markdown"}, ".mdx": {"MDX", "mdx"}, ".rs": {"Rust", "rust"}, ".py": {"Python", "python"}, ".html": {"HTML", "html"}, ".css": {"CSS", "css"}, ".scss": {"SCSS", "scss"}, ".vue": {"Vue", "vue"}, ".svelte": {"Svelte", "svelte"}, ".yaml": {"YAML", "yaml"}, ".yml": {"YAML", "yaml"}, ".toml": {"TOML", "toml"}, ".xml": {"XML", "xml"}, ".sql": {"SQL", "sql"}, ".sh": {"Shell", "shell"}, ".swift": {"Swift", "swift"}, ".java": {"Java", "java"},
	}
	if info, ok := byExtension[strings.ToLower(filepath.Ext(path))]; ok {
		return info
	}
	if lexer := lexers.Match(path); lexer != nil {
		name := lexer.Config().Name
		return LanguageInfo{name, strings.ToLower(strings.ReplaceAll(name, " ", "-"))}
	}
	return LanguageInfo{"Plain Text", "text"}
}
