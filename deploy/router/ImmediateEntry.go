package main

import (
	"context"
	"time"
)

type ImmediateEntry struct {
}

func (entry ImmediateEntry) Take(caller context.Context, gate *FunctionGate, now func() time.Time) (func(), error) {
	return entry.taken(gate.TryEnter(), gate, now)
}

func (ImmediateEntry) taken(entered bool, gate *FunctionGate, now func() time.Time) (func(), error) {
	if !entered {
		return nil, Problems{}.FunctionBusy()
	}
	return gate.ReleaseWith(now), nil
}
