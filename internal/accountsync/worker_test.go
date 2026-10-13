package accountsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestWorkerCommitsIdempotentBatchBeforeCheckpointAndCompletes(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	lifecycle := newLifecycle(now)
	connector := &fakeConnector{batches: []Batch{{
		CursorBefore: "", CursorAfter: "imap:7", Phase: accounts.PhaseHeaders,
		Folders:  []Folder{{StableID: "INBOX", Revision: "9", Name: "INBOX", Role: "inbox"}},
		Messages: []Message{{StableID: "INBOX:42", Revision: "7", FolderStableID: "INBOX", Subject: "hello", ReceivedAt: now, Size: 22, RawRFC822: []byte("Subject: hello\r\n\r\nbody")}},
	}}}
	worker := NewWorker(store, lifecycle, fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{
		Clock: func() time.Time { return now }, LeaseTTL: time.Minute, Holder: "worker-a",
	})

	if err := worker.RunAccount(context.Background(), owner(), account(1)); err != nil {
		t.Fatalf("RunAccount() error = %v", err)
	}
	if store.checkpoint.Cursor != "imap:7" || store.checkpoint.Version != 1 {
		t.Fatalf("checkpoint = %#v", store.checkpoint)
	}
	if len(store.messages) != 1 || len(store.folders) != 1 {
		t.Fatalf("committed messages=%d folders=%d", len(store.messages), len(store.folders))
	}
	if lifecycle.state != accounts.StateIdle || lifecycle.progress.Phase != accounts.PhaseComplete {
		t.Fatalf("lifecycle = %s %#v", lifecycle.state, lifecycle.progress)
	}
	if len(store.events) < 3 || store.events[0] != "lease" || store.events[1] != "batch" || store.events[2] != "checkpoint" {
		t.Fatalf("durability order = %v", store.events)
	}

	// A provider may repeat the last page after a restart. Stable keys replace
	// the same rows, and only the next cursor version advances.
	lifecycle.state = accounts.StateIdle
	connector.batches[0].CursorBefore = "imap:7"
	connector.batches[0].CursorAfter = "imap:8"
	connector.batches[0].Messages[0].Revision = "8"
	if err := worker.RunAccount(context.Background(), owner(), account(lifecycle.revision)); err != nil {
		t.Fatalf("restart RunAccount() error = %v", err)
	}
	if len(store.messages) != 1 || store.checkpoint.Cursor != "imap:8" || store.checkpoint.Version != 2 {
		t.Fatalf("restart state messages=%d checkpoint=%#v", len(store.messages), store.checkpoint)
	}
}

func TestWorkerNeverAdvancesCheckpointWhenBatchWriteFails(t *testing.T) {
	store := newMemoryStore()
	store.commitErr = errors.New("disk unavailable")
	lifecycle := newLifecycle(time.Now().UTC())
	connector := &fakeConnector{batches: []Batch{{CursorAfter: "lost", Phase: accounts.PhaseHeaders}}}
	worker := NewWorker(store, lifecycle, fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})

	err := worker.RunAccount(context.Background(), owner(), account(1))
	if err == nil || store.checkpoint.Cursor != "" || lifecycle.state != accounts.StateDegraded {
		t.Fatalf("RunAccount()=%v checkpoint=%#v state=%s", err, store.checkpoint, lifecycle.state)
	}
}

func TestWorkerJournalsRemoteMutationAndReconcilesAmbiguousOutcome(t *testing.T) {
	store := newMemoryStore()
	lifecycle := newLifecycle(time.Now().UTC())
	connector := &fakeConnector{mutationResults: []MutationResult{{Outcome: OutcomeUnknown}, {Outcome: OutcomeApplied}}}
	worker := NewWorker(store, lifecycle, fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})
	queued, err := worker.EnqueueMutation(context.Background(), owner(), account(1), MutationRequest{
		ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "client-action-1", Kind: MutationSetRead,
		MessageStableIDs: []string{"INBOX:42"}, Value: true,
	})
	if err != nil || queued.State != MutationQueued {
		t.Fatalf("EnqueueMutation() = %#v, %v", queued, err)
	}
	status, err := worker.Mutation(context.Background(), owner(), account(1), queued.ID)
	if err != nil || status.ID != queued.ID {
		t.Fatalf("Mutation() = %#v, %v", status, err)
	}
	foreign := owner()
	foreign.PrincipalID = "foreign"
	if _, err := worker.Mutation(context.Background(), foreign, account(1), queued.ID); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign Mutation() error = %v", err)
	}

	if err := worker.RunMutations(context.Background(), owner(), account(1)); !errors.Is(err, ErrAmbiguousOutcome) {
		t.Fatalf("first RunMutations() error = %v", err)
	}
	if store.mutations[queued.ID].State != MutationReconciling {
		t.Fatalf("mutation after ambiguity = %#v", store.mutations[queued.ID])
	}
	if err := worker.RunMutations(context.Background(), owner(), account(1)); err != nil {
		t.Fatalf("reconcile RunMutations() error = %v", err)
	}
	if store.mutations[queued.ID].State != MutationApplied || connector.applyCalls != 1 || connector.reconcileCalls != 1 || store.projections != 1 {
		t.Fatalf("mutation=%#v apply=%d reconcile=%d", store.mutations[queued.ID], connector.applyCalls, connector.reconcileCalls)
	}
}

func TestWorkerReconcilesApplyingMutationAfterCrashBeforeProjecting(t *testing.T) {
	store := newMemoryStore()
	mutation := Mutation{MutationRequest: MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "crash-recovery", Kind: MutationSetStarred,
		MessageStableIDs: []string{"INBOX:42"}, Value: true}, Scope: Scope{Owner: owner(), AccountID: account(1).ID}, State: MutationApplying}
	store.mutations[mutation.ID] = mutation
	connector := &fakeConnector{mutationResults: []MutationResult{{Outcome: OutcomeApplied}}}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})

	if err := worker.RunMutations(context.Background(), owner(), account(1)); err != nil {
		t.Fatalf("RunMutations() error = %v", err)
	}
	if connector.applyCalls != 0 || connector.reconcileCalls != 1 || store.projections != 1 || store.mutations[mutation.ID].State != MutationApplied {
		t.Fatalf("apply=%d reconcile=%d projections=%d mutation=%#v", connector.applyCalls, connector.reconcileCalls, store.projections, store.mutations[mutation.ID])
	}
}

func TestWorkerKeepsAppliedRemoteMutationReconcilableWhenProjectionFails(t *testing.T) {
	store := newMemoryStore()
	store.projectionErr = errors.New("index unavailable")
	mutation := Mutation{MutationRequest: MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "projection-retry", Kind: MutationSetRead,
		MessageStableIDs: []string{"INBOX:42"}, Value: true}, Scope: Scope{Owner: owner(), AccountID: account(1).ID}, State: MutationApplying}
	store.mutations[mutation.ID] = mutation
	connector := &fakeConnector{mutationResults: []MutationResult{{Outcome: OutcomeApplied}}}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})

	if err := worker.RunMutations(context.Background(), owner(), account(1)); !errors.Is(err, store.projectionErr) {
		t.Fatalf("RunMutations() error = %v", err)
	}
	if store.mutations[mutation.ID].State != MutationReconciling || store.mutations[mutation.ID].LastCode != "projection_failed" {
		t.Fatalf("mutation = %#v", store.mutations[mutation.ID])
	}
}

func TestWorkerPersistsMoveIdentityBeforeProjection(t *testing.T) {
	store := newMemoryStore()
	mutation := Mutation{MutationRequest: MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "move-identity", Kind: MutationMove,
		MessageStableIDs: []string{"INBOX:77:9"}, TargetFolderID: "Archive"}, Scope: Scope{Owner: owner(), AccountID: account(1).ID}, State: MutationQueued}
	store.mutations[mutation.ID] = mutation
	connector := &fakeConnector{mutationResults: []MutationResult{{Outcome: OutcomeApplied, ResultStableIDs: []string{"Archive:88:41"}}}}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})

	if err := worker.RunMutations(context.Background(), owner(), account(1)); err != nil {
		t.Fatalf("RunMutations() error = %v", err)
	}
	stored := store.mutations[mutation.ID]
	if stored.State != MutationApplied || len(stored.ResultStableIDs) != 1 || stored.ResultStableIDs[0] != "Archive:88:41" {
		t.Fatalf("stored mutation = %#v", stored)
	}
	if len(store.projectionMutations) != 1 || store.projectionMutations[0].LastCode != "projection_pending" || store.projectionMutations[0].ResultStableIDs[0] != "Archive:88:41" {
		t.Fatalf("projection mutations = %#v", store.projectionMutations)
	}
}

func TestWorkerFailsClosedWhenMoveIdentityIsMissing(t *testing.T) {
	store := newMemoryStore()
	mutation := Mutation{MutationRequest: MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "missing-move-identity", Kind: MutationMove,
		MessageStableIDs: []string{"INBOX:77:9"}, TargetFolderID: "Archive"}, Scope: Scope{Owner: owner(), AccountID: account(1).ID}, State: MutationQueued}
	store.mutations[mutation.ID] = mutation
	connector := &fakeConnector{mutationResults: []MutationResult{{Outcome: OutcomeApplied}}}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a"})

	if err := worker.RunMutations(context.Background(), owner(), account(1)); !errors.Is(err, ErrAmbiguousOutcome) {
		t.Fatalf("RunMutations() error = %v", err)
	}
	stored := store.mutations[mutation.ID]
	if stored.State != MutationReconciling || stored.LastCode != "move_identity_missing" || store.projections != 0 {
		t.Fatalf("stored mutation = %#v projections=%d", stored, store.projections)
	}
}

func TestWorkerRunsAccountJobsInsideMutationLease(t *testing.T) {
	store := newMemoryStore()
	job := &fakeAccountJob{}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: &fakeConnector{}}, WorkerOptions{Holder: "worker-a", Jobs: []AccountJob{job}})
	if err := worker.RunMutations(context.Background(), owner(), account(1)); err != nil {
		t.Fatal(err)
	}
	if job.calls != 1 || job.lease.Token == "" || job.account.ID != account(1).ID || job.credentials.Password != "secret" {
		t.Fatalf("job calls=%d lease=%#v account=%#v credentials=%#v", job.calls, job.lease, job.account, job.credentials)
	}
}

func TestWorkerRunsSyncJobsAfterMailConnectorInsideSameLease(t *testing.T) {
	store := newMemoryStore()
	job := &fakeAccountJob{}
	connector := &fakeConnector{batches: []Batch{{CursorAfter: "mail-done", Phase: accounts.PhaseBodies}}}
	worker := NewWorker(store, newLifecycle(time.Now().UTC()), fakeSecrets{}, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Holder: "worker-a", SyncJobs: []AccountJob{job}})
	if err := worker.RunAccount(context.Background(), owner(), account(1)); err != nil {
		t.Fatal(err)
	}
	if job.calls != 1 || job.lease.Token == "" || store.checkpoint.Cursor != "mail-done" {
		t.Fatalf("job calls=%d lease=%#v checkpoint=%#v", job.calls, job.lease, store.checkpoint)
	}
}

func owner() accounts.Owner {
	return accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}
}

func account(revision uint64) accounts.ExternalAccount {
	return accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner(), Provider: accounts.ProviderIMAP, MailboxAddress: "remote@example.net", State: accounts.StateReady, Revision: revision}
}

type fakeSecrets struct{}

func (fakeSecrets) Credentials(context.Context, accounts.ExternalAccount) (accounts.Credentials, error) {
	return accounts.Credentials{Username: "remote@example.net", Password: "secret"}, nil
}

type fakeConnector struct {
	batches         []Batch
	mutationResults []MutationResult
	applyCalls      int
	reconcileCalls  int
}

type fakeAccountJob struct {
	calls       int
	lease       Lease
	account     accounts.ExternalAccount
	credentials accounts.Credentials
}

func (job *fakeAccountJob) RunWithLease(_ context.Context, lease Lease, account accounts.ExternalAccount, credentials accounts.Credentials) error {
	job.calls++
	job.lease, job.account, job.credentials = lease, account, credentials
	return nil
}

func (connector *fakeConnector) Sync(ctx context.Context, _ SyncRequest, sink ChangeSink) error {
	for _, batch := range connector.batches {
		if err := sink.Commit(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (connector *fakeConnector) ApplyMutation(context.Context, MutationContext) (MutationResult, error) {
	connector.applyCalls++
	return connector.nextMutationResult(), nil
}

func (connector *fakeConnector) ReconcileMutation(context.Context, MutationContext) (MutationResult, error) {
	connector.reconcileCalls++
	return connector.nextMutationResult(), nil
}

func (connector *fakeConnector) nextMutationResult() MutationResult {
	result := connector.mutationResults[0]
	connector.mutationResults = connector.mutationResults[1:]
	return result
}

type fakeLifecycle struct {
	now      time.Time
	state    accounts.State
	revision uint64
	progress accounts.SyncProgress
}

func newLifecycle(now time.Time) *fakeLifecycle {
	return &fakeLifecycle{now: now, state: accounts.StateReady, revision: 1}
}

func (lifecycle *fakeLifecycle) BeginSync(_ context.Context, _ accounts.Owner, _ accounts.AccountID, expected uint64) (accounts.ExternalAccount, error) {
	if expected != lifecycle.revision {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	lifecycle.state, lifecycle.revision = accounts.StateSyncing, lifecycle.revision+1
	return accountWithState(lifecycle.state, lifecycle.revision), nil
}

func (lifecycle *fakeLifecycle) ReportProgress(_ context.Context, _ accounts.Owner, _ accounts.AccountID, expected uint64, update accounts.SyncProgressUpdate) (accounts.ExternalAccount, error) {
	if expected != lifecycle.revision {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	lifecycle.revision++
	lifecycle.progress = accounts.SyncProgress{Phase: update.Phase, Completed: update.Completed, Total: update.Total, StartedAt: lifecycle.now, UpdatedAt: lifecycle.now}
	return accountWithState(lifecycle.state, lifecycle.revision), nil
}

func (lifecycle *fakeLifecycle) CompleteSync(_ context.Context, _ accounts.Owner, _ accounts.AccountID, expected uint64) (accounts.ExternalAccount, error) {
	if expected != lifecycle.revision {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	lifecycle.state, lifecycle.revision = accounts.StateIdle, lifecycle.revision+1
	lifecycle.progress.Phase = accounts.PhaseComplete
	return accountWithState(lifecycle.state, lifecycle.revision), nil
}

func (lifecycle *fakeLifecycle) RecordFailure(_ context.Context, _ accounts.Owner, _ accounts.AccountID, expected uint64, _ accounts.SyncFailure) (accounts.ExternalAccount, error) {
	if expected != lifecycle.revision {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	lifecycle.state, lifecycle.revision = accounts.StateDegraded, lifecycle.revision+1
	return accountWithState(lifecycle.state, lifecycle.revision), nil
}

func accountWithState(state accounts.State, revision uint64) accounts.ExternalAccount {
	value := account(revision)
	value.State = state
	return value
}

type memoryStore struct {
	lease               Lease
	checkpoint          Checkpoint
	folders             map[string]Folder
	messages            map[string]Message
	mutations           map[string]Mutation
	events              []string
	commitErr           error
	projections         int
	projectionMutations []Mutation
	projectionErr       error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{folders: map[string]Folder{}, messages: map[string]Message{}, mutations: map[string]Mutation{}}
}

func (store *memoryStore) AcquireLease(_ context.Context, _ Scope, holder string, now time.Time, ttl time.Duration) (Lease, error) {
	store.events = append(store.events, "lease")
	store.lease = Lease{Token: "lease-token", Holder: holder, ExpiresAt: now.Add(ttl)}
	return store.lease, nil
}
func (store *memoryStore) ReleaseLease(context.Context, Lease) error { return nil }
func (store *memoryStore) Checkpoint(context.Context, Scope) (Checkpoint, error) {
	return store.checkpoint, nil
}
func (store *memoryStore) CommitBatch(_ context.Context, _ Lease, expected uint64, batch Batch) (Checkpoint, error) {
	if store.commitErr != nil {
		return Checkpoint{}, store.commitErr
	}
	if expected != store.checkpoint.Version || batch.CursorBefore != store.checkpoint.Cursor {
		return Checkpoint{}, ErrCheckpointConflict
	}
	store.events = append(store.events, "batch")
	for _, folder := range batch.Folders {
		store.folders[folder.StableID] = folder
	}
	for _, message := range batch.Messages {
		store.messages[message.StableID] = message
	}
	store.events = append(store.events, "checkpoint")
	store.checkpoint = Checkpoint{Cursor: batch.CursorAfter, Version: expected + 1}
	return store.checkpoint, nil
}
func (store *memoryStore) EnqueueMutation(_ context.Context, _ Scope, mutation Mutation) (Mutation, error) {
	store.mutations[mutation.ID] = mutation
	return mutation, nil
}

func (store *memoryStore) MutationByIdempotencyKey(_ context.Context, scope Scope, key string) (Mutation, bool, error) {
	for _, mutation := range store.mutations {
		if mutation.Scope == scope && mutation.IdempotencyKey == key {
			return mutation, true, nil
		}
	}
	return Mutation{}, false, nil
}

func (store *memoryStore) Mutation(_ context.Context, scope Scope, id string) (Mutation, error) {
	mutation, found := store.mutations[id]
	if !found || mutation.Scope != scope {
		return Mutation{}, accounts.ErrNotFound
	}
	return mutation, nil
}
func (store *memoryStore) PendingMutations(context.Context, Lease, int) ([]Mutation, error) {
	for _, mutation := range store.mutations {
		if mutation.State == MutationQueued || mutation.State == MutationApplying || mutation.State == MutationReconciling {
			return []Mutation{mutation}, nil
		}
	}
	return nil, nil
}
func (store *memoryStore) UpdateMutation(_ context.Context, _ Lease, mutation Mutation) error {
	store.mutations[mutation.ID] = mutation
	return nil
}
func (store *memoryStore) ApplyMutationProjection(_ context.Context, _ Lease, mutation Mutation) error {
	store.projections++
	store.projectionMutations = append(store.projectionMutations, mutation)
	return store.projectionErr
}
func (store *memoryStore) DeleteAccountData(context.Context, Lease) error {
	store.messages = map[string]Message{}
	store.folders = map[string]Folder{}
	store.mutations = map[string]Mutation{}
	store.checkpoint = Checkpoint{}
	return nil
}
