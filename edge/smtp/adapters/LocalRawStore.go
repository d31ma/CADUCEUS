package adapters

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type LocalRawStore struct {
	folder string
}

func NewLocalRawStore(folder string) LocalRawStore {
	return LocalRawStore{folder: folder}
}

func (store LocalRawStore) Put(_ context.Context, key string, content []byte) error {
	target, err := store.target(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), store.folderMode()); err != nil {
		return err
	}
	temporary, err := store.writeTemporary(filepath.Dir(target), content)
	if err != nil {
		return err
	}
	return store.publish(temporary, target)
}

func (store LocalRawStore) target(key string) (string, error) {
	if store.unsafe(key) {
		return "", InvalidKey{}
	}
	return filepath.Join(store.folder, filepath.FromSlash(key)), nil
}

func (LocalRawStore) unsafe(key string) bool {
	return key == "" || path.IsAbs(key) || filepath.IsAbs(key) || strings.Contains(key, "\\") || slices.Contains(strings.Split(key, "/"), "..")
}

func (store LocalRawStore) writeTemporary(folder string, content []byte) (string, error) {
	file, err := os.CreateTemp(folder, ".incoming-*")
	if err != nil {
		return "", err
	}
	return store.keepOrRemove(file.Name(), store.fill(file, content))
}

func (LocalRawStore) fill(file *os.File, content []byte) error {
	_, writeError := file.Write(content)
	syncError := file.Sync()
	closeError := file.Close()
	return errors.Join(writeError, syncError, closeError)
}

func (LocalRawStore) keepOrRemove(name string, err error) (string, error) {
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func (store LocalRawStore) publish(temporary string, target string) error {
	if _, err := store.keepOrRemove(temporary, os.Rename(temporary, target)); err != nil {
		return err
	}
	return store.syncFolder(filepath.Dir(target))
}

func (LocalRawStore) syncFolder(folder string) error {
	directory, err := os.Open(folder)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func (LocalRawStore) folderMode() os.FileMode {
	return 0o750
}
