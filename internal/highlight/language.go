package highlight

import (
	"path/filepath"
	"strings"

	"github.com/eugenioenko/textmate-go/grammars"
)

type LanguageInfo struct{ Name, ID string }

func Language(path string) LanguageInfo {
	name := strings.ToLower(filepath.Base(path))
	if name == "go.mod" || name == "go.sum" {
		return LanguageInfo{"Go Module", "gomod"}
	}
	if info, ok := grammars.InfoForFilename(path); ok {
		return LanguageInfo{info.DisplayName, info.ID}
	}
	return LanguageInfo{"Plain Text", "text"}
}
