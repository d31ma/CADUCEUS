package main

import (
	"context"
	"time"
)

type QueuedEntry struct {
	Background bool
}

func (entry QueuedEntry) Take(caller context.Context, gate *FunctionGate, now func() time.Time) (func(), error) {
	waiter := gate.Enter(entry.Background, now())
	return entry.wait(caller, gate, waiter, now)
}

func (QueuedEntry) wait(caller context.Context, gate *FunctionGate, waiter *GateWaiter, now func() time.Time) (func(), error) {
	select {
	case <-waiter.Ready:
		return gate.ReleaseWith(now), nil
	case <-caller.Done():
		gate.Leave(waiter, now())
		return nil, caller.Err()
	}
}
