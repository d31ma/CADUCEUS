package main

import "go/ast"

type NumberFields struct {
	Function  *ast.FuncDecl
	Fields    map[string]bool
	Positions map[string]int
}

func (fields NumberFields) Values() []ast.Expr {
	values := []ast.Expr{}
	ast.Inspect(fields.Function.Body, func(node ast.Node) bool {
		values = append(values, fields.literalValues(node)...)
		return true
	})
	return values
}

func (fields NumberFields) Parameters() []NumberParameter {
	parameters := []NumberParameter{}
	for _, value := range fields.Values() {
		parameters = append(parameters, fields.parameter(value)...)
	}
	return parameters
}

func (fields NumberFields) literalValues(node ast.Node) []ast.Expr {
	literal, isLiteral := node.(*ast.CompositeLit)
	switch {
	case !isLiteral || len(literal.Elts) == 0:
		return nil
	case fields.isKeyed(literal.Elts[0]):
		return fields.keyedValues(literal.Elts)
	}
	return fields.positionalValue(literal)
}

func (NumberFields) isKeyed(element ast.Expr) bool {
	_, isPair := element.(*ast.KeyValueExpr)
	return isPair
}

func (fields NumberFields) positionalValue(literal *ast.CompositeLit) []ast.Expr {
	position, isNumbered := fields.Positions[TypeName{Expression: literal.Type}.String()]
	if !isNumbered || position >= len(literal.Elts) {
		return nil
	}
	return []ast.Expr{literal.Elts[position]}
}

func (fields NumberFields) keyedValues(elements []ast.Expr) []ast.Expr {
	values := []ast.Expr{}
	for _, element := range elements {
		values = append(values, fields.keyedValue(element)...)
	}
	return values
}

func (fields NumberFields) keyedValue(element ast.Expr) []ast.Expr {
	pair, isPair := element.(*ast.KeyValueExpr)
	if !isPair || !fields.Fields[(TypeName{Expression: pair.Key}).String()] {
		return nil
	}
	return []ast.Expr{pair.Value}
}

func (fields NumberFields) parameter(value ast.Expr) []NumberParameter {
	name, isName := value.(*ast.Ident)
	if !isName {
		return nil
	}
	return fields.parameterAt(fields.parameterNames(), name.Name)
}

func (fields NumberFields) parameterAt(names []string, name string) []NumberParameter {
	for index, parameter := range names {
		if parameter == name {
			return []NumberParameter{{Function: fields.Function.Name.Name, Index: index}}
		}
	}
	return nil
}

func (fields NumberFields) parameterNames() []string {
	names := []string{}
	for _, field := range fields.Function.Type.Params.List {
		names = append(names, fields.Names(field)...)
	}
	return names
}

func (NumberFields) Names(field *ast.Field) []string {
	names := []string{}
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	return names
}
