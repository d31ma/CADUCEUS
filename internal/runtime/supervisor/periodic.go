package supervisor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// WorkerSnapshot is the serializable metrics surface of a PeriodicWorker.
type WorkerSnapshot struct {
	Name            string   `json:"name"`
	Running         bool     `json:"running"`
	InFlight        bool     `json:"inFlight"`
	Runs            int      `json:"runs"`
	Succeeded       int      `json:"succeeded"`
	Failed          int      `json:"failed"`
	Skipped         int      `json:"skipped"`
	LastStartedAt   string   `json:"lastStartedAt,omitempty"`
	LastCompletedAt string   `json:"lastCompletedAt,omitempty"`
	LastDurationMs  *float64 `json:"lastDurationMs,omitempty"`
	LastError       string   `json:"lastError,omitempty"`
}

// PeriodicWorker runs one task on a fixed interval and never overlaps runs.
// It is the Go port of the JavaScript PeriodicWorker: the interval is a fixed
// ticker (setInterval), not a delay after each run, and a tick that lands while
// a run is in flight is counted as skipped.
type PeriodicWorker struct {
	name     string
	interval time.Duration
	task     func(context.Context) error
	now      func() time.Time

	mu              sync.Mutex
	ticker          *time.Ticker
	stopTicker      chan struct{}
	tickerDone      chan struct{}
	active          chan struct{}
	runs            int
	succeeded       int
	failed          int
	skipped         int
	lastStartedAt   string
	lastCompletedAt string
	lastDurationMs  *float64
	lastError       string
}

// NewPeriodicWorker validates and returns a stopped worker.
func NewPeriodicWorker(name string, interval time.Duration, task func(context.Context) error) (*PeriodicWorker, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("A periodic worker requires a name")
	}
	if interval <= 0 {
		return nil, errors.New("A periodic worker interval must be greater than zero")
	}
	return &PeriodicWorker{name: name, interval: interval, task: task, now: time.Now}, nil
}

// Start begins the interval and answers the function that stops it.
func (worker *PeriodicWorker) Start() (StopFunc, error) {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.ticker != nil {
		return nil, fmt.Errorf("Periodic worker %q is already running", worker.name)
	}
	worker.ticker = time.NewTicker(worker.interval)
	worker.stopTicker = make(chan struct{})
	worker.tickerDone = make(chan struct{})
	ticker, stop, done := worker.ticker, worker.stopTicker, worker.tickerDone
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				go worker.RunNow()
			}
		}
	}()
	return func(ctx context.Context) error { return worker.Stop(ctx) }, nil
}

// Stop ends the interval and waits for an in-flight run to finish.
func (worker *PeriodicWorker) Stop(ctx context.Context) error {
	worker.mu.Lock()
	if worker.ticker != nil {
		worker.ticker.Stop()
		close(worker.stopTicker)
		<-worker.tickerDone
		worker.ticker = nil
	}
	active := worker.active
	worker.mu.Unlock()
	if active == nil {
		return nil
	}
	select {
	case <-active:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunNow runs the task once unless a run is already in flight, in which case
// it counts a skip and answers false. It answers whether the run succeeded.
func (worker *PeriodicWorker) RunNow() bool {
	worker.mu.Lock()
	if worker.active != nil {
		worker.skipped++
		worker.mu.Unlock()
		return false
	}
	done := make(chan struct{})
	worker.active = done
	worker.runs++
	worker.lastStartedAt = Timestamp(worker.now())
	worker.mu.Unlock()

	started := time.Now()
	err := worker.execute()
	elapsed := float64(time.Since(started)) / float64(time.Millisecond)
	duration := math.Round(elapsed*100) / 100

	worker.mu.Lock()
	if err != nil {
		worker.failed++
		worker.lastError = err.Error()
	} else {
		worker.succeeded++
		worker.lastError = ""
	}
	worker.lastCompletedAt = Timestamp(worker.now())
	worker.lastDurationMs = &duration
	worker.active = nil
	worker.mu.Unlock()
	close(done)
	return err == nil
}

func (worker *PeriodicWorker) execute() (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return worker.task(context.Background())
}

// Snapshot answers the worker metrics.
func (worker *PeriodicWorker) Snapshot() WorkerSnapshot {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	snapshot := WorkerSnapshot{
		Name:            worker.name,
		Running:         worker.ticker != nil,
		InFlight:        worker.active != nil,
		Runs:            worker.runs,
		Succeeded:       worker.succeeded,
		Failed:          worker.failed,
		Skipped:         worker.skipped,
		LastStartedAt:   worker.lastStartedAt,
		LastCompletedAt: worker.lastCompletedAt,
		LastError:       worker.lastError,
	}
	if worker.lastDurationMs != nil {
		duration := *worker.lastDurationMs
		snapshot.LastDurationMs = &duration
	}
	return snapshot
}
