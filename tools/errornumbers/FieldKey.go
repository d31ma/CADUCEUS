package main

import "go/ast"

type FieldKey struct {
	Expression ast.Expr
}

func (key FieldKey) Name() string {
	name, isName := key.Expression.(*ast.Ident)
	if !isName {
		return ""
	}
	return name.Name
}
