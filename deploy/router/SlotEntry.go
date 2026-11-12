package main

import (
	"context"
	"time"
)

type SlotEntry interface {
	Take(caller context.Context, gate *FunctionGate, now func() time.Time) (func(), error)
}
