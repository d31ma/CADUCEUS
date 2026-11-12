package main

import (
	"cmp"
	"fmt"
	"go/ast"
)

type RouteMethod struct {
	Declaration *ast.FuncDecl
	fields      map[string]bool
}

func (method RouteMethod) WithFields(fields map[string]bool) RouteMethod {
	method.fields = fields
	return method
}

func (method RouteMethod) ReceiverType() string {
	receiver := method.Declaration.Recv.List[0].Type
	pointer, isPointer := receiver.(*ast.StarExpr)
	if isPointer {
		receiver = pointer.X
	}
	return method.identifierName(receiver)
}

func (method RouteMethod) Reason() string {
	call, reason := method.onlyCall()
	return cmp.Or(reason, method.targetReason(call), method.argumentReason(call))
}

func (method RouteMethod) onlyCall() (*ast.CallExpr, string) {
	statements := method.Declaration.Body.List
	if len(statements) != 1 {
		return nil, fmt.Sprintf("has %d statements; a route method has exactly one", len(statements))
	}
	return method.statementCall(statements[0])
}

func (method RouteMethod) statementCall(statement ast.Stmt) (*ast.CallExpr, string) {
	switch statement := statement.(type) {
	case *ast.ReturnStmt:
		return method.returnedCall(statement)
	case *ast.ExprStmt:
		return method.bareCall(statement)
	}
	return nil, "is not a return statement"
}

func (RouteMethod) returnedCall(statement *ast.ReturnStmt) (*ast.CallExpr, string) {
	if len(statement.Results) != 1 {
		return nil, fmt.Sprintf("returns %d values; it must return one Service call", len(statement.Results))
	}
	call, isCall := statement.Results[0].(*ast.CallExpr)
	if !isCall {
		return nil, "returns something that is not a call; it must return one Service call"
	}
	return call, ""
}

func (method RouteMethod) bareCall(statement *ast.ExprStmt) (*ast.CallExpr, string) {
	call, isCall := statement.X.(*ast.CallExpr)
	if !isCall || method.Declaration.Type.Results.NumFields() > 0 {
		return nil, "is not a return statement"
	}
	return call, ""
}

func (method RouteMethod) targetReason(call *ast.CallExpr) string {
	if call == nil || method.fields[method.serviceField(call)] {
		return ""
	}
	return "does not call a method on one of the controller's own fields (its Services)"
}

func (method RouteMethod) serviceField(call *ast.CallExpr) string {
	outer, isOuterSelector := call.Fun.(*ast.SelectorExpr)
	if !isOuterSelector {
		return ""
	}
	return method.fieldOnReceiver(outer.X)
}

func (method RouteMethod) fieldOnReceiver(expression ast.Expr) string {
	inner, isInnerSelector := expression.(*ast.SelectorExpr)
	if !isInnerSelector || method.identifierName(inner.X) != method.receiverName() {
		return ""
	}
	return inner.Sel.Name
}

func (method RouteMethod) receiverName() string {
	names := method.Declaration.Recv.List[0].Names
	if len(names) == 0 {
		return ""
	}
	return names[0].Name
}

func (RouteMethod) identifierName(expression ast.Expr) string {
	identifier, isIdentifier := expression.(*ast.Ident)
	if !isIdentifier {
		return ""
	}
	return identifier.Name
}

func (method RouteMethod) argumentReason(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	return method.firstForeignArgument(call.Args, method.parameterNames())
}

func (method RouteMethod) firstForeignArgument(arguments []ast.Expr, parameters map[string]bool) string {
	for index, argument := range arguments {
		if !parameters[method.identifierName(argument)] {
			return fmt.Sprintf("passes argument %d that is not one of the method's own parameters", index+1)
		}
	}
	return ""
}

func (method RouteMethod) parameterNames() map[string]bool {
	names := map[string]bool{}
	for _, parameter := range method.Declaration.Type.Params.List {
		for _, name := range parameter.Names {
			names[name.Name] = true
		}
	}
	return names
}
