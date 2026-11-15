package main

import (
	"go/ast"
	"go/types"
)

type CheckedPackage struct {
	Files []*ast.File
	Info  *types.Info
	Types *types.Package
}

func (CheckedPackage) NewInfo() *types.Info {
	return &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
}
