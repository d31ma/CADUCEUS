package main

import "time"

type RateWindow struct {
	Started time.Time
	Count   int
}

func (window RateWindow) Counted() RateWindow {
	return RateWindow{Started: window.Started, Count: window.Count + 1}
}
