package accountsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestSchedulerRunsDueAccountAndPersistsNextPoll(t *testing.T) {
	now := time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
	scheduled := ScheduledAccount{Scope: Scope{Owner: owner(), AccountID: account(1).ID}, Provider: accounts.ProviderIMAP, NextRunAt: now}
	store := &fakeScheduleStore{due: []ScheduledAccount{scheduled}}
	runner := &fakeRunner{}
	scheduler := NewScheduler(store, fakeAccountSource{account: account(4)}, runner, SchedulerOptions{Clock: func() time.Time { return now }, SuccessPoll: 2 * time.Minute})
	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.mutations != 1 || runner.syncs != 1 || len(store.saved) != 1 || !store.saved[0].NextRunAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("mutations=%d syncs=%d schedule=%#v", runner.mutations, runner.syncs, store.saved)
	}
}

func TestSchedulerBacksOffFailuresAndDoesNotRunPausedAccount(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeScheduleStore{due: []ScheduledAccount{{Scope: Scope{Owner: owner(), AccountID: account(1).ID}, Attempts: 2}}}
	runner := &fakeRunner{syncErr: errors.New("offline")}
	scheduler := NewScheduler(store, fakeAccountSource{account: account(1)}, runner, SchedulerOptions{Clock: func() time.Time { return now }})
	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.saved[0].Attempts != 3 || !store.saved[0].NextRunAt.Equal(now.Add(4*time.Minute)) {
		t.Fatalf("retry = %#v", store.saved[0])
	}

	paused := account(1)
	paused.State = accounts.StatePaused
	store.due, store.saved = []ScheduledAccount{{Scope: Scope{Owner: owner(), AccountID: paused.ID}}}, nil
	scheduler.accounts = fakeAccountSource{account: paused}
	runner.syncs = 0
	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.syncs != 0 || !store.saved[0].NextRunAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("paused schedule = %#v", store.saved)
	}
}

func TestSchedulerCleansAndFinalizesDeletingAccount(t *testing.T) {
	now := time.Now().UTC()
	deleting := account(4)
	deleting.State = accounts.StateDeleting
	store := &fakeScheduleStore{due: []ScheduledAccount{{Scope: Scope{Owner: owner(), AccountID: deleting.ID}}}}
	source := &fakeDeletionSource{account: deleting}
	runner := &fakeRunner{}
	scheduler := NewScheduler(store, source, runner, SchedulerOptions{Clock: func() time.Time { return now }})

	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.cleanups != 1 || source.finalized != 1 || len(store.saved) != 0 {
		t.Fatalf("cleanups=%d finalized=%d reschedules=%d", runner.cleanups, source.finalized, len(store.saved))
	}
}

type fakeScheduleStore struct {
	due   []ScheduledAccount
	saved []ScheduledAccount
}

func (store *fakeScheduleStore) DueAccounts(context.Context, time.Time, int) ([]ScheduledAccount, error) {
	return store.due, nil
}
func (store *fakeScheduleStore) Reschedule(_ context.Context, value ScheduledAccount) error {
	store.saved = append(store.saved, value)
	return nil
}

type fakeAccountSource struct{ account accounts.ExternalAccount }

func (source fakeAccountSource) Get(context.Context, accounts.Owner, accounts.AccountID) (accounts.ExternalAccount, error) {
	return source.account, nil
}

type fakeDeletionSource struct {
	account   accounts.ExternalAccount
	finalized int
}

func (source *fakeDeletionSource) Get(context.Context, accounts.Owner, accounts.AccountID) (accounts.ExternalAccount, error) {
	return source.account, nil
}
func (source *fakeDeletionSource) FinalizeDelete(context.Context, accounts.Owner, accounts.AccountID) error {
	source.finalized++
	return nil
}

type fakeRunner struct {
	mutations   int
	syncs       int
	mutationErr error
	syncErr     error
	cleanups    int
}

func (runner *fakeRunner) RunMutations(context.Context, accounts.Owner, accounts.ExternalAccount) error {
	runner.mutations++
	return runner.mutationErr
}
func (runner *fakeRunner) RunAccount(context.Context, accounts.Owner, accounts.ExternalAccount) error {
	runner.syncs++
	return runner.syncErr
}
func (runner *fakeRunner) CleanupAccount(context.Context, accounts.Owner, accounts.ExternalAccount) error {
	runner.cleanups++
	return nil
}
