package main

import "go/ast"

type StructFields struct {
	Files []*ast.File
}

func (structs StructFields) Positions(methods []NumberMethod) map[string]int {
	fields := structs.all()
	positions := map[string]int{}
	for _, method := range methods {
		structs.addPosition(positions, method, fields[method.ReceiverType()])
	}
	return positions
}

func (structs StructFields) all() map[string][]string {
	fields := map[string][]string{}
	for _, file := range structs.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			structs.addStruct(fields, node)
			return true
		})
	}
	return fields
}

func (structs StructFields) addStruct(fields map[string][]string, node ast.Node) {
	spec, isTypeSpec := node.(*ast.TypeSpec)
	if !isTypeSpec {
		return
	}
	fields[spec.Name.Name] = structs.names(spec.Type)
}

func (structs StructFields) names(expression ast.Expr) []string {
	names := []string{}
	structure, isStruct := expression.(*ast.StructType)
	if !isStruct {
		return names
	}
	for _, field := range structure.Fields.List {
		names = append(names, structs.fieldNames(field)...)
	}
	return names
}

func (StructFields) fieldNames(field *ast.Field) []string {
	if len(field.Names) == 0 {
		return []string{TypeName{Expression: field.Type}.String()}
	}
	return NumberFields{}.Names(field)
}

func (StructFields) addPosition(positions map[string]int, method NumberMethod, names []string) {
	for index, name := range names {
		if name == method.Field() {
			positions[method.ReceiverType()] = index
		}
	}
}
