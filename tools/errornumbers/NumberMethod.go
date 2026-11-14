package main

import "go/ast"

type NumberMethod struct {
	Declaration *ast.FuncDecl
}

func (method NumberMethod) Is() bool {
	declaration := method.Declaration
	return declaration.Recv != nil && declaration.Name.Name == "Number" && declaration.Body != nil && method.returnsOneValue(declaration.Body.List)
}

func (method NumberMethod) Result() ast.Expr {
	return method.Declaration.Body.List[0].(*ast.ReturnStmt).Results[0]
}

func (method NumberMethod) ReceiverType() string {
	return TypeName{Expression: method.Declaration.Recv.List[0].Type}.String()
}

func (method NumberMethod) Field() string {
	selector, isSelector := method.Result().(*ast.SelectorExpr)
	if !isSelector {
		return ""
	}
	return selector.Sel.Name
}

func (NumberMethod) returnsOneValue(statements []ast.Stmt) bool {
	if len(statements) != 1 {
		return false
	}
	result, isReturn := statements[0].(*ast.ReturnStmt)
	return isReturn && len(result.Results) == 1
}
