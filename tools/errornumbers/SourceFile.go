package main

import "go/ast"

type SourceFile struct {
	Name      string
	Syntax    *ast.File
	Framework bool
}
