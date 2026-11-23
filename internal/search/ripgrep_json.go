package search

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func (m *Matcher) canUseRipgrepPositions() bool {
	o := m.options
	return !o.Regex && !o.Word && utf8.ValidString(o.Query) &&
		utf8.RuneCountInString(o.Query) <= 64 && !strings.ContainsAny(o.Query, "\r\n\x00\uFEFF")
}

type ripgrepBytes struct {
	Text  string `json:"text"`
	Bytes []byte `json:"bytes"`
}

func (v ripgrepBytes) string() string {
	if v.Bytes != nil {
		return string(v.Bytes)
	}
	return v.Text
}

type ripgrepSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ripgrepLine struct {
	Number int
	Text   string
	Spans  []ripgrepSpan
}

func (rg *ripgrepSearch) scanJSONWorkspace(includeIgnored bool, inScope func(string) bool, globs []string, visit func(string, []ripgrepLine) error) (map[string]struct{}, error) {
	args := rg.workspaceArgs(includeIgnored, globs, "--json")
	scanned := make(map[string]struct{})
	err := rg.stream(args, func(stdout io.Reader) error {
		return readRipgrepJSON(stdout, func(relative string) bool {
			_, duplicate := scanned[relative]
			return !duplicate && inScope(relative)
		}, func(relative string, lines []ripgrepLine) error {
			if err := visit(relative, lines); err != nil {
				return err
			}
			scanned[relative] = struct{}{}
			return nil
		})
	})
	if len(scanned) == 0 && ripgrepNoMatches(err) {
		err = nil
	}
	return scanned, err
}

func readRipgrepJSON(stdout io.Reader, inScope func(string) bool, visit func(string, []ripgrepLine) error) error {
	decoder := json.NewDecoder(stdout)
	var current string
	var lines []ripgrepLine
	var eligible, summary bool
	for {
		var event struct {
			Type string `json:"type"`
			Data struct {
				Path         ripgrepBytes  `json:"path"`
				Lines        ripgrepBytes  `json:"lines"`
				LineNumber   int           `json:"line_number"`
				Submatches   []ripgrepSpan `json:"submatches"`
				BinaryOffset *int64        `json:"binary_offset"`
			} `json:"data"`
		}
		err := decoder.Decode(&event)
		if err == io.EOF && summary {
			return nil
		}
		if err != nil {
			return err
		}
		path := strings.TrimPrefix(filepath.ToSlash(event.Data.Path.string()), "./")
		switch event.Type {
		case "begin":
			if current != "" || !filepath.IsLocal(path) || path == "." || filepath.ToSlash(filepath.Clean(path)) != path {
				return fmt.Errorf("unexpected ripgrep file start %q", path)
			}
			current, eligible = path, inScope(path)
		case "match":
			if current == "" || path != current {
				return fmt.Errorf("unexpected ripgrep match path %q", path)
			}
			if eligible {
				lines = append(lines, ripgrepLine{event.Data.LineNumber, event.Data.Lines.string(), event.Data.Submatches})
			}
		case "end":
			if current == "" || path != current {
				return fmt.Errorf("unexpected ripgrep file end %q", path)
			}
			if eligible && event.Data.BinaryOffset == nil && len(lines) != 0 {
				if err := visit(path, lines); err != nil {
					return err
				}
			}
			current, lines = "", nil
		case "summary":
			if current != "" {
				return fmt.Errorf("ripgrep omitted the end of %q", current)
			}
			summary = true
		default:
			return fmt.Errorf("unexpected ripgrep event %q", event.Type)
		}
	}
}
