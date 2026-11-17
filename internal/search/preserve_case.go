package search

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Keep casing independently for matching snake/kebab segments, otherwise use
// all-upper, all-lower or the first cased character as the replacement's style.
func preserveCase(original, replacement string) string {
	if original == "" || replacement == "" {
		return replacement
	}
	delimiter := ""
	for _, sep := range []string{"-", "_"} {
		if strings.Contains(original, sep) && strings.Contains(replacement, sep) && strings.Count(original, sep) == strings.Count(replacement, sep) {
			if delimiter != "" {
				delimiter = ""
				break
			}
			delimiter = sep
		}
	}
	if delimiter != "" {
		source, target := strings.Split(original, delimiter), strings.Split(replacement, delimiter)
		for i := range target {
			target[i] = preserveCase(source[i], target[i])
		}
		return strings.Join(target, delimiter)
	}
	upper, lower := false, false
	for _, r := range original {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
	}
	if upper && !lower {
		return strings.ToUpper(replacement)
	}
	if lower && !upper {
		return strings.ToLower(replacement)
	}
	first, _ := utf8.DecodeRuneInString(original)
	next, size := utf8.DecodeRuneInString(replacement)
	if unicode.IsUpper(first) {
		return string(unicode.ToUpper(next)) + replacement[size:]
	}
	if unicode.IsLower(first) {
		return string(unicode.ToLower(next)) + replacement[size:]
	}
	return replacement
}

// Disambiguate $1Name as capture 1 followed by Name. Go's expander otherwise
// treats the whole identifier as a named capture. A doubled dollar stays literal.
func normalizeReplacement(value string, groups int) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '$' || i+1 == len(value) {
			b.WriteByte(value[i])
			continue
		}
		if value[i+1] == '$' {
			b.WriteString("$$")
			i++
			continue
		}
		if value[i+1] == '&' {
			b.WriteString("${0}")
			i++
			continue
		}
		if value[i+1] < '0' || value[i+1] > '9' {
			b.WriteByte('$')
			continue
		}
		end := i + 1
		for end < len(value) && value[end] >= '0' && value[end] <= '9' {
			end++
		}
		chosen := end
		for chosen > i+1 {
			n, err := strconv.Atoi(value[i+1 : chosen])
			if err == nil && n <= groups {
				break
			}
			chosen--
		}
		if chosen == i+1 {
			b.WriteString("$$")
			continue
		}
		b.WriteString("${" + value[i+1:chosen] + "}")
		i = chosen - 1
	}
	return b.String()
}
