package main

import (
	"slices"
	"sync"
	"time"
)

type FunctionGate struct {
	mutex      *sync.Mutex
	capacity   int
	busy       int
	users      []*GateWaiter
	background []*GateWaiter
	promoted   bool
}

func (FunctionGate) New(capacity int) *FunctionGate {
	return &FunctionGate{mutex: &sync.Mutex{}, capacity: capacity}
}

func (FunctionGate) patience() time.Duration {
	return 60 * time.Second
}

func (gate *FunctionGate) Enter(background bool, now time.Time) *GateWaiter {
	gate.mutex.Lock()
	defer gate.mutex.Unlock()
	waiter := GateWaiter{}.New(now)
	gate.queue(waiter, background)
	gate.handOut(now)
	return waiter
}

func (gate *FunctionGate) TryEnter() bool {
	gate.mutex.Lock()
	defer gate.mutex.Unlock()
	return gate.takeFreeSlot()
}

func (gate *FunctionGate) Leave(waiter *GateWaiter, now time.Time) {
	gate.mutex.Lock()
	defer gate.mutex.Unlock()
	gate.users = gate.without(gate.users, waiter)
	gate.background = gate.without(gate.background, waiter)
	gate.busy -= waiter.Held()
	gate.handOut(now)
}

func (gate *FunctionGate) Release(now time.Time) {
	gate.mutex.Lock()
	defer gate.mutex.Unlock()
	gate.busy--
	gate.handOut(now)
}

func (gate *FunctionGate) ReleaseWith(now func() time.Time) func() {
	return func() { gate.Release(now()) }
}

func (gate *FunctionGate) queue(waiter *GateWaiter, background bool) {
	if background {
		gate.background = append(gate.background, waiter)
		return
	}
	gate.users = append(gate.users, waiter)
}

func (gate *FunctionGate) takeFreeSlot() bool {
	if gate.busy >= gate.capacity {
		return false
	}
	gate.busy++
	return true
}

func (gate *FunctionGate) handOut(now time.Time) {
	if gate.busy >= gate.capacity {
		return
	}
	gate.handTo(gate.next(now))
}

func (gate *FunctionGate) handTo(waiter *GateWaiter) {
	if waiter == nil {
		return
	}
	gate.busy++
	waiter.Hand()
}

func (gate *FunctionGate) next(now time.Time) *GateWaiter {
	jump := gate.backgroundJumps(now)
	gate.promoted = jump
	return gate.pop(jump || len(gate.users) == 0)
}

func (gate *FunctionGate) backgroundJumps(now time.Time) bool {
	return !gate.promoted && len(gate.background) > 0 && now.Sub(gate.background[0].Since) > gate.patience()
}

func (gate *FunctionGate) pop(fromBackground bool) *GateWaiter {
	if fromBackground {
		return gate.popFrom(&gate.background)
	}
	return gate.popFrom(&gate.users)
}

func (FunctionGate) popFrom(line *[]*GateWaiter) *GateWaiter {
	if len(*line) == 0 {
		return nil
	}
	waiter := (*line)[0]
	*line = (*line)[1:]
	return waiter
}

func (FunctionGate) without(line []*GateWaiter, waiter *GateWaiter) []*GateWaiter {
	return slices.DeleteFunc(line, func(other *GateWaiter) bool { return other == waiter })
}
