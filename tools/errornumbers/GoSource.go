package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

type GoSource struct {
	Root      string
	Positions *token.FileSet
}

func (source GoSource) Parse(name string) (*ast.File, error) {
	text, err := os.ReadFile(filepath.Join(source.Root, name))
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(source.Positions, name, source.withPackageClause(name, text), 0)
}

func (GoSource) withPackageClause(name string, text []byte) []byte {
	if _, err := parser.ParseFile(token.NewFileSet(), name, text, parser.PackageClauseOnly); err == nil {
		return text
	}
	header := "package tachyon\n//line " + filepath.Base(name) + ":1\n"
	return append([]byte(header), text...)
}
