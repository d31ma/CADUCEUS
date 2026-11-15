package main

import (
	"go/ast"
	"go/types"
)

type NumberMethod struct {
	Function CheckedFunction
}

func (method NumberMethod) Is() bool {
	declaration := method.Function.Declaration
	return declaration.Recv != nil && declaration.Name.Name == "Number" && method.returnsOneValue(declaration.Body.List) && method.returnsAnInteger()
}

func (method NumberMethod) Result() ast.Expr {
	return method.Function.Declaration.Body.List[0].(*ast.ReturnStmt).Results[0]
}

func (method NumberMethod) Fields() []*types.Var {
	return SelectedField{Info: method.Function.Info}.In(method.Result())
}

func (NumberMethod) returnsOneValue(statements []ast.Stmt) bool {
	if len(statements) != 1 {
		return false
	}
	result, isReturn := statements[0].(*ast.ReturnStmt)
	return isReturn && len(result.Results) == 1
}

func (method NumberMethod) returnsAnInteger() bool {
	signature := method.Function.Object().Signature()
	return signature.Params().Len() == 0 && signature.Results().Len() == 1 && method.isInteger(signature.Results().At(0).Type())
}

func (NumberMethod) isInteger(result types.Type) bool {
	basic, isBasic := result.Underlying().(*types.Basic)
	return isBasic && basic.Info()&types.IsInteger != 0
}
