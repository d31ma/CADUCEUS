package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

type GoSource struct {
	Root      string
	Positions *token.FileSet
}

func (source GoSource) Parse(name string) (SourceFile, error) {
	text, err := os.ReadFile(filepath.Join(source.Root, name))
	if err != nil {
		return SourceFile{}, err
	}
	framework := !source.hasPackageClause(name, text)
	syntax, err := parser.ParseFile(source.Positions, name, source.withPackageClause(name, text, framework), 0)
	return SourceFile{Name: name, Syntax: syntax, Framework: framework}, err
}

func (GoSource) hasPackageClause(name string, text []byte) bool {
	_, err := parser.ParseFile(token.NewFileSet(), name, text, parser.PackageClauseOnly)
	return err == nil
}

func (GoSource) withPackageClause(name string, text []byte, framework bool) []byte {
	if !framework {
		return text
	}
	header := "package tachyon\n//line " + filepath.Base(name) + ":1\n"
	return append([]byte(header), text...)
}
