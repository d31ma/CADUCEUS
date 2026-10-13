package fylostore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accountsend"
)

func TestAccountSyncPersistsSubmissionRawInBucketAndTransitionsUnderLease(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	raw := []byte("From: remote@example.net\r\nTo: recipient@example.org\r\n\r\nhello\r\n")
	submission := accountsend.Submission{ID: "sub_abcdefghijklmnopqrst", IdempotencyKey: "compose-1", Revision: 1, Scope: scope, EnvelopeFrom: "remote@example.net",
		Recipients: []string{"recipient@example.org"}, RawRFC822: raw, State: accountsend.StateQueued, CreatedAt: now, UpdatedAt: now}
	queued, err := store.EnqueueSubmission(context.Background(), scope, submission)
	if err != nil || queued.ID != submission.ID {
		t.Fatalf("EnqueueSubmission() = %#v, %v", queued, err)
	}
	duplicate := submission
	duplicate.ID = "sub_bbbbbbbbbbbbbbbbbbbb"
	if got, err := store.EnqueueSubmission(context.Background(), scope, duplicate); err != nil || got.ID != submission.ID {
		t.Fatalf("idempotent EnqueueSubmission() = %#v, %v", got, err)
	}
	if countCollection(machine, ProviderSubmissions) != 1 || countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("submission docs=%d raw=%d", countCollection(machine, ProviderSubmissions), countCollection(machine, ProviderSubmitFiles))
	}
	if document := oneDocument(machine, ProviderSubmissions); document["rawRFC822"] != nil || document["rawBucketId"] == "" {
		t.Fatalf("submission document leaked or omitted bucket identity: %#v", document)
	}
	lease, err := store.AcquireLease(context.Background(), scope, "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingSubmissions(context.Background(), lease, 20)
	if err != nil || len(pending) != 1 || string(pending[0].RawRFC822) != string(raw) {
		t.Fatalf("PendingSubmissions() = %#v, %v", pending, err)
	}
	pending[0].State = accountsend.StateApplying
	pending[0].Attempts = 1
	pending[0].Revision++
	if err := store.UpdateSubmission(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	pending[0].State = accountsend.StateSent
	pending[0].Revision++
	if err := store.UpdateSubmission(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	if countCollection(machine, ProviderSubmitFiles) != 0 {
		t.Fatal("sent submission retained its raw queue bucket")
	}
}

func TestAccountSyncCancelsQueuedSubmissionAndCleanupPurgesAmbiguousJournal(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	makeSubmission := func(id, key string) accountsend.Submission {
		return accountsend.Submission{ID: id, IdempotencyKey: key, Revision: 1, Scope: scope, EnvelopeFrom: "remote@example.net", Recipients: []string{"recipient@example.org"},
			RawRFC822: []byte("From: remote@example.net\r\n\r\nhello\r\n"), State: accountsend.StateQueued, CreatedAt: now, UpdatedAt: now}
	}
	queued, _ := store.EnqueueSubmission(context.Background(), scope, makeSubmission("sub_abcdefghijklmnopqrst", "cancel-1"))
	cancelled, err := store.CancelSubmission(context.Background(), scope, queued.ID)
	if err != nil || cancelled.State != accountsend.StateCancelled || countCollection(machine, ProviderSubmitFiles) != 0 {
		t.Fatalf("CancelSubmission() = %#v, %v raw=%d", cancelled, err, countCollection(machine, ProviderSubmitFiles))
	}
	if _, err := store.CancelSubmission(context.Background(), scope, queued.ID); !errors.Is(err, accountsend.ErrSubmissionConflict) {
		t.Fatalf("second cancel error = %v", err)
	}
	queued, _ = store.EnqueueSubmission(context.Background(), scope, makeSubmission("sub_bbbbbbbbbbbbbbbbbbbb", "cleanup-1"))
	lease, _ := store.AcquireLease(context.Background(), scope, "cleanup", now, time.Minute)
	pending, _ := store.PendingSubmissions(context.Background(), lease, 20)
	pending[0].State = accountsend.StateApplying
	pending[0].Revision++
	if err := store.UpdateSubmission(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CancelSubmission(context.Background(), scope, queued.ID); !errors.Is(err, accountsend.ErrSubmissionConflict) {
		t.Fatalf("applying cancel error = %v", err)
	}
	if err := store.DeleteAccountData(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{ProviderSubmissions, ProviderSubmitFiles, ProviderSyncState} {
		if got := countCollection(machine, collection); got != 0 {
			t.Fatalf("%s retained %d records", collection, got)
		}
	}
}

func TestAccountSyncReplacesOnlyQueuedSubmissionWithRevisionCAS(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	original := accountsend.Submission{ID: "sub_abcdefghijklmnopqrst", IdempotencyKey: "edit-1", Revision: 1, Scope: scope,
		EnvelopeFrom: "remote@example.net", Recipients: []string{"first@example.org"}, RawRFC822: []byte("original"),
		State: accountsend.StateQueued, NotBefore: timePointer(now.Add(time.Hour)), CreatedAt: now, UpdatedAt: now}
	if _, err := store.EnqueueSubmission(context.Background(), scope, original); err != nil {
		t.Fatal(err)
	}
	nextTime := now.Add(2 * time.Hour)
	replaced, err := store.ReplaceSubmission(context.Background(), scope, accountsend.Request{ID: original.ID, IdempotencyKey: original.IdempotencyKey,
		EnvelopeFrom: original.EnvelopeFrom, Recipients: []string{"second@example.org"}, RawRFC822: []byte("replacement"), NotBefore: &nextTime}, 1)
	if err != nil || replaced.Revision != 2 || replaced.Attempts != 0 || len(replaced.Recipients) != 1 || replaced.Recipients[0] != "second@example.org" {
		t.Fatalf("ReplaceSubmission() = %#v, %v", replaced, err)
	}
	if countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("raw bucket count = %d", countCollection(machine, ProviderSubmitFiles))
	}
	lease, _ := store.AcquireLease(context.Background(), scope, "worker", now, time.Minute)
	pending, err := store.PendingSubmissions(context.Background(), lease, 20)
	if err != nil || len(pending) != 1 || string(pending[0].RawRFC822) != "replacement" {
		t.Fatalf("PendingSubmissions() = %#v, %v", pending, err)
	}
	if _, err := store.ReplaceSubmission(context.Background(), scope, accountsend.Request{ID: original.ID, IdempotencyKey: original.IdempotencyKey,
		EnvelopeFrom: original.EnvelopeFrom, Recipients: original.Recipients, RawRFC822: []byte("stale")}, 1); !errors.Is(err, accountsend.ErrSubmissionConflict) {
		t.Fatalf("stale ReplaceSubmission() error = %v", err)
	}
}

func TestAccountSyncStagesRecipientBatchUntilAtomicCommit(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	batch := accountsend.Submission{ID: "sub_batchabcdefghijklmnop", IdempotencyKey: "batch-1", Revision: 1, Scope: scope,
		EnvelopeFrom: "remote@example.net", Recipients: []string{"one@example.org", "two@example.org"}, State: accountsend.StateBuilding,
		CreatedAt: now, UpdatedAt: now}
	batch, err := store.BeginSubmissionBatch(context.Background(), scope, batch)
	if err != nil || batch.State != accountsend.StateBuilding || countCollection(machine, ProviderSubmitFiles) != 0 {
		t.Fatalf("BeginSubmissionBatch() = %#v, %v", batch, err)
	}
	first := accountsend.AppendRequest{ID: batch.ID, Revision: batch.Revision, Index: 0, Recipient: "one@example.org", RawRFC822: []byte("first")}
	batch, err = store.AppendSubmissionDelivery(context.Background(), scope, first)
	if err != nil || batch.Revision != 2 || countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("first append = %#v, %v files=%d", batch, err, countCollection(machine, ProviderSubmitFiles))
	}
	replayed, err := store.AppendSubmissionDelivery(context.Background(), scope, first)
	if err != nil || replayed.Revision != 2 || countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("replayed append = %#v, %v files=%d", replayed, err, countCollection(machine, ProviderSubmitFiles))
	}
	batch, err = store.AppendSubmissionDelivery(context.Background(), scope, accountsend.AppendRequest{ID: batch.ID, Revision: batch.Revision,
		Index: 1, Recipient: "two@example.org", RawRFC822: []byte("second")})
	if err != nil || batch.Revision != 3 || countCollection(machine, ProviderSubmitFiles) != 2 {
		t.Fatalf("second append = %#v, %v files=%d", batch, err, countCollection(machine, ProviderSubmitFiles))
	}
	lease, _ := store.AcquireLease(context.Background(), scope, "worker", now, time.Minute)
	if pending, err := store.PendingSubmissions(context.Background(), lease, 20); err != nil || len(pending) != 0 {
		t.Fatalf("building PendingSubmissions() = %#v, %v", pending, err)
	}
	batch, err = store.CommitSubmissionBatch(context.Background(), scope, batch.ID, batch.Revision)
	if err != nil || batch.State != accountsend.StateQueued || batch.Revision != 4 {
		t.Fatalf("CommitSubmissionBatch() = %#v, %v", batch, err)
	}
	pending, err := store.PendingSubmissions(context.Background(), lease, 20)
	if err != nil || len(pending) != 1 || len(pending[0].Deliveries) != 2 || string(pending[0].Deliveries[1].RawRFC822) != "second" {
		t.Fatalf("committed PendingSubmissions() = %#v, %v", pending, err)
	}
	if _, err := store.CancelSubmission(context.Background(), scope, batch.ID); err != nil || countCollection(machine, ProviderSubmitFiles) != 0 {
		t.Fatalf("CancelSubmission() error=%v files=%d", err, countCollection(machine, ProviderSubmitFiles))
	}
}

func TestAccountSyncReplacesCommittedBatchAtomicallyAndCanAbort(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	batch := accountsend.Submission{ID: "sub_batchreplaceabcdefgh", IdempotencyKey: "batch-edit-1", Revision: 1, Scope: scope,
		EnvelopeFrom: "remote@example.net", Recipients: []string{"old@example.org"}, State: accountsend.StateBuilding,
		CreatedAt: now, UpdatedAt: now}
	batch, _ = store.BeginSubmissionBatch(context.Background(), scope, batch)
	batch, _ = store.AppendSubmissionDelivery(context.Background(), scope, accountsend.AppendRequest{ID: batch.ID, Revision: batch.Revision,
		Index: 0, Recipient: "old@example.org", RawRFC822: []byte("old")})
	batch, _ = store.CommitSubmissionBatch(context.Background(), scope, batch.ID, batch.Revision)
	if countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatal("initial batch bucket was not persisted")
	}

	replacement := accountsend.BatchRequest{ID: batch.ID, IdempotencyKey: batch.IdempotencyKey, EnvelopeFrom: batch.EnvelopeFrom,
		Recipients: []string{"new-one@example.org", "new-two@example.org"}, NotBefore: timePointer(now.Add(2 * time.Hour))}
	editing, err := store.BeginSubmissionBatchReplacement(context.Background(), scope, replacement, batch.Revision)
	if err != nil || editing.State != accountsend.StateEditing || editing.Revision != batch.Revision+1 {
		t.Fatalf("BeginSubmissionBatchReplacement() = %#v, %v", editing, err)
	}
	editing, err = store.AppendSubmissionReplacementDelivery(context.Background(), scope, accountsend.AppendRequest{ID: editing.ID, Revision: editing.Revision,
		Index: 0, Recipient: replacement.Recipients[0], RawRFC822: []byte("discarded")})
	if err != nil || countCollection(machine, ProviderSubmitFiles) != 2 {
		t.Fatalf("replacement append = %#v, %v files=%d", editing, err, countCollection(machine, ProviderSubmitFiles))
	}
	batch, err = store.AbortSubmissionBatchReplacement(context.Background(), scope, editing.ID, editing.Revision)
	if err != nil || batch.State != accountsend.StateQueued || countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("AbortSubmissionBatchReplacement() = %#v, %v files=%d", batch, err, countCollection(machine, ProviderSubmitFiles))
	}

	editing, err = store.BeginSubmissionBatchReplacement(context.Background(), scope, replacement, batch.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for index, recipient := range replacement.Recipients {
		editing, err = store.AppendSubmissionReplacementDelivery(context.Background(), scope, accountsend.AppendRequest{ID: editing.ID,
			Revision: editing.Revision, Index: index, Recipient: recipient, RawRFC822: []byte("new-" + recipient)})
		if err != nil {
			t.Fatal(err)
		}
	}
	batch, err = store.CommitSubmissionBatchReplacement(context.Background(), scope, editing.ID, editing.Revision)
	if err != nil || batch.State != accountsend.StateQueued || batch.DeliveryCount != 2 || countCollection(machine, ProviderSubmitFiles) != 2 {
		t.Fatalf("CommitSubmissionBatchReplacement() = %#v, %v files=%d", batch, err, countCollection(machine, ProviderSubmitFiles))
	}
	lease, _ := store.AcquireLease(context.Background(), scope, "worker", now, time.Minute)
	pending, err := store.PendingSubmissions(context.Background(), lease, 20)
	if err != nil || len(pending) != 1 || len(pending[0].Deliveries) != 2 || string(pending[0].Deliveries[0].RawRFC822) != "new-new-one@example.org" {
		t.Fatalf("replacement PendingSubmissions() = %#v, %v", pending, err)
	}
}

func TestAccountSyncRecoversAbandonedBatchEditWithoutLosingActiveDelivery(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	batch := accountsend.Submission{ID: "sub_batchrecoveryabcdefgh", IdempotencyKey: "batch-recover-1", Revision: 1, Scope: scope,
		EnvelopeFrom: "remote@example.net", Recipients: []string{"old@example.org"}, State: accountsend.StateBuilding,
		CreatedAt: now, UpdatedAt: now}
	batch, _ = store.BeginSubmissionBatch(context.Background(), scope, batch)
	batch, _ = store.AppendSubmissionDelivery(context.Background(), scope, accountsend.AppendRequest{ID: batch.ID, Revision: batch.Revision,
		Index: 0, Recipient: "old@example.org", RawRFC822: []byte("active")})
	batch, _ = store.CommitSubmissionBatch(context.Background(), scope, batch.ID, batch.Revision)
	editing, _ := store.BeginSubmissionBatchReplacement(context.Background(), scope, accountsend.BatchRequest{ID: batch.ID,
		IdempotencyKey: batch.IdempotencyKey, EnvelopeFrom: batch.EnvelopeFrom, Recipients: []string{"new@example.org"}}, batch.Revision)
	_, _ = store.AppendSubmissionReplacementDelivery(context.Background(), scope, accountsend.AppendRequest{ID: batch.ID, Revision: editing.Revision,
		Index: 0, Recipient: "new@example.org", RawRFC822: []byte("staged")})
	oneDocument(machine, ProviderSubmissions)["updatedAt"] = now.Add(-submissionEditTimeout - time.Minute).Format(time.RFC3339Nano)
	lease, _ := store.AcquireLease(context.Background(), scope, "worker", now, time.Minute)
	pending, err := store.PendingSubmissions(context.Background(), lease, 20)
	if err != nil || len(pending) != 1 || pending[0].State != accountsend.StateQueued || len(pending[0].Deliveries) != 1 ||
		string(pending[0].Deliveries[0].RawRFC822) != "active" || countCollection(machine, ProviderSubmitFiles) != 1 {
		t.Fatalf("recovered PendingSubmissions() = %#v, %v files=%d", pending, err, countCollection(machine, ProviderSubmitFiles))
	}
}

var _ accountsend.Store = (*AccountSync)(nil)
