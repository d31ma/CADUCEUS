package main

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strings"
)

type OwnerScan struct {
	Root     string
	Owner    Owner
	Kit      Owner
	Standard types.Importer
}

func (scan OwnerScan) Numbers() ([]UsedNumber, error) {
	positions := token.NewFileSet()
	checked, err := scan.checked(positions)
	if err != nil {
		return nil, err
	}
	copies, err := NameList{}.Read(filepath.Join(scan.Root, scan.Owner.Folder, "packages", "kit.list"))
	return scan.kept(NumberReader{Positions: positions, Packages: checked}.Numbers(), copies), err
}

func (scan OwnerScan) checked(positions *token.FileSet) ([]CheckedPackage, error) {
	sources, err := scan.sources(positions)
	loader := PackageLoader{Positions: positions, Sources: sources, Standard: scan.Standard, Checked: map[string]*CheckedPackage{}}
	return loader.All(), err
}

func (scan OwnerScan) sources(positions *token.FileSet) (map[string][]*ast.File, error) {
	files, err := scan.goFiles()
	if err != nil {
		return nil, err
	}
	parsed, err := scan.parse(positions, files)
	if err != nil {
		return nil, err
	}
	return SourcePackages{Root: scan.Root, Owner: scan.Owner}.Group(parsed)
}

func (scan OwnerScan) goFiles() ([]string, error) {
	files, err := SourceFolder{Root: scan.Root, Folder: scan.Owner.Folder}.GoFiles()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return files, err
}

func (scan OwnerScan) parse(positions *token.FileSet, files []string) ([]SourceFile, error) {
	parsed := []SourceFile{}
	source := GoSource{Root: scan.Root, Positions: positions}
	for _, name := range files {
		file, err := source.Parse(name)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, file)
	}
	return parsed, nil
}

func (scan OwnerScan) kept(numbers []UsedNumber, copies map[string]bool) []UsedNumber {
	kept := []UsedNumber{}
	for _, used := range numbers {
		kept = append(kept, scan.keep(used, copies)...)
	}
	return kept
}

func (scan OwnerScan) keep(used UsedNumber, copies map[string]bool) []UsedNumber {
	switch {
	case scan.Owner.IsKit() && !scan.Kit.Holds(used.Number):
		return nil
	case !scan.Owner.IsKit() && scan.Kit.Holds(used.Number) && copies[scan.kitPackage(used.File)]:
		return nil
	}
	return []UsedNumber{used}
}

func (scan OwnerScan) kitPackage(file string) string {
	relative, err := filepath.Rel(filepath.Join(scan.Owner.Folder, "packages"), file)
	if err != nil {
		return ""
	}
	return strings.Split(filepath.ToSlash(relative), "/")[0]
}
