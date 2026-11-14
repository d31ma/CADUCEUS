package main

import "go/ast"

type TypeName struct {
	Expression ast.Expr
}

func (name TypeName) String() string {
	switch expression := name.Expression.(type) {
	case *ast.StarExpr:
		return TypeName{Expression: expression.X}.String()
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	}
	return ""
}
