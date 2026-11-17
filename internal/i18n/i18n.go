// Package i18n provides the workbench's display languages. English source text is
// the message key and fallback; user content never passes through the catalog.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

type Language string

const (
	English Language = "en"
	Chinese Language = "zh-CN"
)

//go:embed zh-CN.json
var chineseJSON []byte

var chinese = func() map[string]string {
	var messages map[string]string
	if err := json.Unmarshal(chineseJSON, &messages); err != nil {
		panic(err)
	}
	return messages
}()

// Parse accepts the supported language IDs; an empty preference uses English.
func Parse(value string) (Language, bool) {
	switch value {
	case "", string(English):
		return English, true
	case string(Chinese):
		return Chinese, true
	default:
		return English, false
	}
}

func Text(language Language, message string, args ...any) string {
	if language == Chinese {
		if translated, ok := chinese[message]; ok {
			message = translated
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	}
	return message
}
