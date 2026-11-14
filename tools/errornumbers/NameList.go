package main

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

type NameList struct{}

func (list NameList) Read(path string) (map[string]bool, error) {
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	return list.names(string(text)), err
}

func (NameList) names(text string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Fields(text) {
		names[line] = true
	}
	return names
}
