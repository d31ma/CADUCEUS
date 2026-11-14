package main

import (
	"go/ast"
	"go/token"
	"strconv"
)

type IntegerLiteral struct {
	Expression ast.Expr
}

func (literal IntegerLiteral) Value() (int, bool) {
	basic, isBasic := literal.Expression.(*ast.BasicLit)
	if !isBasic || basic.Kind != token.INT {
		return 0, false
	}
	value, err := strconv.ParseInt(basic.Value, 0, 0)
	return int(value), err == nil
}
