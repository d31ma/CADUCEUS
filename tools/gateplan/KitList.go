package main

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

type KitList struct {
	Path string
}

func (list KitList) Packages() ([]string, error) {
	content, failure := os.ReadFile(list.Path)
	return strings.Fields(string(content)), list.missingMeansEmpty(failure)
}

func (KitList) missingMeansEmpty(failure error) error {
	if errors.Is(failure, fs.ErrNotExist) {
		return nil
	}
	return failure
}
