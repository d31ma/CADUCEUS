package main

import "context"

type ImmediateEntry struct {
}

func (ImmediateEntry) Take(caller context.Context, slot chan struct{}) (func(), error) {
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	default:
		return nil, Problems{}.FunctionBusy()
	}
}
