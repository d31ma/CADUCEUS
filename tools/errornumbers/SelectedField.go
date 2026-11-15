package main

import (
	"go/ast"
	"go/types"
)

type SelectedField struct {
	Info *types.Info
}

func (selected SelectedField) Of(expression ast.Expr) *types.Var {
	selector, isSelector := ast.Unparen(expression).(*ast.SelectorExpr)
	if !isSelector {
		return nil
	}
	return selected.field(selected.Info.Uses[selector.Sel])
}

func (selected SelectedField) In(expression ast.Expr) []*types.Var {
	fields := []*types.Var{}
	ast.Inspect(expression, func(node ast.Node) bool {
		fields = append(fields, selected.selectorField(node)...)
		_, isSelector := node.(*ast.SelectorExpr)
		return !isSelector
	})
	return fields
}

func (selected SelectedField) selectorField(node ast.Node) []*types.Var {
	selector, isSelector := node.(*ast.SelectorExpr)
	if !isSelector || selected.Of(selector) == nil {
		return nil
	}
	return []*types.Var{selected.Of(selector)}
}

func (SelectedField) field(object types.Object) *types.Var {
	field, isVariable := object.(*types.Var)
	if !isVariable || !field.IsField() {
		return nil
	}
	return field.Origin()
}
