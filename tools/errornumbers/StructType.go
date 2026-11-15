package main

import "go/types"

type StructType struct {
	Type types.Type
}

func (structType StructType) Struct() *types.Struct {
	if structType.Type == nil {
		return nil
	}
	structure, _ := types.Unalias(structType.pointed(structType.Type)).Underlying().(*types.Struct)
	return structure
}

func (StructType) pointed(literalType types.Type) types.Type {
	pointer, isPointer := literalType.Underlying().(*types.Pointer)
	if !isPointer {
		return literalType
	}
	return pointer.Elem()
}

func (StructType) FieldsByName(structure *types.Struct) map[string]*types.Var {
	byName := map[string]*types.Var{}
	for index := range structure.NumFields() {
		byName[structure.Field(index).Name()] = structure.Field(index)
	}
	return byName
}
