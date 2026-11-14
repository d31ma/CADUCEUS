package main

import (
	"errors"
	"io/fs"
	"os"
)

type SettingFile struct {
	Name string
}

func (file SettingFile) Read() ([]byte, bool, error) {
	source, err := os.ReadFile(file.Name)
	return file.result(source, err)
}

func (SettingFile) result(source []byte, err error) ([]byte, bool, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return source, err == nil, err
}
