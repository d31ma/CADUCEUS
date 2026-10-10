package accountsend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestWorkerEnqueuesExactOwnedSenderAndDeliversUnderLease(t *testing.T) {
	now := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	connector := &fakeConnector{results: []Result{{Outcome: OutcomeSent}}}
	worker := NewWorker(store, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Clock: func() time.Time { return now }})
	account := sendAccount()
	queued, err := worker.Enqueue(context.Background(), account.Owner, account, validRequest())
	if err != nil || queued.State != StateQueued || len(queued.RawRFC822) == 0 {
		t.Fatalf("Enqueue() = %#v, %v", queued, err)
	}
	lease := accountsync.Lease{Scope: queued.Scope, Token: "lease", Holder: "worker", ExpiresAt: now.Add(time.Minute)}
	if err := worker.RunWithLease(context.Background(), lease, account, accounts.Credentials{Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if got := store.items[queued.ID]; got.State != StateSent || got.Attempts != 1 || connector.calls != 1 {
		t.Fatalf("submission=%#v calls=%d", got, connector.calls)
	}
}

func TestWorkerMakesCrashRecoveryAmbiguousWithoutResending(t *testing.T) {
	now := time.Now().UTC()
	store := newMemoryStore()
	request := validRequest()
	store.items[request.ID] = Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Scope: accountsync.Scope{Owner: sendOwner(), AccountID: sendAccount().ID},
		Revision: 1, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, RawRFC822: request.RawRFC822, State: StateApplying, CreatedAt: now, UpdatedAt: now}
	connector := &fakeConnector{}
	worker := NewWorker(store, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Clock: func() time.Time { return now }})
	lease := accountsync.Lease{Scope: store.items[request.ID].Scope, Token: "lease", Holder: "worker", ExpiresAt: now.Add(time.Minute)}
	if err := worker.RunWithLease(context.Background(), lease, sendAccount(), accounts.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if got := store.items[request.ID]; got.State != StateAmbiguous || got.LastCode != "worker_restarted" || connector.calls != 0 {
		t.Fatalf("submission=%#v calls=%d", got, connector.calls)
	}
}

func TestWorkerRejectsSpoofedSenderAndCancelsOnlyQueuedWork(t *testing.T) {
	store := newMemoryStore()
	worker := NewWorker(store, map[accounts.Provider]Connector{}, WorkerOptions{})
	account := sendAccount()
	request := validRequest()
	request.EnvelopeFrom = "other@remote.example"
	if _, err := worker.Enqueue(context.Background(), account.Owner, account, request); !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("spoofed Enqueue() error = %v", err)
	}
	request = validRequest()
	queued, _ := NewWorker(store, map[accounts.Provider]Connector{}, WorkerOptions{}).Enqueue(context.Background(), account.Owner, account, request)
	cancelled, err := worker.Cancel(context.Background(), account.Owner, account, queued.ID)
	if err != nil || cancelled.State != StateCancelled {
		t.Fatalf("Cancel() = %#v, %v", cancelled, err)
	}
	if _, err := worker.Cancel(context.Background(), account.Owner, account, queued.ID); !errors.Is(err, ErrSubmissionConflict) {
		t.Fatalf("second Cancel() error = %v", err)
	}
}

func TestWorkerStagesAndDeliversTrackedBatchPerRecipient(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	connector := &fakeConnector{results: []Result{{Outcome: OutcomeSent}, {Outcome: OutcomeSent}}}
	worker := NewWorker(store, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Clock: func() time.Time { return now }})
	account := sendAccount()
	batch, err := worker.BeginBatch(context.Background(), account.Owner, account, BatchRequest{ID: "sub_batchabcdefghijklmnop", IdempotencyKey: "tracked-group-1",
		EnvelopeFrom: account.MailboxAddress, Recipients: []string{"one@example.net", "two@example.net"}})
	if err != nil || batch.State != StateBuilding || batch.Revision != 1 {
		t.Fatalf("BeginBatch() = %#v, %v", batch, err)
	}
	batch, err = worker.AppendBatch(context.Background(), account.Owner, account, AppendRequest{ID: batch.ID, Revision: batch.Revision, Index: 0,
		Recipient: "one@example.net", RawRFC822: []byte("first")})
	if err != nil || batch.Revision != 2 {
		t.Fatalf("first AppendBatch() = %#v, %v", batch, err)
	}
	batch, err = worker.AppendBatch(context.Background(), account.Owner, account, AppendRequest{ID: batch.ID, Revision: batch.Revision, Index: 1,
		Recipient: "two@example.net", RawRFC822: []byte("second")})
	if err != nil || batch.Revision != 3 {
		t.Fatalf("second AppendBatch() = %#v, %v", batch, err)
	}
	batch, err = worker.CommitBatch(context.Background(), account.Owner, account, batch.ID, batch.Revision)
	if err != nil || batch.State != StateQueued || batch.Revision != 4 {
		t.Fatalf("CommitBatch() = %#v, %v", batch, err)
	}
	lease := accountsync.Lease{Scope: batch.Scope, Token: "lease", Holder: "worker", ExpiresAt: now.Add(time.Minute)}
	if err := worker.RunWithLease(context.Background(), lease, account, accounts.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if got := store.items[batch.ID]; got.State != StateSent || connector.calls != 2 || len(connector.submissions) != 2 ||
		connector.submissions[0].Recipients[0] != "one@example.net" || string(connector.submissions[1].RawRFC822) != "second" {
		t.Fatalf("submission=%#v connector=%#v", got, connector.submissions)
	}
}

func TestWorkerMakesPartiallyDeliveredBatchAmbiguous(t *testing.T) {
	now := time.Now().UTC()
	store := newMemoryStore()
	account := sendAccount()
	scope := accountsync.Scope{Owner: account.Owner, AccountID: account.ID}
	store.items["sub_batchabcdefghijklmnop"] = Submission{ID: "sub_batchabcdefghijklmnop", IdempotencyKey: "partial-1", Revision: 4, Scope: scope,
		EnvelopeFrom: account.MailboxAddress, Recipients: []string{"one@example.net", "two@example.net"}, State: StateQueued,
		Deliveries: []Delivery{{Recipient: "one@example.net", RawRFC822: []byte("first")}, {Recipient: "two@example.net", RawRFC822: []byte("second")}}, CreatedAt: now, UpdatedAt: now}
	connector := &fakeConnector{results: []Result{{Outcome: OutcomeSent}, {Outcome: OutcomeRetry}}}
	worker := NewWorker(store, map[accounts.Provider]Connector{accounts.ProviderIMAP: connector}, WorkerOptions{Clock: func() time.Time { return now }})
	lease := accountsync.Lease{Scope: scope, Token: "lease", Holder: "worker", ExpiresAt: now.Add(time.Minute)}
	if err := worker.RunWithLease(context.Background(), lease, account, accounts.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if got := store.items["sub_batchabcdefghijklmnop"]; got.State != StateAmbiguous || got.LastCode != "batch_partial_delivery" || connector.calls != 2 {
		t.Fatalf("submission=%#v calls=%d", got, connector.calls)
	}
}

func validRequest() Request {
	return Request{ID: "sub_abcdefghijklmnopqrst", IdempotencyKey: "compose-action-1", EnvelopeFrom: "remote@remote.example",
		Recipients: []string{"recipient@example.net"}, RawRFC822: []byte("From: remote@remote.example\r\nTo: recipient@example.net\r\nMessage-ID: <one@remote.example>\r\n\r\nhello\r\n")}
}

func sendOwner() accounts.Owner {
	return accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}
}

func sendAccount() accounts.ExternalAccount {
	return accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: sendOwner(), Provider: accounts.ProviderIMAP, MailboxAddress: "remote@remote.example",
		State: accounts.StateIdle, Capabilities: accounts.Capabilities{Send: true}}
}

type memoryStore struct {
	items        map[string]Submission
	replacements map[string]Submission
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[string]Submission{}, replacements: map[string]Submission{}}
}

func (store *memoryStore) EnqueueSubmission(_ context.Context, _ accountsync.Scope, submission Submission) (Submission, error) {
	for _, current := range store.items {
		if current.IdempotencyKey == submission.IdempotencyKey && current.Scope == submission.Scope {
			return current, nil
		}
	}
	store.items[submission.ID] = submission
	return submission, nil
}

func (store *memoryStore) BeginSubmissionBatch(ctx context.Context, scope accountsync.Scope, submission Submission) (Submission, error) {
	return store.EnqueueSubmission(ctx, scope, submission)
}

func (store *memoryStore) AppendSubmissionDelivery(_ context.Context, scope accountsync.Scope, request AppendRequest) (Submission, error) {
	current, found := store.items[request.ID]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if request.Index < len(current.Deliveries) {
		delivery := current.Deliveries[request.Index]
		if delivery.Recipient == request.Recipient && string(delivery.RawRFC822) == string(request.RawRFC822) {
			return current, nil
		}
		return Submission{}, ErrSubmissionConflict
	}
	if current.State != StateBuilding || current.Revision != request.Revision || request.Index != len(current.Deliveries) || current.Recipients[request.Index] != request.Recipient {
		return Submission{}, ErrSubmissionConflict
	}
	current.Deliveries = append(current.Deliveries, Delivery{Recipient: request.Recipient, RawRFC822: append([]byte(nil), request.RawRFC822...)})
	current.Revision++
	store.items[request.ID] = current
	return current, nil
}

func (store *memoryStore) CommitSubmissionBatch(_ context.Context, scope accountsync.Scope, id string, revision uint64) (Submission, error) {
	current, found := store.items[id]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateBuilding || current.Revision != revision || len(current.Deliveries) != len(current.Recipients) {
		return Submission{}, ErrSubmissionConflict
	}
	current.State = StateQueued
	current.Revision++
	store.items[id] = current
	return current, nil
}

func (store *memoryStore) BeginSubmissionBatchReplacement(_ context.Context, scope accountsync.Scope, request BatchRequest, expectedRevision uint64) (Submission, error) {
	current, found := store.items[request.ID]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateQueued || current.Revision != expectedRevision || len(current.Deliveries) < 1 {
		return Submission{}, ErrSubmissionConflict
	}
	current.State = StateEditing
	current.Revision++
	store.items[request.ID] = current
	store.replacements[request.ID] = Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Scope: scope,
		Revision: current.Revision, EnvelopeFrom: request.EnvelopeFrom, Recipients: append([]string(nil), request.Recipients...),
		State: StateEditing, NotBefore: request.NotBefore}
	return current, nil
}

func (store *memoryStore) AppendSubmissionReplacementDelivery(_ context.Context, scope accountsync.Scope, request AppendRequest) (Submission, error) {
	current, found := store.items[request.ID]
	replacement := store.replacements[request.ID]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateEditing || current.Revision != request.Revision || request.Index != len(replacement.Deliveries) ||
		request.Index >= len(replacement.Recipients) || replacement.Recipients[request.Index] != request.Recipient {
		return Submission{}, ErrSubmissionConflict
	}
	replacement.Deliveries = append(replacement.Deliveries, Delivery{Recipient: request.Recipient, RawRFC822: append([]byte(nil), request.RawRFC822...)})
	current.Revision++
	replacement.Revision = current.Revision
	store.items[request.ID] = current
	store.replacements[request.ID] = replacement
	return current, nil
}

func (store *memoryStore) CommitSubmissionBatchReplacement(_ context.Context, scope accountsync.Scope, id string, revision uint64) (Submission, error) {
	current, found := store.items[id]
	replacement := store.replacements[id]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateEditing || current.Revision != revision || len(replacement.Deliveries) != len(replacement.Recipients) {
		return Submission{}, ErrSubmissionConflict
	}
	replacement.State = StateQueued
	replacement.Revision++
	store.items[id] = replacement
	delete(store.replacements, id)
	return replacement, nil
}

func (store *memoryStore) AbortSubmissionBatchReplacement(_ context.Context, scope accountsync.Scope, id string, revision uint64) (Submission, error) {
	current, found := store.items[id]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateEditing || current.Revision != revision {
		return Submission{}, ErrSubmissionConflict
	}
	current.State = StateQueued
	current.Revision++
	store.items[id] = current
	delete(store.replacements, id)
	return current, nil
}

func (store *memoryStore) PendingSubmissions(_ context.Context, lease accountsync.Lease, limit int) ([]Submission, error) {
	result := []Submission{}
	for _, submission := range store.items {
		if submission.Scope == lease.Scope && (submission.State == StateQueued || submission.State == StateApplying) && len(result) < limit {
			result = append(result, submission)
		}
	}
	return result, nil
}

func (store *memoryStore) ReplaceSubmission(_ context.Context, scope accountsync.Scope, request Request, expectedRevision uint64) (Submission, error) {
	current, found := store.items[request.ID]
	if !found || current.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if current.State != StateQueued || current.Revision != expectedRevision || current.IdempotencyKey != request.IdempotencyKey {
		return Submission{}, ErrSubmissionConflict
	}
	current.Revision++
	current.EnvelopeFrom = request.EnvelopeFrom
	current.Recipients = append([]string(nil), request.Recipients...)
	current.RawRFC822 = append([]byte(nil), request.RawRFC822...)
	current.NotBefore = request.NotBefore
	current.Attempts = 0
	current.LastCode = ""
	store.items[request.ID] = current
	return current, nil
}

func (store *memoryStore) UpdateSubmission(_ context.Context, _ accountsync.Lease, submission Submission) error {
	if current := store.items[submission.ID]; submission.Revision != current.Revision+1 {
		return ErrSubmissionConflict
	}
	store.items[submission.ID] = submission
	return nil
}

func (store *memoryStore) CancelSubmission(_ context.Context, scope accountsync.Scope, id string) (Submission, error) {
	submission, found := store.items[id]
	if !found || submission.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	if submission.State != StateQueued {
		return Submission{}, ErrSubmissionConflict
	}
	submission.State = StateCancelled
	submission.Revision++
	store.items[id] = submission
	return submission, nil
}

func (store *memoryStore) GetSubmission(_ context.Context, scope accountsync.Scope, id string) (Submission, error) {
	submission, found := store.items[id]
	if !found || submission.Scope != scope {
		return Submission{}, accounts.ErrNotFound
	}
	return submission, nil
}

type fakeConnector struct {
	results     []Result
	calls       int
	submissions []Submission
}

func (connector *fakeConnector) Submit(_ context.Context, input Context) (Result, error) {
	connector.calls++
	connector.submissions = append(connector.submissions, input.Submission)
	result := connector.results[0]
	connector.results = connector.results[1:]
	return result, nil
}
