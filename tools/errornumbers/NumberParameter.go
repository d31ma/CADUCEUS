package main

import "go/types"

type NumberParameter struct {
	Function *types.Func
	Index    int
}
