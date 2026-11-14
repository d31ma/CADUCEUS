package main

import (
	"go/ast"
	"go/token"
)

type NumberReader struct {
	Positions *token.FileSet
	Files     []*ast.File
}

func (reader NumberReader) Numbers() []UsedNumber {
	functions := reader.functions()
	methods := reader.numberMethods(functions)
	fields := NumberFields{Fields: reader.numberFields(methods), Positions: StructFields{Files: reader.Files}.Positions(methods)}
	used := reader.literalResults(methods)
	used = append(used, reader.fieldValues(functions, fields)...)
	return append(used, reader.passedValues(reader.numberParameters(functions, fields))...)
}

func (reader NumberReader) functions() []*ast.FuncDecl {
	functions := []*ast.FuncDecl{}
	for _, file := range reader.Files {
		functions = append(functions, reader.fileFunctions(file)...)
	}
	return functions
}

func (reader NumberReader) fileFunctions(file *ast.File) []*ast.FuncDecl {
	functions := []*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		functions = append(functions, reader.function(declaration)...)
	}
	return functions
}

func (NumberReader) function(declaration ast.Decl) []*ast.FuncDecl {
	function, isFunction := declaration.(*ast.FuncDecl)
	if !isFunction || function.Body == nil {
		return nil
	}
	return []*ast.FuncDecl{function}
}

func (reader NumberReader) numberMethods(functions []*ast.FuncDecl) []NumberMethod {
	methods := []NumberMethod{}
	for _, function := range functions {
		methods = append(methods, reader.numberMethod(function)...)
	}
	return methods
}

func (NumberReader) numberMethod(function *ast.FuncDecl) []NumberMethod {
	method := NumberMethod{Declaration: function}
	if !method.Is() {
		return nil
	}
	return []NumberMethod{method}
}

func (reader NumberReader) literalResults(methods []NumberMethod) []UsedNumber {
	used := []UsedNumber{}
	for _, method := range methods {
		used = append(used, reader.integer(method.Result())...)
	}
	return used
}

func (reader NumberReader) numberFields(methods []NumberMethod) map[string]bool {
	fields := map[string]bool{}
	for _, method := range methods {
		fields[method.Field()] = method.Field() != ""
	}
	return fields
}

func (reader NumberReader) fieldValues(functions []*ast.FuncDecl, fields NumberFields) []UsedNumber {
	used := []UsedNumber{}
	for _, function := range functions {
		fields.Function = function
		used = append(used, reader.integers(fields.Values())...)
	}
	return used
}

func (reader NumberReader) integers(values []ast.Expr) []UsedNumber {
	used := []UsedNumber{}
	for _, value := range values {
		used = append(used, reader.integer(value)...)
	}
	return used
}

func (reader NumberReader) numberParameters(functions []*ast.FuncDecl, fields NumberFields) []NumberParameter {
	parameters := []NumberParameter{}
	for _, function := range functions {
		fields.Function = function
		parameters = append(parameters, fields.Parameters()...)
	}
	return parameters
}

func (reader NumberReader) passedValues(parameters []NumberParameter) []UsedNumber {
	used := []UsedNumber{}
	for _, parameter := range parameters {
		used = append(used, reader.callValues(parameter)...)
	}
	return used
}

func (reader NumberReader) callValues(parameter NumberParameter) []UsedNumber {
	used := []UsedNumber{}
	for _, file := range reader.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			used = append(used, reader.integer(parameter.Argument(node))...)
			return true
		})
	}
	return used
}

func (reader NumberReader) integer(expression ast.Expr) []UsedNumber {
	value, isInteger := IntegerLiteral{Expression: expression}.Value()
	if !isInteger {
		return nil
	}
	position := reader.Positions.Position(expression.Pos())
	return []UsedNumber{{Number: value, File: position.Filename, Line: position.Line}}
}
