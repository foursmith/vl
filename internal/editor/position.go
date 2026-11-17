package editor

import (
	"errors"
	"strconv"
	"strings"
)

// ParsePosition parses one-based line[:column] coordinates.
func ParsePosition(value string) (line, column int, err error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) > 2 {
		return 0, 0, errors.New("Enter a line number or line:column")
	}
	line, err = strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		return 0, 0, errors.New("Line number must be greater than 0")
	}
	column = 1
	if len(parts) == 2 {
		column, err = strconv.Atoi(parts[1])
		if err != nil || column < 1 {
			return 0, 0, errors.New("Column number must be greater than 0")
		}
	}
	return line, column, nil
}
