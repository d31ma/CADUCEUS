package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
)

type SourceFile struct {
	Name   string
	Source []byte
}

func (file SourceFile) Problems() ([]Problem, error) {
	positions := token.NewFileSet()
	syntax, err := parser.ParseFile(positions, file.Name, file.withPackageClause(), 0)
	if err != nil {
		return nil, err
	}
	return ControllerFile{positions: positions, syntax: syntax}.Problems(), nil
}

func (file SourceFile) withPackageClause() []byte {
	header := "package controllers\n//line " + filepath.Base(file.Name) + ":1\n"
	return append([]byte(header), file.Source...)
}
