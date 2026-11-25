package jsonc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Set writes a JSON value at an object path, preserving the surrounding source.
// Intermediate objects are created as needed. The last matching key wins.
func Set(source []byte, path []string, value []byte) ([]byte, error) {
	clean, trailing, err := normalize(source)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 || !json.Valid(clean) || !json.Valid(value) {
		return nil, errors.New("invalid JSONC edit")
	}
	start := skipSpace(clean, 0)
	end := len(bytes.TrimRight(clean, " \t\r\n"))
	for i, key := range path {
		if clean[start] != '{' {
			return splice(source, start, end, objectValue(path[i:], value)), nil
		}
		obj, err := findMember(clean[start:end], key)
		if err != nil {
			return nil, err
		}
		if obj.valueEnd == 0 {
			// A trailing comma already separates the new field from the last one.
			lastEnd := start + obj.lastEnd
			for _, comma := range trailing {
				if comma >= lastEnd && comma < end-1 {
					lastEnd = 0
					break
				}
			}
			if obj.lastEnd == 0 {
				lastEnd = 0
			}
			member := objectValue(path[i:], value)
			return insertMember(source, end-1, lastEnd, member[1:len(member)-1]), nil
		}
		end = start + obj.valueEnd
		start += obj.valueStart
	}
	return splice(source, start, end, value), nil
}

type objectMember struct {
	valueStart, valueEnd int
	lastEnd              int
}

func findMember(object []byte, key string) (objectMember, error) {
	decoder := json.NewDecoder(bytes.NewReader(object))
	if _, err := decoder.Token(); err != nil {
		return objectMember{}, err
	}
	var member objectMember
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return objectMember{}, err
		}
		colon := skipSpace(object, int(decoder.InputOffset()))
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return objectMember{}, err
		}
		member.lastEnd = int(decoder.InputOffset())
		if strings.EqualFold(name.(string), key) {
			member.valueStart = skipSpace(object, colon+1)
			member.valueEnd = member.lastEnd
		}
	}
	return member, nil
}

func objectValue(path []string, value []byte) []byte {
	var result []byte
	for _, key := range path {
		name, _ := json.Marshal(key)
		result = append(result, '{')
		result = append(result, name...)
		result = append(result, ':', ' ')
	}
	return append(append(result, value...), bytes.Repeat([]byte{'}'}, len(path))...)
}

func insertMember(source []byte, closing, comma int, member []byte) []byte {
	lineStart := bytes.LastIndexByte(source[:closing], '\n') + 1
	indent := source[lineStart:closing]
	var insertion []byte
	if len(bytes.Trim(indent, " \t")) == 0 {
		// Keep the closing line and any comments on preceding lines in place.
		newline := "\n"
		if lineStart >= 2 && source[lineStart-2] == '\r' {
			newline = "\r\n"
		}
		insertion = append(bytes.Clone(indent), ' ', ' ')
		insertion = append(insertion, member...)
		insertion = append(insertion, newline...)
		closing = lineStart
	} else {
		insertion = append([]byte{' '}, member...)
		insertion = append(insertion, ' ')
	}
	result := splice(source, closing, closing, insertion)
	if comma != 0 {
		result = splice(result, comma, comma, []byte{','})
	}
	return result
}

func splice(source []byte, start, end int, replacement []byte) []byte {
	result := make([]byte, 0, len(source)-(end-start)+len(replacement))
	result = append(result, source[:start]...)
	result = append(result, replacement...)
	return append(result, source[end:]...)
}
