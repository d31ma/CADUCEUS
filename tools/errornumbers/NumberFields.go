package main

import (
	"go/ast"
	"go/types"
)

type NumberFields struct {
	Fields map[*types.Var]bool
}

func (fields NumberFields) Values(function CheckedFunction) []ast.Expr {
	values := []ast.Expr{}
	ast.Inspect(function.Declaration.Body, func(node ast.Node) bool {
		values = append(values, fields.literalValues(function.Info, node)...)
		return true
	})
	return values
}

func (fields NumberFields) AddCopied(function CheckedFunction) bool {
	added := false
	for _, value := range fields.Values(function) {
		added = fields.add(SelectedField{Info: function.Info}.Of(value)) || added
	}
	return added
}

func (fields NumberFields) AddAll(found []*types.Var) {
	for _, field := range found {
		fields.add(field)
	}
}

func (fields NumberFields) add(field *types.Var) bool {
	if field == nil || fields.Fields[field] {
		return false
	}
	fields.Fields[field] = true
	return true
}

func (fields NumberFields) literalValues(info *types.Info, node ast.Node) []ast.Expr {
	literal, isLiteral := node.(*ast.CompositeLit)
	if !isLiteral {
		return nil
	}
	return fields.structValues(literal.Elts, StructType{Type: info.TypeOf(literal)}.Struct())
}

func (fields NumberFields) structValues(elements []ast.Expr, structure *types.Struct) []ast.Expr {
	switch {
	case structure == nil || len(elements) == 0:
		return nil
	case fields.isKeyed(elements[0]):
		return fields.keyedValues(elements, StructType{}.FieldsByName(structure))
	}
	return fields.positionalValues(elements, structure)
}

func (NumberFields) isKeyed(element ast.Expr) bool {
	_, isPair := element.(*ast.KeyValueExpr)
	return isPair
}

func (fields NumberFields) positionalValues(elements []ast.Expr, structure *types.Struct) []ast.Expr {
	values := []ast.Expr{}
	for index := range min(len(elements), structure.NumFields()) {
		values = append(values, fields.value(structure.Field(index), elements[index])...)
	}
	return values
}

func (fields NumberFields) keyedValues(elements []ast.Expr, byName map[string]*types.Var) []ast.Expr {
	values := []ast.Expr{}
	for _, element := range elements {
		values = append(values, fields.keyedValue(element, byName)...)
	}
	return values
}

func (fields NumberFields) keyedValue(element ast.Expr, byName map[string]*types.Var) []ast.Expr {
	pair, isPair := element.(*ast.KeyValueExpr)
	if !isPair {
		return nil
	}
	return fields.value(byName[FieldKey{Expression: pair.Key}.Name()], pair.Value)
}

func (fields NumberFields) value(field *types.Var, value ast.Expr) []ast.Expr {
	if field == nil || !fields.Fields[field.Origin()] {
		return nil
	}
	return []ast.Expr{value}
}
