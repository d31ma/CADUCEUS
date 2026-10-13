package accountsync

import (
	"context"
	"errors"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

type SchedulerOptions struct {
	Clock       func() time.Time
	Interval    time.Duration
	SuccessPoll time.Duration
	BatchSize   int
}

type Scheduler struct {
	schedules   ScheduleStore
	accounts    AccountSource
	runner      AccountRunner
	clock       func() time.Time
	interval    time.Duration
	successPoll time.Duration
	batchSize   int
}

func NewScheduler(schedules ScheduleStore, source AccountSource, runner AccountRunner, options SchedulerOptions) *Scheduler {
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	if options.Interval <= 0 {
		options.Interval = 5 * time.Second
	}
	if options.SuccessPoll <= 0 {
		options.SuccessPoll = time.Minute
	}
	if options.BatchSize <= 0 || options.BatchSize > 100 {
		options.BatchSize = 20
	}
	return &Scheduler{schedules: schedules, accounts: source, runner: runner, clock: clock, interval: options.Interval, successPoll: options.SuccessPoll, batchSize: options.BatchSize}
}

func (scheduler *Scheduler) Run(ctx context.Context) error {
	if scheduler == nil || scheduler.schedules == nil || scheduler.accounts == nil || scheduler.runner == nil {
		return errors.New("external account sync scheduler is not configured")
	}
	if err := scheduler.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	ticker := time.NewTicker(scheduler.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := scheduler.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

func (scheduler *Scheduler) Tick(ctx context.Context) error {
	now := scheduler.clock().UTC()
	due, err := scheduler.schedules.DueAccounts(ctx, now, scheduler.batchSize)
	if err != nil {
		return err
	}
	for _, scheduled := range due {
		account, err := scheduler.accounts.Get(ctx, scheduled.Scope.Owner, scheduled.Scope.AccountID)
		if err != nil {
			if errors.Is(err, accounts.ErrNotFound) {
				continue
			}
			return err
		}
		next := scheduled
		finalized := false
		switch account.State {
		case accounts.StateReady, accounts.StateIdle, accounts.StateDegraded:
			if err := scheduler.runner.RunMutations(ctx, scheduled.Scope.Owner, account); err != nil && !errors.Is(err, ErrAmbiguousOutcome) && !errors.Is(err, ErrLeaseHeld) {
				next = scheduler.retry(next, now, "mutation_failed")
				if err := scheduler.schedules.Reschedule(ctx, next); err != nil {
					return err
				}
				continue
			}
			// Reload because lifecycle mutations use optimistic revisions and a
			// previous worker or API request may have changed the state.
			account, err = scheduler.accounts.Get(ctx, scheduled.Scope.Owner, scheduled.Scope.AccountID)
			if err == nil {
				err = scheduler.runner.RunAccount(ctx, scheduled.Scope.Owner, account)
			}
			if err == nil {
				next.Attempts, next.LastCode, next.NextRunAt = 0, "", now.Add(scheduler.successPoll)
			} else if errors.Is(err, ErrLeaseHeld) || errors.Is(err, ErrAmbiguousOutcome) {
				next.NextRunAt = now.Add(10 * time.Second)
			} else {
				next = scheduler.retry(next, now, "sync_failed")
			}
		case accounts.StatePaused, accounts.StateAuthRequired:
			next.NextRunAt = now.Add(24 * time.Hour)
		case accounts.StateDeleting:
			err = scheduler.runner.CleanupAccount(ctx, scheduled.Scope.Owner, account)
			if err == nil {
				deletions, ok := scheduler.accounts.(AccountDeletionSource)
				if !ok {
					return errors.New("external account deletion finalizer is not configured")
				}
				err = deletions.FinalizeDelete(ctx, scheduled.Scope.Owner, account.ID)
			}
			if err == nil {
				finalized = true
			} else if errors.Is(err, ErrLeaseHeld) {
				next.NextRunAt = now.Add(10 * time.Second)
			} else {
				next = scheduler.retry(next, now, "cleanup_failed")
			}
		case accounts.StateSyncing:
			next.NextRunAt = now.Add(30 * time.Second)
		default:
			next = scheduler.retry(next, now, "invalid_state")
		}
		if finalized {
			continue
		}
		if err := scheduler.schedules.Reschedule(ctx, next); err != nil {
			return err
		}
	}
	return nil
}

func (scheduler *Scheduler) retry(value ScheduledAccount, now time.Time, code string) ScheduledAccount {
	value.Attempts++
	shift := value.Attempts - 1
	if shift > 8 {
		shift = 8
	}
	delay := time.Minute * time.Duration(1<<shift)
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	value.LastCode, value.NextRunAt = code, now.Add(delay)
	return value
}
