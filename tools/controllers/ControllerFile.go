package main

import (
	"go/ast"
	"go/token"
)

type ControllerFile struct {
	positions *token.FileSet
	syntax    *ast.File
}

func (file ControllerFile) Problems() []Problem {
	fields := file.structFields()
	problems := []Problem{}
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		problems = append(problems, file.methodProblems(node, fields)...)
		return true
	})
	return problems
}

func (file ControllerFile) structFields() map[string]map[string]bool {
	fields := map[string]map[string]bool{}
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		file.addStruct(fields, node)
		return true
	})
	return fields
}

func (file ControllerFile) addStruct(fields map[string]map[string]bool, node ast.Node) {
	spec, isTypeSpec := node.(*ast.TypeSpec)
	if !isTypeSpec {
		return
	}
	fields[spec.Name.Name] = file.fieldNames(spec.Type)
}

func (ControllerFile) fieldNames(expression ast.Expr) map[string]bool {
	names := map[string]bool{}
	structure, isStruct := expression.(*ast.StructType)
	if !isStruct {
		return names
	}
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			names[name.Name] = true
		}
	}
	return names
}

func (file ControllerFile) methodProblems(node ast.Node, fields map[string]map[string]bool) []Problem {
	declaration, isFunction := node.(*ast.FuncDecl)
	if !isFunction || declaration.Recv == nil || !declaration.Name.IsExported() {
		return nil
	}
	return file.routeProblems(declaration, fields)
}

func (file ControllerFile) routeProblems(declaration *ast.FuncDecl, fields map[string]map[string]bool) []Problem {
	method := RouteMethod{Declaration: declaration}
	controllerFields, isController := fields[method.ReceiverType()]
	if !isController {
		return nil
	}
	return file.problemFor(method, controllerFields)
}

func (file ControllerFile) problemFor(method RouteMethod, fields map[string]bool) []Problem {
	reason := method.WithFields(fields).Reason()
	if reason == "" {
		return nil
	}
	position := file.positions.Position(method.Declaration.Pos())
	return []Problem{{File: position.Filename, Line: position.Line, Controller: method.ReceiverType(), Method: method.Declaration.Name.Name, Reason: reason}}
}
