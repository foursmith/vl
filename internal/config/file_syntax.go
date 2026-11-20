package config

import (
	"errors"
	"os"
)

func FileSyntaxPath(path string) (string, error) {
	dir, err := statePath("syntax")
	if err != nil {
		return "", err
	}
	return fileStatePath(dir, path)
}

func LoadFileSyntax(record string) (string, error) {
	var id string
	err := loadJSON(record, &id)
	return id, err
}

func SaveFileSyntax(record, id string) error {
	if id != "" {
		return saveJSON(record, id)
	}
	err := os.Remove(record)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
