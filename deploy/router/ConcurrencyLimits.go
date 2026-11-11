package main

import "context"

type ConcurrencyLimits struct {
	Slots map[string]chan struct{}
}

func (ConcurrencyLimits) Of(functions []string, writers []string, prefix string, others int) ConcurrencyLimits {
	slots := map[string]chan struct{}{}
	for _, service := range functions {
		slots[prefix+service] = make(chan struct{}, others)
	}
	for _, service := range writers {
		slots[prefix+service] = make(chan struct{}, 1)
	}
	return ConcurrencyLimits{Slots: slots}
}

func (limits ConcurrencyLimits) Enter(context context.Context, function string) (func(), error) {
	slot := limits.Slots[function]
	return limits.take(context, slot)
}

func (ConcurrencyLimits) take(context context.Context, slot chan struct{}) (func(), error) {
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-context.Done():
		return nil, context.Err()
	}
}
