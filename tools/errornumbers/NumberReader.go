package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

type NumberReader struct {
	Positions *token.FileSet
	Packages  []CheckedPackage
}

func (reader NumberReader) Numbers() []UsedNumber {
	functions := reader.functions()
	methods := reader.numberMethods(functions)
	fields := reader.numberFields(functions, methods)
	used := reader.literalResults(methods)
	used = append(used, reader.fieldValues(functions, fields)...)
	used = append(used, reader.passedValues(reader.numberParameters(functions, fields))...)
	return reader.sorted(used)
}

func (reader NumberReader) functions() []CheckedFunction {
	functions := []CheckedFunction{}
	for _, checked := range reader.Packages {
		functions = append(functions, reader.packageFunctions(checked)...)
	}
	return functions
}

func (reader NumberReader) packageFunctions(checked CheckedPackage) []CheckedFunction {
	functions := []CheckedFunction{}
	for _, file := range checked.Files {
		functions = append(functions, reader.fileFunctions(file, checked.Info)...)
	}
	return functions
}

func (reader NumberReader) fileFunctions(file *ast.File, info *types.Info) []CheckedFunction {
	functions := []CheckedFunction{}
	for _, declaration := range file.Decls {
		functions = append(functions, reader.function(declaration, info)...)
	}
	return functions
}

func (NumberReader) function(declaration ast.Decl, info *types.Info) []CheckedFunction {
	function, isFunction := declaration.(*ast.FuncDecl)
	if !isFunction || function.Body == nil || info.Defs[function.Name] == nil {
		return nil
	}
	return []CheckedFunction{{Declaration: function, Info: info}}
}

func (reader NumberReader) numberMethods(functions []CheckedFunction) []NumberMethod {
	methods := []NumberMethod{}
	for _, function := range functions {
		methods = append(methods, reader.numberMethod(function)...)
	}
	return methods
}

func (NumberReader) numberMethod(function CheckedFunction) []NumberMethod {
	method := NumberMethod{Function: function}
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

func (reader NumberReader) numberFields(functions []CheckedFunction, methods []NumberMethod) NumberFields {
	fields := NumberFields{Fields: reader.returnedFields(methods)}
	for reader.addCopiedFields(functions, fields) {
	}
	return fields
}

func (NumberReader) returnedFields(methods []NumberMethod) map[*types.Var]bool {
	fields := map[*types.Var]bool{}
	for _, method := range methods {
		NumberFields{Fields: fields}.AddAll(method.Fields())
	}
	return fields
}

func (NumberReader) addCopiedFields(functions []CheckedFunction, fields NumberFields) bool {
	added := false
	for _, function := range functions {
		added = fields.AddCopied(function) || added
	}
	return added
}

func (reader NumberReader) fieldValues(functions []CheckedFunction, fields NumberFields) []UsedNumber {
	used := []UsedNumber{}
	for _, function := range functions {
		used = append(used, reader.integers(fields.Values(function))...)
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

func (reader NumberReader) numberParameters(functions []CheckedFunction, fields NumberFields) map[NumberParameter]bool {
	parameters := map[NumberParameter]bool{}
	for _, function := range functions {
		reader.addParameters(parameters, function.Parameters(fields.Values(function)))
	}
	return parameters
}

func (NumberReader) addParameters(parameters map[NumberParameter]bool, found []NumberParameter) {
	for _, parameter := range found {
		parameters[parameter] = true
	}
}

func (reader NumberReader) passedValues(parameters map[NumberParameter]bool) []UsedNumber {
	used := []UsedNumber{}
	for _, checked := range reader.Packages {
		used = append(used, reader.packageCalls(checked, parameters)...)
	}
	return used
}

func (reader NumberReader) packageCalls(checked CheckedPackage, parameters map[NumberParameter]bool) []UsedNumber {
	used := []UsedNumber{}
	for _, file := range checked.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			used = append(used, reader.callValues(node, CalledFunction{Info: checked.Info}, parameters)...)
			return true
		})
	}
	return used
}

func (reader NumberReader) callValues(node ast.Node, called CalledFunction, parameters map[NumberParameter]bool) []UsedNumber {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return nil
	}
	return reader.arguments(call, called.Of(call), parameters)
}

func (reader NumberReader) arguments(call *ast.CallExpr, function *types.Func, parameters map[NumberParameter]bool) []UsedNumber {
	used := []UsedNumber{}
	for index, argument := range call.Args {
		used = append(used, reader.passed(parameters[NumberParameter{Function: function, Index: index}], argument)...)
	}
	return used
}

func (reader NumberReader) passed(isNumber bool, argument ast.Expr) []UsedNumber {
	if !isNumber {
		return nil
	}
	return reader.integer(argument)
}

func (reader NumberReader) integer(expression ast.Expr) []UsedNumber {
	value, isInteger := IntegerLiteral{Expression: expression}.Value()
	if !isInteger {
		return nil
	}
	position := reader.Positions.Position(expression.Pos())
	return []UsedNumber{{Number: value, File: position.Filename, Line: position.Line}}
}

func (NumberReader) sorted(used []UsedNumber) []UsedNumber {
	sort.SliceStable(used, func(first int, second int) bool {
		return UsedNumber{}.Before(used[first], used[second])
	})
	return used
}
