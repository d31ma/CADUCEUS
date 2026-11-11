package main

import (
	"context"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
)

type FolderStore struct {
	Folders map[string]string
}

func (store FolderStore) Get(context context.Context, key string) (Reply, error) {
	prefix, rest, _ := strings.Cut(key, "/")
	body, err := store.load(store.Folders[prefix], rest)
	if err != nil {
		return Reply{}, err
	}
	header := http.Header{}
	header.Set("Content-Type", store.contentType(rest))
	header.Set("Cache-Control", "no-cache")
	return Reply{Status: http.StatusOK, Header: header, Body: body}, nil
}

func (store FolderStore) load(folder string, name string) ([]byte, error) {
	body, err := store.read(folder, name)
	return body, store.problem(err)
}

func (FolderStore) read(folder string, name string) ([]byte, error) {
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(name)
}

func (FolderStore) problem(err error) error {
	if err != nil && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) || strings.Contains(err.Error(), "is a directory")) {
		return Problems{}.NotFound()
	}
	return err
}

func (FolderStore) contentType(name string) string {
	kind := mime.TypeByExtension(path.Ext(name))
	if kind == "" {
		return "application/octet-stream"
	}
	return kind
}
