package main

import (
	"go/ast"
	"go/types"
)

type CalledFunction struct {
	Info *types.Info
}

func (called CalledFunction) Of(call *ast.CallExpr) *types.Func {
	function, _ := called.Info.Uses[called.name(ast.Unparen(call.Fun))].(*types.Func)
	return called.origin(function)
}

func (CalledFunction) name(expression ast.Expr) *ast.Ident {
	switch callee := expression.(type) {
	case *ast.Ident:
		return callee
	case *ast.SelectorExpr:
		return callee.Sel
	}
	return nil
}

func (CalledFunction) origin(function *types.Func) *types.Func {
	if function == nil {
		return nil
	}
	return function.Origin()
}
