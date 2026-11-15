package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

type PackageLoader struct {
	Positions *token.FileSet
	Sources   map[string][]*ast.File
	Standard  types.Importer
	Checked   map[string]*CheckedPackage
}

func (loader PackageLoader) All() []CheckedPackage {
	checked := []CheckedPackage{}
	for _, importPath := range loader.importPaths() {
		checked = append(checked, *loader.load(importPath))
	}
	return checked
}

func (loader PackageLoader) Import(importPath string) (*types.Package, error) {
	return loader.imported(importPath)
}

func (loader PackageLoader) imported(importPath string) (*types.Package, error) {
	switch {
	case loader.Sources[importPath] != nil:
		return loader.checkedTypes(importPath, loader.load(importPath))
	case loader.isStandard(importPath):
		return loader.Standard.Import(importPath)
	}
	return nil, fmt.Errorf("%s is neither in this folder nor in the standard library", importPath)
}

func (loader PackageLoader) importPaths() []string {
	importPaths := []string{}
	for importPath := range loader.Sources {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	return importPaths
}

func (PackageLoader) isStandard(importPath string) bool {
	return !strings.Contains(strings.Split(importPath, "/")[0], ".")
}

func (PackageLoader) checkedTypes(importPath string, checked *CheckedPackage) (*types.Package, error) {
	if checked.Types == nil {
		return nil, fmt.Errorf("%s imports itself", importPath)
	}
	return checked.Types, nil
}

func (loader PackageLoader) load(importPath string) *CheckedPackage {
	if checked, isChecked := loader.Checked[importPath]; isChecked {
		return checked
	}
	return loader.check(importPath)
}

func (loader PackageLoader) check(importPath string) *CheckedPackage {
	checked := &CheckedPackage{Files: loader.Sources[importPath], Info: CheckedPackage{}.NewInfo()}
	loader.Checked[importPath] = checked
	config := types.Config{Importer: loader, Error: func(error) {}}
	checked.Types, _ = config.Check(importPath, loader.Positions, checked.Files, checked.Info)
	return checked
}
