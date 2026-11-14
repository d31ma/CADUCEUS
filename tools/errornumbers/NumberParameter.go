package main

import "go/ast"

type NumberParameter struct {
	Function string
	Index    int
}

func (parameter NumberParameter) Argument(node ast.Node) ast.Expr {
	call, isCall := node.(*ast.CallExpr)
	if !isCall || (TypeName{Expression: call.Fun}).String() != parameter.Function || len(call.Args) <= parameter.Index {
		return nil
	}
	return call.Args[parameter.Index]
}
