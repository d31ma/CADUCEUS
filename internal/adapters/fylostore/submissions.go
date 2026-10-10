package fylostore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
	"github.com/d31ma/caduceus/internal/accountsync"
)

const submissionEditTimeout = 5 * time.Minute

func (store *AccountSync) EnqueueSubmission(ctx context.Context, scope accountsync.Scope, submission accountsend.Submission) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderSubmissions, store.documents.readAccess(scope.Owner))
	if err != nil {
		return accountsend.Submission{}, err
	}
	for _, raw := range documents {
		current, err := decodeSubmissionRecord(raw)
		if err != nil {
			return accountsend.Submission{}, err
		}
		if current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) {
			continue
		}
		if current.ID == submission.ID || current.IdempotencyKey == submission.IdempotencyKey {
			return current.submission(scope, nil), nil
		}
	}
	rawID, err := store.putSubmissionRaw(ctx, scope, submission)
	if err != nil {
		return accountsend.Submission{}, err
	}
	record := newSubmissionRecord(scope, submission, rawID)
	if _, err := store.documents.put(ctx, ProviderSubmissions, record, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		cleanup := store.documents.delete(ctx, ProviderSubmitFiles, rawID, store.documents.writeAccess(scope.Owner, scope.AccountID))
		return accountsend.Submission{}, errors.Join(err, cleanup)
	}
	return submission, nil
}

func (store *AccountSync) BeginSubmissionBatch(ctx context.Context, scope accountsync.Scope, submission accountsend.Submission) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderSubmissions, store.documents.readAccess(scope.Owner))
	if err != nil {
		return accountsend.Submission{}, err
	}
	for _, raw := range documents {
		current, err := decodeSubmissionRecord(raw)
		if err != nil {
			return accountsend.Submission{}, err
		}
		if current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) {
			continue
		}
		if current.ID == submission.ID || current.IdempotencyKey == submission.IdempotencyKey {
			return current.submission(scope, nil), nil
		}
	}
	if submission.State != accountsend.StateBuilding || len(submission.Recipients) < 1 {
		return accountsend.Submission{}, accountsend.ErrInvalidSubmission
	}
	if _, err := store.documents.put(ctx, ProviderSubmissions, newSubmissionRecord(scope, submission, ""), store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	return submission, nil
}

func (store *AccountSync) AppendSubmissionDelivery(ctx context.Context, scope accountsync.Scope, request accountsend.AppendRequest) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, request.ID)
	if err != nil {
		return accountsend.Submission{}, err
	}
	digest := sha256.Sum256(request.RawRFC822)
	digestText := hex.EncodeToString(digest[:])
	if request.Index < len(current.DeliveryRecipients) {
		if request.Index < len(current.DeliveryDigests) && current.DeliveryRecipients[request.Index] == request.Recipient && current.DeliveryDigests[request.Index] == digestText {
			return current.submission(scope, nil), nil
		}
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	if current.State != accountsend.StateBuilding || current.Revision != request.Revision || request.Index != len(current.DeliveryRecipients) ||
		request.Index >= len(current.Recipients) || current.Recipients[request.Index] != request.Recipient {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	payload := accountsend.Submission{ID: current.ID, RawRFC822: request.RawRFC822}
	rawID, err := store.putSubmissionRaw(ctx, scope, payload)
	if err != nil {
		return accountsend.Submission{}, err
	}
	next := current
	next.DeliveryRecipients = append(append([]string(nil), current.DeliveryRecipients...), request.Recipient)
	next.DeliveryRawBucketIDs = append(append([]string(nil), current.DeliveryRawBucketIDs...), rawID)
	next.DeliveryDigests = append(append([]string(nil), current.DeliveryDigests...), digestText)
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, access); err != nil {
		cleanup := store.documents.delete(ctx, ProviderSubmitFiles, rawID, access)
		return accountsend.Submission{}, errors.Join(err, cleanup)
	}
	return next.submission(scope, nil), nil
}

func (store *AccountSync) CommitSubmissionBatch(ctx context.Context, scope accountsync.Scope, id string, expectedRevision uint64) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, id)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State == accountsend.StateQueued && current.Revision == expectedRevision+1 && len(current.DeliveryRecipients) == len(current.Recipients) {
		return current.submission(scope, nil), nil
	}
	if current.State != accountsend.StateBuilding || current.Revision != expectedRevision || len(current.DeliveryRecipients) != len(current.Recipients) ||
		len(current.DeliveryRawBucketIDs) != len(current.Recipients) || len(current.DeliveryDigests) != len(current.Recipients) {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	next := current
	next.State = accountsend.StateQueued
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	return next.submission(scope, nil), nil
}

// BeginSubmissionBatchReplacement moves an undelivered batch into a durable
// editing state. Active delivery buckets remain authoritative until commit.
func (store *AccountSync) BeginSubmissionBatchReplacement(ctx context.Context, scope accountsync.Scope, request accountsend.BatchRequest, expectedRevision uint64) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, request.ID)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State == accountsend.StateEditing && current.Revision == expectedRevision+1 &&
		current.IdempotencyKey == request.IdempotencyKey && current.ReplacementEnvelopeFrom == request.EnvelopeFrom &&
		equalStrings(current.ReplacementRecipients, request.Recipients) && current.ReplacementNotBefore == timeText(request.NotBefore) {
		return current.submission(scope, nil), nil
	}
	if current.State != accountsend.StateQueued || current.Revision != expectedRevision ||
		current.IdempotencyKey != request.IdempotencyKey || len(current.DeliveryRawBucketIDs) < 1 {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	next := current
	next.State = accountsend.StateEditing
	next.Revision++
	next.ReplacementEnvelopeFrom = request.EnvelopeFrom
	next.ReplacementRecipients = append([]string(nil), request.Recipients...)
	next.ReplacementRawBucketIDs = []string{}
	next.ReplacementDigests = []string{}
	next.ReplacementNotBefore = timeText(request.NotBefore)
	next.UpdatedAt = time.Now().UTC()
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	return next.submission(scope, nil), nil
}

func (store *AccountSync) AppendSubmissionReplacementDelivery(ctx context.Context, scope accountsync.Scope, request accountsend.AppendRequest) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, request.ID)
	if err != nil {
		return accountsend.Submission{}, err
	}
	digest := sha256.Sum256(request.RawRFC822)
	digestText := hex.EncodeToString(digest[:])
	if request.Index < len(current.ReplacementRawBucketIDs) {
		if request.Index < len(current.ReplacementRecipients) && request.Index < len(current.ReplacementDigests) &&
			current.ReplacementRecipients[request.Index] == request.Recipient && current.ReplacementDigests[request.Index] == digestText {
			return current.submission(scope, nil), nil
		}
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	if current.State != accountsend.StateEditing || current.Revision != request.Revision ||
		request.Index != len(current.ReplacementRawBucketIDs) || request.Index >= len(current.ReplacementRecipients) ||
		current.ReplacementRecipients[request.Index] != request.Recipient {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	rawID, err := store.putSubmissionRaw(ctx, scope, accountsend.Submission{ID: current.ID, RawRFC822: request.RawRFC822})
	if err != nil {
		return accountsend.Submission{}, err
	}
	next := current
	next.ReplacementRawBucketIDs = append(append([]string(nil), current.ReplacementRawBucketIDs...), rawID)
	next.ReplacementDigests = append(append([]string(nil), current.ReplacementDigests...), digestText)
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, access); err != nil {
		cleanup := store.documents.delete(ctx, ProviderSubmitFiles, rawID, access)
		return accountsend.Submission{}, errors.Join(err, cleanup)
	}
	return next.submission(scope, nil), nil
}

func (store *AccountSync) CommitSubmissionBatchReplacement(ctx context.Context, scope accountsync.Scope, id string, expectedRevision uint64) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, id)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State == accountsend.StateQueued && current.Revision == expectedRevision+1 {
		return current.submission(scope, nil), nil
	}
	if current.State != accountsend.StateEditing || current.Revision != expectedRevision ||
		len(current.ReplacementRecipients) < 1 || len(current.ReplacementRecipients) != len(current.ReplacementRawBucketIDs) ||
		len(current.ReplacementRecipients) != len(current.ReplacementDigests) {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	notBefore, err := parseTimeText(current.ReplacementNotBefore)
	if err != nil {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	oldRawIDs := append([]string(nil), current.DeliveryRawBucketIDs...)
	next := current
	next.EnvelopeFrom = current.ReplacementEnvelopeFrom
	next.Recipients = append([]string(nil), current.ReplacementRecipients...)
	next.DeliveryRecipients = append([]string(nil), current.ReplacementRecipients...)
	next.DeliveryRawBucketIDs = append([]string(nil), current.ReplacementRawBucketIDs...)
	next.DeliveryDigests = append([]string(nil), current.ReplacementDigests...)
	next.NotBefore = notBefore
	next.State = accountsend.StateQueued
	next.Attempts = 0
	next.LastCode = ""
	next.Revision++
	next.clearReplacement()
	next.UpdatedAt = time.Now().UTC()
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	_ = store.deleteRawIDs(ctx, scope, oldRawIDs)
	return next.submission(scope, nil), nil
}

func (store *AccountSync) AbortSubmissionBatchReplacement(ctx context.Context, scope accountsync.Scope, id string, expectedRevision uint64) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, id)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State == accountsend.StateQueued && current.Revision == expectedRevision+1 {
		return current.submission(scope, nil), nil
	}
	if current.State != accountsend.StateEditing || current.Revision != expectedRevision {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	stagedRawIDs := append([]string(nil), current.ReplacementRawBucketIDs...)
	next := current
	next.State = accountsend.StateQueued
	next.Revision++
	next.clearReplacement()
	next.UpdatedAt = time.Now().UTC()
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	_ = store.deleteRawIDs(ctx, scope, stagedRawIDs)
	return next.submission(scope, nil), nil
}

// ReplaceSubmission changes only a still-queued journal entry. The new bucket
// is published before the document CAS and the old bucket is removed only
// after that CAS succeeds, so the durable row always points at readable MIME.
func (store *AccountSync) ReplaceSubmission(ctx context.Context, scope accountsync.Scope, request accountsend.Request, expectedRevision uint64) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, request.ID)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State != accountsend.StateQueued || current.Revision != expectedRevision || current.IdempotencyKey != request.IdempotencyKey || len(current.DeliveryRawBucketIDs) != 0 {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	now := time.Now().UTC()
	replacement := current.submission(scope, request.RawRFC822)
	replacement.EnvelopeFrom = request.EnvelopeFrom
	replacement.Recipients = append([]string(nil), request.Recipients...)
	replacement.NotBefore = request.NotBefore
	replacement.Attempts = 0
	replacement.LastCode = ""
	replacement.Revision = current.Revision + 1
	replacement.UpdatedAt = now
	newRawID, err := store.putSubmissionRaw(ctx, scope, replacement)
	if err != nil {
		return accountsend.Submission{}, err
	}
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	next := newSubmissionRecord(scope, replacement, newRawID)
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, access); err != nil {
		cleanup := store.documents.delete(ctx, ProviderSubmitFiles, newRawID, access)
		return accountsend.Submission{}, errors.Join(err, cleanup)
	}
	if current.RawBucketID != "" {
		if err := store.documents.delete(ctx, ProviderSubmitFiles, current.RawBucketID, access); err != nil {
			// The replacement is already the durable authoritative source. A stale
			// bucket cleanup failure must not make an accepted edit look rejected
			// and invite a conflicting retry of the same revision.
			return replacement, nil
		}
	}
	return replacement, nil
}

func (store *AccountSync) PendingSubmissions(ctx context.Context, lease accountsync.Lease, limit int) ([]accountsend.Submission, error) {
	if limit < 1 {
		return nil, nil
	}
	if limit > accountsend.MaxPending {
		limit = accountsend.MaxPending
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	if err := store.requireLease(ctx, lease); err != nil {
		return nil, err
	}
	documents, err := store.documents.find(ctx, ProviderSubmissions, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return nil, err
	}
	result := make([]accountsend.Submission, 0, limit)
	for documentID, raw := range documents {
		record, err := decodeSubmissionRecord(raw)
		if err != nil {
			return nil, err
		}
		if record.AccountID != lease.Scope.AccountID || !recordOwner(record.TenantID, record.PrincipalID, lease.Scope) {
			continue
		}
		if record.State == accountsend.StateEditing {
			if record.UpdatedAt.After(time.Now().UTC().Add(-submissionEditTimeout)) {
				continue
			}
			if err := store.deleteRawIDs(ctx, lease.Scope, record.ReplacementRawBucketIDs); err != nil {
				return nil, err
			}
			next := record
			next.State = accountsend.StateQueued
			next.Revision++
			next.clearReplacement()
			next.UpdatedAt = time.Now().UTC()
			if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, record, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID)); err != nil {
				return nil, err
			}
			record = next
		}
		if record.State != accountsend.StateQueued && record.State != accountsend.StateApplying {
			continue
		}
		var payload []byte
		if record.RawBucketID != "" {
			payload, err = store.getSubmissionRaw(ctx, lease.Scope, record.RawBucketID)
			if err != nil {
				return nil, err
			}
		}
		submission := record.submission(lease.Scope, payload)
		for index, rawID := range record.DeliveryRawBucketIDs {
			deliveryRaw, err := store.getSubmissionRaw(ctx, lease.Scope, rawID)
			if err != nil {
				return nil, err
			}
			submission.Deliveries = append(submission.Deliveries, accountsend.Delivery{Recipient: record.DeliveryRecipients[index], RawRFC822: deliveryRaw})
		}
		result = append(result, submission)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (store *AccountSync) UpdateSubmission(ctx context.Context, lease accountsync.Lease, replacement accountsend.Submission) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	if err := store.requireLease(ctx, lease); err != nil {
		return err
	}
	documentID, current, err := store.findSubmission(ctx, lease.Scope, replacement.ID)
	if err != nil {
		return err
	}
	if !validSubmissionTransition(current.State, replacement.State) || replacement.Revision != current.Revision+1 || current.IdempotencyKey != replacement.IdempotencyKey ||
		current.EnvelopeFrom != replacement.EnvelopeFrom || !equalStrings(current.Recipients, replacement.Recipients) {
		return accountsend.ErrSubmissionConflict
	}
	next := newSubmissionRecord(lease.Scope, replacement, current.RawBucketID)
	next.DeliveryRecipients = append([]string(nil), current.DeliveryRecipients...)
	next.DeliveryRawBucketIDs = append([]string(nil), current.DeliveryRawBucketIDs...)
	next.DeliveryDigests = append([]string(nil), current.DeliveryDigests...)
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID)); err != nil {
		return err
	}
	if replacement.State == accountsend.StateSent {
		return store.deleteSubmissionRaw(ctx, lease.Scope, current)
	}
	return nil
}

func (store *AccountSync) CancelSubmission(ctx context.Context, scope accountsync.Scope, id string) (accountsend.Submission, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.findSubmission(ctx, scope, id)
	if err != nil {
		return accountsend.Submission{}, err
	}
	if current.State != accountsend.StateQueued && current.State != accountsend.StateBuilding && current.State != accountsend.StateEditing {
		return accountsend.Submission{}, accountsend.ErrSubmissionConflict
	}
	next := current
	next.State, next.LastCode, next.UpdatedAt = accountsend.StateCancelled, "", time.Now().UTC()
	next.Revision++
	if err := store.documents.patch(ctx, ProviderSubmissions, documentID, next, current, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return accountsend.Submission{}, err
	}
	if err := store.deleteSubmissionRaw(ctx, scope, current); err != nil {
		return accountsend.Submission{}, err
	}
	return next.submission(scope, nil), nil
}

func (store *AccountSync) GetSubmission(ctx context.Context, scope accountsync.Scope, id string) (accountsend.Submission, error) {
	_, record, err := store.findSubmission(ctx, scope, id)
	if err != nil {
		return accountsend.Submission{}, err
	}
	return record.submission(scope, nil), nil
}

func (store *AccountSync) requireLease(ctx context.Context, lease accountsync.Lease) error {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return accountsync.ErrLeaseHeld
	}
	return nil
}

func (store *AccountSync) findSubmission(ctx context.Context, scope accountsync.Scope, id string) (string, submissionRecord, error) {
	documents, err := store.documents.find(ctx, ProviderSubmissions, store.documents.readAccess(scope.Owner))
	if err != nil {
		return "", submissionRecord{}, err
	}
	for documentID, raw := range documents {
		record, err := decodeSubmissionRecord(raw)
		if err != nil {
			return "", submissionRecord{}, err
		}
		if record.ID == id && record.AccountID == scope.AccountID && recordOwner(record.TenantID, record.PrincipalID, scope) {
			return documentID, record, nil
		}
	}
	return "", submissionRecord{}, accounts.ErrNotFound
}

func (store *AccountSync) putSubmissionRaw(ctx context.Context, scope accountsync.Scope, submission accountsend.Submission) (string, error) {
	file, err := os.CreateTemp(store.stagingRoot, "provider-submission-*.eml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return "", err
	}
	if _, err := file.Write(submission.RawRFC822); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	request := map[string]any{"op": "putData", "collection": ProviderSubmitFiles, "file": map[string]any{"path": path},
		"fileOptions": map[string]any{"maxBytes": accountsend.MaxRawBytes}, "meta": map[string]any{"accountId": scope.AccountID, "submissionId": submission.ID}}
	addAccess(request, store.documents.writeAccess(scope.Owner, scope.AccountID))
	result, err := store.documents.call(ctx, request)
	if err != nil {
		return "", err
	}
	var id string
	if err := json.Unmarshal(result, &id); err != nil || id == "" {
		return "", fmt.Errorf("decode FYLO provider submission raw identifier: %w", err)
	}
	return id, nil
}

func (store *AccountSync) getSubmissionRaw(ctx context.Context, scope accountsync.Scope, id string) ([]byte, error) {
	request := map[string]any{"op": "getFileData", "collection": ProviderSubmitFiles, "id": id}
	addAccess(request, store.documents.readAccess(scope.Owner))
	result, err := store.documents.call(ctx, request)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data          string `json:"data"`
		ContentLength int    `json:"contentLength"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.Data == "" || payload.ContentLength < 1 || payload.ContentLength > accountsend.MaxRawBytes {
		return nil, errors.New("invalid FYLO provider submission payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil || len(decoded) != payload.ContentLength {
		return nil, errors.New("invalid FYLO provider submission encoding")
	}
	return decoded, nil
}

func (store *AccountSync) deleteSubmissionRecords(ctx context.Context, scope accountsync.Scope) error {
	documents, err := store.documents.find(ctx, ProviderSubmissions, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		record, err := decodeSubmissionRecord(raw)
		if err != nil {
			return err
		}
		if record.AccountID != scope.AccountID || !recordOwner(record.TenantID, record.PrincipalID, scope) {
			continue
		}
		access := store.documents.writeAccess(scope.Owner, scope.AccountID)
		if err := store.documents.delete(ctx, ProviderSubmissions, id, access); err != nil {
			return err
		}
		// FYLO patchDoc is a shallow merge. Terminal records intentionally retain
		// the opaque bucket id after their raw file is erased, because omitting an
		// optional field cannot clear it. Only non-terminal records can still own
		// live raw data at account-cleanup time.
		if record.State != accountsend.StateSent && record.State != accountsend.StateCancelled {
			if err := store.deleteSubmissionRaw(ctx, scope, record); err != nil {
				return err
			}
		}
	}
	return nil
}

type submissionRecord struct {
	ID                      string             `json:"id"`
	IdempotencyKey          string             `json:"idempotencyKey"`
	Revision                uint64             `json:"revision"`
	AccountID               accounts.AccountID `json:"accountId"`
	TenantID                string             `json:"tenantId"`
	PrincipalID             string             `json:"principalId"`
	OwnerAddress            string             `json:"ownerAddress"`
	EnvelopeFrom            string             `json:"envelopeFrom"`
	Recipients              []string           `json:"recipients"`
	RawBucketID             string             `json:"rawBucketId,omitempty"`
	DeliveryRecipients      []string           `json:"deliveryRecipients,omitempty"`
	DeliveryRawBucketIDs    []string           `json:"deliveryRawBucketIds,omitempty"`
	DeliveryDigests         []string           `json:"deliveryDigests,omitempty"`
	ReplacementEnvelopeFrom string             `json:"replacementEnvelopeFrom"`
	ReplacementRecipients   []string           `json:"replacementRecipients"`
	ReplacementRawBucketIDs []string           `json:"replacementRawBucketIds"`
	ReplacementDigests      []string           `json:"replacementDigests"`
	ReplacementNotBefore    string             `json:"replacementNotBefore"`
	State                   accountsend.State  `json:"state"`
	Attempts                uint16             `json:"attempts"`
	NotBefore               *time.Time         `json:"notBefore,omitempty"`
	LastCode                string             `json:"lastCode,omitempty"`
	CreatedAt               time.Time          `json:"createdAt"`
	UpdatedAt               time.Time          `json:"updatedAt"`
}

func newSubmissionRecord(scope accountsync.Scope, submission accountsend.Submission, rawID string) submissionRecord {
	return submissionRecord{ID: submission.ID, IdempotencyKey: submission.IdempotencyKey, AccountID: scope.AccountID, TenantID: scope.Owner.TenantID,
		PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address, Revision: submission.Revision, EnvelopeFrom: submission.EnvelopeFrom, Recipients: append([]string(nil), submission.Recipients...),
		RawBucketID: rawID, DeliveryRecipients: []string{}, DeliveryRawBucketIDs: []string{}, DeliveryDigests: []string{},
		ReplacementRecipients: []string{}, ReplacementRawBucketIDs: []string{}, ReplacementDigests: []string{},
		State: submission.State, Attempts: submission.Attempts, NotBefore: submission.NotBefore, LastCode: submission.LastCode,
		CreatedAt: submission.CreatedAt, UpdatedAt: submission.UpdatedAt}
}

func decodeSubmissionRecord(raw json.RawMessage) (submissionRecord, error) {
	var record submissionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return submissionRecord{}, fmt.Errorf("decode FYLO provider submission: %w", err)
	}
	return record, nil
}

func (record submissionRecord) submission(scope accountsync.Scope, payload []byte) accountsend.Submission {
	deliveryCount := len(record.DeliveryRecipients)
	if record.State == accountsend.StateEditing {
		deliveryCount = len(record.ReplacementRawBucketIDs)
	}
	return accountsend.Submission{ID: record.ID, IdempotencyKey: record.IdempotencyKey, Revision: record.Revision, Scope: scope, EnvelopeFrom: record.EnvelopeFrom,
		Recipients: append([]string(nil), record.Recipients...), RawRFC822: payload, State: record.State, Attempts: record.Attempts,
		DeliveryCount: deliveryCount, NotBefore: record.NotBefore, LastCode: record.LastCode, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func (store *AccountSync) deleteSubmissionRaw(ctx context.Context, scope accountsync.Scope, record submissionRecord) error {
	ids := append([]string(nil), record.DeliveryRawBucketIDs...)
	ids = append(ids, record.ReplacementRawBucketIDs...)
	if record.RawBucketID != "" {
		ids = append(ids, record.RawBucketID)
	}
	return store.deleteRawIDs(ctx, scope, ids)
}

func (store *AccountSync) deleteRawIDs(ctx context.Context, scope accountsync.Scope, ids []string) error {
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	for _, id := range ids {
		if err := store.documents.delete(ctx, ProviderSubmitFiles, id, access); err != nil {
			return err
		}
	}
	return nil
}

func (record *submissionRecord) clearReplacement() {
	record.ReplacementEnvelopeFrom = ""
	record.ReplacementRecipients = []string{}
	record.ReplacementRawBucketIDs = []string{}
	record.ReplacementDigests = []string{}
	record.ReplacementNotBefore = ""
}

func timeText(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTimeText(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func validSubmissionTransition(current, next accountsend.State) bool {
	if current == next {
		return true
	}
	if current == accountsend.StateQueued {
		return next == accountsend.StateApplying || next == accountsend.StateCancelled
	}
	if current == accountsend.StateApplying {
		return next == accountsend.StateQueued || next == accountsend.StateSent || next == accountsend.StateFailed || next == accountsend.StateAmbiguous
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
