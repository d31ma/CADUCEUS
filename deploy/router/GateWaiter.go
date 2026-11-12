package main

import "time"

type GateWaiter struct {
	Since  time.Time
	Ready  chan struct{}
	handed bool
}

func (GateWaiter) New(since time.Time) *GateWaiter {
	return &GateWaiter{Since: since, Ready: make(chan struct{})}
}

func (waiter *GateWaiter) Hand() {
	waiter.handed = true
	close(waiter.Ready)
}

func (waiter *GateWaiter) Held() int {
	if waiter.handed {
		return 1
	}
	return 0
}
