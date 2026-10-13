package accountsync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

type WatcherOptions struct {
	Interval    time.Duration
	MaxWatchers int
}

// Watcher maintains bounded provider notification connections. It never
// projects mailbox data directly: a provider event only advances NextRunAt so
// the lease-owning synchronization worker performs the durable import.
type Watcher struct {
	schedules   WatchScheduleSource
	control     ScheduleControl
	accounts    AccountSource
	secrets     CredentialsSource
	connectors  map[accounts.Provider]ChangeWatcher
	interval    time.Duration
	maxWatchers int

	mutation    sync.Mutex
	sequence    uint64
	active      map[string]watchRegistration
	unsupported map[string]uint64
}

type watchRegistration struct {
	id     uint64
	cancel context.CancelFunc
}

func NewWatcher(schedules WatchScheduleSource, control ScheduleControl, source AccountSource, secrets CredentialsSource, connectors map[accounts.Provider]ChangeWatcher, options WatcherOptions) *Watcher {
	if options.Interval <= 0 {
		options.Interval = 30 * time.Second
	}
	if options.MaxWatchers <= 0 || options.MaxWatchers > 1000 {
		options.MaxWatchers = 100
	}
	return &Watcher{schedules: schedules, control: control, accounts: source, secrets: secrets, connectors: connectors,
		interval: options.Interval, maxWatchers: options.MaxWatchers, active: map[string]watchRegistration{}, unsupported: map[string]uint64{}}
}

func (watcher *Watcher) Run(ctx context.Context) error {
	if !watcher.configured() {
		return errors.New("external account change watcher is not configured")
	}
	if err := watcher.Reconcile(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(watcher.interval)
	defer ticker.Stop()
	defer watcher.stopAll()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := watcher.Reconcile(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

func (watcher *Watcher) Reconcile(ctx context.Context) error {
	if !watcher.configured() {
		return errors.New("external account change watcher is not configured")
	}
	scheduled, err := watcher.schedules.WatchAccounts(ctx, watcher.maxWatchers)
	if err != nil {
		return err
	}
	desired := make(map[string]bool, len(scheduled))
	for _, candidate := range scheduled {
		account, err := watcher.accounts.Get(ctx, candidate.Scope.Owner, candidate.Scope.AccountID)
		if err != nil {
			if errors.Is(err, accounts.ErrNotFound) {
				continue
			}
			return err
		}
		connector := watcher.connectors[account.Provider]
		if connector == nil || !watchableAccount(account) {
			continue
		}
		key := watchScopeKey(candidate.Scope)
		desired[key] = true
		watcher.start(ctx, key, candidate.Scope, account, connector)
	}
	watcher.stopUndesired(desired)
	return nil
}

func (watcher *Watcher) configured() bool {
	return watcher != nil && watcher.schedules != nil && watcher.control != nil && watcher.accounts != nil && watcher.secrets != nil
}

func (watcher *Watcher) start(parent context.Context, key string, scope Scope, account accounts.ExternalAccount, connector ChangeWatcher) {
	watcher.mutation.Lock()
	defer watcher.mutation.Unlock()
	unsupportedRevision, unsupported := watcher.unsupported[key]
	if _, running := watcher.active[key]; running || unsupported && unsupportedRevision == account.Revision || len(watcher.active) >= watcher.maxWatchers {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	watcher.sequence++
	registration := watchRegistration{id: watcher.sequence, cancel: cancel}
	watcher.active[key] = registration
	go watcher.wait(ctx, key, registration.id, scope, account, connector)
}

func (watcher *Watcher) wait(ctx context.Context, key string, registrationID uint64, scope Scope, account accounts.ExternalAccount, connector ChangeWatcher) {
	defer watcher.finish(key, registrationID)
	credentials, err := watcher.secrets.Credentials(ctx, account)
	if err != nil {
		return
	}
	defer clearCredentials(&credentials)
	changed, err := connector.WaitForChange(ctx, account, credentials)
	if errors.Is(err, ErrWatchUnsupported) {
		watcher.markUnsupported(key, account.Revision)
		return
	}
	if err == nil && changed {
		_ = watcher.control.Retry(context.WithoutCancel(ctx), scope)
	}
}

func (watcher *Watcher) finish(key string, registrationID uint64) {
	watcher.mutation.Lock()
	defer watcher.mutation.Unlock()
	if current, ok := watcher.active[key]; ok && current.id == registrationID {
		delete(watcher.active, key)
	}
}

func (watcher *Watcher) markUnsupported(key string, revision uint64) {
	watcher.mutation.Lock()
	defer watcher.mutation.Unlock()
	watcher.unsupported[key] = revision
}

func (watcher *Watcher) stopUndesired(desired map[string]bool) {
	watcher.mutation.Lock()
	defer watcher.mutation.Unlock()
	for key, registration := range watcher.active {
		if !desired[key] {
			registration.cancel()
			delete(watcher.active, key)
		}
	}
}

func (watcher *Watcher) stopAll() {
	watcher.mutation.Lock()
	defer watcher.mutation.Unlock()
	for key, registration := range watcher.active {
		registration.cancel()
		delete(watcher.active, key)
	}
}

func watchableAccount(account accounts.ExternalAccount) bool {
	return account.State == accounts.StateReady || account.State == accounts.StateIdle || account.State == accounts.StateDegraded
}

func watchScopeKey(scope Scope) string {
	return fmt.Sprintf("%s\x00%s\x00%s", scope.Owner.TenantID, scope.Owner.PrincipalID, scope.AccountID)
}
