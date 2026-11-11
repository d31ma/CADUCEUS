package main

import "context"

type QueuedEntry struct {
}

func (QueuedEntry) Take(caller context.Context, slot chan struct{}) (func(), error) {
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-caller.Done():
		return nil, caller.Err()
	}
}
