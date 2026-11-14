package oidc

import (
	"errors"
	"os"
	"path/filepath"
)

type AtomicFile struct{}

func (atomic AtomicFile) Write(path string, raw []byte) error {
	folder := filepath.Dir(path)
	err := os.MkdirAll(folder, atomic.folderMode())
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(folder, "write-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (AtomicFile) folderMode() os.FileMode {
	return 0o700
}
