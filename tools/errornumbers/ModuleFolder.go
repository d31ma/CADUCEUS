package main

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

type ModuleFolder struct {
	Root string
	Top  string
}

func (folder ModuleFolder) ImportPath(directory string) (string, error) {
	base := folder.base(directory)
	module, err := folder.modulePath(base)
	return folder.joined(module, base, directory), err
}

func (folder ModuleFolder) base(directory string) string {
	for current := directory; folder.inside(current); current = filepath.Dir(current) {
		if folder.hasModuleFile(current) {
			return current
		}
	}
	return ""
}

func (folder ModuleFolder) inside(directory string) bool {
	return directory == folder.Top || strings.HasPrefix(directory, folder.Top+string(filepath.Separator))
}

func (folder ModuleFolder) hasModuleFile(directory string) bool {
	_, err := os.Stat(filepath.Join(folder.Root, directory, "go.mod"))
	return err == nil
}

func (folder ModuleFolder) modulePath(base string) (string, error) {
	if base == "" {
		return "", nil
	}
	text, err := os.ReadFile(filepath.Join(folder.Root, base, "go.mod"))
	return ModuleFile{Text: string(text)}.Path(), err
}

func (ModuleFolder) joined(module string, base string, directory string) string {
	if module == "" {
		return filepath.ToSlash(directory)
	}
	relative, _ := filepath.Rel(base, directory)
	return path.Join(module, filepath.ToSlash(relative))
}
