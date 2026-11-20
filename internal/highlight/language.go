package highlight

import (
	"path/filepath"
	"strings"

	"github.com/eugenioenko/textmate-go/grammars"
)

type LanguageInfo struct{ Name, ID, Scope string }

var plainText = LanguageInfo{Name: "Plain Text", ID: "text"}

const maxShebangBytes = 256

var scriptInterpreters = map[string]string{
	"sh": "shellscript", "bash": "shellscript", "dash": "shellscript", "ash": "shellscript",
	"ksh": "shellscript", "ksh93": "shellscript", "mksh": "shellscript", "zsh": "shellscript",
	"fish": "fish", "nu": "nushell", "pwsh": "powershell", "powershell": "powershell",
	"node": "javascript", "nodejs": "javascript", "bun": "javascript", "deno": "javascript", "qjs": "javascript",
	"tsx": "typescript", "ts-node": "typescript", "ts-node-esm": "typescript",
	"coffee": "coffee",
	"python": "python", "pythonw": "python", "pypy": "python",
	"ruby": "ruby", "jruby": "ruby", "perl": "perl", "php": "php",
	"lua": "lua", "luajit": "lua", "fennel": "fennel",
	"awk": "awk", "gawk": "awk", "mawk": "awk", "nawk": "awk",
	"Rscript": "r", "elixir": "elixir", "escript": "erlang", "julia": "julia",
	"groovy": "groovy", "java": "java", "jbang": "java",
	"kotlin": "kotlin", "kotlinc": "kotlin", "scala": "scala", "scala-cli": "scala",
	"swift": "swift", "dart": "dart", "crystal": "crystal", "rust-script": "rust",
	"runghc": "haskell", "runhaskell": "haskell", "ocaml": "ocaml",
	"bb": "clojure", "clojure": "clojure", "clj": "clojure", "hy": "hy",
	"sbcl": "common-lisp", "clisp": "common-lisp", "ecl": "common-lisp",
	"guile": "scheme", "racket": "scheme", "chezscheme": "scheme",
	"dotnet-script": "csharp", "fsharpi": "fsharp",
}

// These distributions commonly add a numeric version to the executable name.
var versionedInterpreters = map[string]bool{
	"python": true, "pythonw": true, "pypy": true, "ruby": true, "perl": true, "php": true,
	"lua": true, "luajit": true,
}

func Language(path string) LanguageInfo {
	name := strings.ToLower(filepath.Base(path))
	if name == "go.mod" || name == "go.sum" {
		return LanguageInfo{Name: "Go Module", ID: "gomod"}
	}
	if info, ok := grammars.InfoForFilename(path); ok {
		return LanguageInfo{info.DisplayName, info.ID, info.ScopeName}
	}
	return plainText
}

// WithShebang falls back to a script interpreter on the first line when the
// filename did not identify a language.
func (l LanguageInfo) WithShebang(source string) LanguageInfo {
	if l.ID != plainText.ID || !strings.HasPrefix(source, "#!") {
		return l
	}
	line, _, _ := strings.Cut(source[:min(len(source), maxShebangBytes)], "\n")
	words := strings.Fields(line[2:])
	if len(words) == 0 {
		return l
	}
	interpreter := filepath.Base(words[0])
	if interpreter == "env" {
		words = words[1:]
		if len(words) > 0 && (words[0] == "-S" || words[0] == "--split-string") {
			words = words[1:]
		}
		if len(words) == 0 {
			return l
		}
		interpreter = filepath.Base(words[0])
	}
	if interpreter == "uv" && len(words) >= 3 && words[1] == "run" && (words[2] == "--script" || words[2] == "-s") {
		return LanguageByID("python")
	}
	if id := scriptInterpreters[interpreter]; id != "" {
		return LanguageByID(id)
	}
	if i := strings.IndexAny(interpreter, "0123456789"); i > 0 && versionedInterpreters[interpreter[:i]] {
		version := interpreter[i:]
		if version[len(version)-1] != '.' && strings.Trim(version, "0123456789.") == "" && !strings.Contains(version, "..") {
			return LanguageByID(scriptInterpreters[interpreter[:i]])
		}
	}
	return l
}

func LanguageByID(id string) LanguageInfo {
	if id == plainText.ID {
		return plainText
	}
	if info, ok := grammars.InfoForID(id); ok {
		return LanguageInfo{info.DisplayName, info.ID, info.ScopeName}
	}
	return LanguageInfo{}
}

func Languages() []LanguageInfo {
	languages := []LanguageInfo{plainText}
	for _, info := range grammars.Infos() {
		languages = append(languages, LanguageInfo{info.DisplayName, info.ID, info.ScopeName})
	}
	return languages
}
