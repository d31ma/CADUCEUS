package main

import (
	"errors"
	"os"
	"path/filepath"
)

type AppFolderReader struct {
	Root string
}

func (reader AppFolderReader) Read() (AppFolders, error) {
	entries, failure := os.ReadDir(filepath.Join(reader.Root, "apps"))
	folders, listFailure := reader.withKitLists(reader.appNames(entries))
	return folders, errors.Join(failure, listFailure)
}

func (AppFolderReader) appNames(entries []os.DirEntry) []string {
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func (reader AppFolderReader) withKitLists(names []string) (AppFolders, error) {
	folders := AppFolders{}
	var failures error
	for _, name := range names {
		packages, failure := KitList{Path: filepath.Join(reader.Root, "apps", name, "server", "packages", "kit.list")}.Packages()
		folders = append(folders, AppFolder{Name: name, KitPackages: packages})
		failures = errors.Join(failures, failure)
	}
	return folders, failures
}
