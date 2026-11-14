package main

import (
	"io/fs"
	"path/filepath"
	"strings"
)

type SourceFolder struct {
	Root   string
	Folder string
}

func (folder SourceFolder) GoFiles() ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(filepath.Join(folder.Root, folder.Folder), func(path string, entry fs.DirEntry, err error) error {
		files = append(files, folder.goFile(path, entry)...)
		return folder.next(entry, err)
	})
	return files, err
}

func (folder SourceFolder) next(entry fs.DirEntry, err error) error {
	switch {
	case err != nil:
		return err
	case entry.IsDir() && folder.skipped(entry.Name()):
		return filepath.SkipDir
	}
	return nil
}

func (SourceFolder) skipped(name string) bool {
	return map[string]bool{"vendor": true, "testdata": true, "tests": true, "node_modules": true, ".tachyon": true}[name]
}

func (folder SourceFolder) goFile(path string, entry fs.DirEntry) []string {
	if entry == nil || entry.IsDir() || !folder.isSource(entry.Name()) {
		return nil
	}
	relative, _ := filepath.Rel(folder.Root, path)
	return []string{relative}
}

func (SourceFolder) isSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}
