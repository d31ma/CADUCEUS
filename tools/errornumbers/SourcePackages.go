package main

import (
	"go/ast"
	"path/filepath"
)

type SourcePackages struct {
	Root  string
	Owner Owner
}

func (packages SourcePackages) Group(files []SourceFile) (map[string][]*ast.File, error) {
	folders := packages.folders(files)
	grouped := map[string][]*ast.File{}
	for directory, syntax := range folders {
		importPath, err := packages.importPath(directory)
		if err != nil {
			return nil, err
		}
		grouped[importPath] = append(grouped[importPath], syntax...)
	}
	return grouped, nil
}

func (packages SourcePackages) folders(files []SourceFile) map[string][]*ast.File {
	folders := map[string][]*ast.File{}
	for _, file := range files {
		directory := packages.folder(file)
		folders[directory] = append(folders[directory], file.Syntax)
	}
	return folders
}

func (packages SourcePackages) folder(file SourceFile) string {
	if file.Framework {
		return ""
	}
	return filepath.Dir(file.Name)
}

func (packages SourcePackages) importPath(directory string) (string, error) {
	if directory == "" {
		return packages.FrameworkPath(), nil
	}
	return ModuleFolder{Root: packages.Root, Top: packages.Owner.Folder}.ImportPath(directory)
}

func (packages SourcePackages) FrameworkPath() string {
	return "tachyon/" + filepath.ToSlash(packages.Owner.Folder)
}
