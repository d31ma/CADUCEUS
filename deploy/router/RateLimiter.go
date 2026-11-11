package main

import (
	"sync"
	"time"
)

type RateLimiter struct {
	Limit   int
	Window  time.Duration
	Now     func() time.Time
	mutex   *sync.Mutex
	windows map[string]RateWindow
}

func (RateLimiter) New(limit int, window time.Duration, now func() time.Time) RateLimiter {
	return RateLimiter{Limit: limit, Window: window, Now: now, mutex: &sync.Mutex{}, windows: map[string]RateWindow{}}
}

func (limiter RateLimiter) Allow(key string) bool {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	now := limiter.Now()
	limiter.forgetExpired(now)
	current := limiter.current(limiter.windows[key], now).Counted()
	limiter.windows[key] = current
	return current.Count <= limiter.Limit
}

func (limiter RateLimiter) current(window RateWindow, now time.Time) RateWindow {
	if now.Sub(window.Started) >= limiter.Window {
		return RateWindow{Started: now}
	}
	return window
}

func (limiter RateLimiter) forgetExpired(now time.Time) {
	for key, window := range limiter.windows {
		limiter.forgetWhenExpired(key, window, now)
	}
}

func (limiter RateLimiter) forgetWhenExpired(key string, window RateWindow, now time.Time) {
	if now.Sub(window.Started) >= limiter.Window {
		delete(limiter.windows, key)
	}
}
