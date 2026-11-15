package main

import (
	"go/ast"
	"go/types"
)

type CheckedFunction struct {
	Declaration *ast.FuncDecl
	Info        *types.Info
}

func (function CheckedFunction) Object() *types.Func {
	object, _ := function.Info.Defs[function.Declaration.Name].(*types.Func)
	return object
}

func (function CheckedFunction) Parameters(values []ast.Expr) []NumberParameter {
	parameters := []NumberParameter{}
	for _, value := range values {
		parameters = append(parameters, function.parameter(value)...)
	}
	return parameters
}

func (function CheckedFunction) parameter(value ast.Expr) []NumberParameter {
	name, isName := value.(*ast.Ident)
	if !isName {
		return nil
	}
	return function.parameterAt(function.Info.Uses[name])
}

func (function CheckedFunction) parameterAt(object types.Object) []NumberParameter {
	parameters := function.Object().Signature().Params()
	for index := range parameters.Len() {
		if parameters.At(index) == object {
			return []NumberParameter{{Function: function.Object(), Index: index}}
		}
	}
	return nil
}
