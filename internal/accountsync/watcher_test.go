package accountsync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestWatcherTurnsProviderChangeIntoDurableScheduleWake(t *testing.T) {
	external := account(4)
	external.State = accounts.StateIdle
	scope := Scope{Owner: external.Owner, AccountID: external.ID}
	schedules := &fakeWatchSchedules{accounts: []ScheduledAccount{{Scope: scope, Provider: external.Provider}}, retried: make(chan Scope, 1)}
	watcher := NewWatcher(schedules, schedules, fakeAccountSource{account: external}, fakeWatchCredentials{}, map[accounts.Provider]ChangeWatcher{
		external.Provider: fakeChangeWatcher{changed: true},
	}, WatcherOptions{MaxWatchers: 2})
	if err := watcher.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case woke := <-schedules.retried:
		if woke != scope {
			t.Fatalf("woken scope = %#v", woke)
		}
	case <-time.After(time.Second):
		t.Fatal("provider change did not wake the durable schedule")
	}
}

func TestWatcherSuppressesUnsupportedRevision(t *testing.T) {
	external := account(4)
	external.State = accounts.StateReady
	scope := Scope{Owner: external.Owner, AccountID: external.ID}
	schedules := &fakeWatchSchedules{accounts: []ScheduledAccount{{Scope: scope, Provider: external.Provider}}, retried: make(chan Scope, 1)}
	connector := &fakeUnsupportedWatcher{}
	watcher := NewWatcher(schedules, schedules, fakeAccountSource{account: external}, fakeWatchCredentials{}, map[accounts.Provider]ChangeWatcher{
		external.Provider: connector,
	}, WatcherOptions{})
	if err := watcher.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for connector.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := watcher.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if connector.calls.Load() != 1 {
		t.Fatalf("unsupported connector calls = %d", connector.calls.Load())
	}
}

func TestWatcherRunStopsActiveConnectionsOnCancellation(t *testing.T) {
	external := account(4)
	external.State = accounts.StateReady
	scope := Scope{Owner: external.Owner, AccountID: external.ID}
	schedules := &fakeWatchSchedules{accounts: []ScheduledAccount{{Scope: scope, Provider: external.Provider}}, retried: make(chan Scope, 1)}
	connector := blockingChangeWatcher{started: make(chan struct{}), stopped: make(chan struct{})}
	watcher := NewWatcher(schedules, schedules, fakeAccountSource{account: external}, fakeWatchCredentials{}, map[accounts.Provider]ChangeWatcher{external.Provider: connector}, WatcherOptions{Interval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- watcher.Run(ctx) }()
	select {
	case <-connector.started:
	case <-time.After(time.Second):
		t.Fatal("watch connection did not start")
	}
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
	select {
	case <-connector.stopped:
	case <-time.After(time.Second):
		t.Fatal("active watch connection was not canceled")
	}
}

type fakeWatchSchedules struct {
	accounts []ScheduledAccount
	retried  chan Scope
}

func (store *fakeWatchSchedules) WatchAccounts(context.Context, int) ([]ScheduledAccount, error) {
	return store.accounts, nil
}

func (store *fakeWatchSchedules) Retry(_ context.Context, scope Scope) error {
	store.retried <- scope
	return nil
}

func (*fakeWatchSchedules) Diagnostics(context.Context, Scope) (Diagnostics, error) {
	return Diagnostics{}, nil
}

type fakeWatchCredentials struct{}

func (fakeWatchCredentials) Credentials(context.Context, accounts.ExternalAccount) (accounts.Credentials, error) {
	return accounts.Credentials{Password: "secret"}, nil
}

type fakeChangeWatcher struct{ changed bool }

func (watcher fakeChangeWatcher) WaitForChange(context.Context, accounts.ExternalAccount, accounts.Credentials) (bool, error) {
	return watcher.changed, nil
}

type fakeUnsupportedWatcher struct{ calls atomic.Uint64 }

func (watcher *fakeUnsupportedWatcher) WaitForChange(context.Context, accounts.ExternalAccount, accounts.Credentials) (bool, error) {
	watcher.calls.Add(1)
	return false, ErrWatchUnsupported
}

type blockingChangeWatcher struct {
	started chan struct{}
	stopped chan struct{}
}

func (watcher blockingChangeWatcher) WaitForChange(ctx context.Context, _ accounts.ExternalAccount, _ accounts.Credentials) (bool, error) {
	close(watcher.started)
	<-ctx.Done()
	close(watcher.stopped)
	return false, ctx.Err()
}
