package accountsend

import (
	"context"
	"errors"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

type WorkerOptions struct {
	Clock func() time.Time
}

type Worker struct {
	store      Store
	connectors map[accounts.Provider]Connector
	clock      func() time.Time
}

func NewWorker(store Store, connectors map[accounts.Provider]Connector, options WorkerOptions) *Worker {
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Worker{store: store, connectors: connectors, clock: clock}
}

func (worker *Worker) Enqueue(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request Request) (Submission, error) {
	if worker == nil || worker.store == nil || worker.clock == nil {
		return Submission{}, ErrInvalidSubmission
	}
	now := worker.clock().UTC()
	if ValidateRequest(request, now) != nil || account.Owner.TenantID != owner.TenantID ||
		account.Owner.PrincipalID != owner.PrincipalID || request.EnvelopeFrom != account.MailboxAddress || !account.Capabilities.Send || !sendable(account.State) {
		return Submission{}, ErrInvalidSubmission
	}
	submission := Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey,
		Revision: 1,
		Scope:    accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: request.EnvelopeFrom,
		Recipients: append([]string(nil), request.Recipients...), RawRFC822: append([]byte(nil), request.RawRFC822...),
		State: StateQueued, NotBefore: request.NotBefore, CreatedAt: now, UpdatedAt: now}
	return worker.store.EnqueueSubmission(ctx, submission.Scope, submission)
}

func (worker *Worker) BeginBatch(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request BatchRequest) (Submission, error) {
	if worker == nil || worker.store == nil || worker.clock == nil {
		return Submission{}, ErrInvalidSubmission
	}
	now := worker.clock().UTC()
	if ValidateBatchRequest(request, now) != nil || !worker.authorizedSender(owner, account, request.EnvelopeFrom) {
		return Submission{}, ErrInvalidSubmission
	}
	submission := Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Revision: 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: request.EnvelopeFrom,
		Recipients: append([]string(nil), request.Recipients...), State: StateBuilding, NotBefore: request.NotBefore,
		CreatedAt: now, UpdatedAt: now}
	return worker.store.BeginSubmissionBatch(ctx, submission.Scope, submission)
}

func (worker *Worker) AppendBatch(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request AppendRequest) (Submission, error) {
	if worker == nil || worker.store == nil || ValidateAppendRequest(request) != nil || !worker.authorizedSender(owner, account, account.MailboxAddress) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.AppendSubmissionDelivery(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request)
}

func (worker *Worker) CommitBatch(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (Submission, error) {
	if worker == nil || worker.store == nil || !submissionID.MatchString(id) || revision < 1 || !worker.authorizedSender(owner, account, account.MailboxAddress) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.CommitSubmissionBatch(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, id, revision)
}

func (worker *Worker) BeginBatchReplace(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request BatchRequest, expectedRevision uint64) (Submission, error) {
	if worker == nil || worker.store == nil || worker.clock == nil || expectedRevision < 1 ||
		ValidateBatchRequest(request, worker.clock().UTC()) != nil || !worker.authorizedSender(owner, account, request.EnvelopeFrom) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.BeginSubmissionBatchReplacement(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request, expectedRevision)
}

func (worker *Worker) AppendBatchReplace(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request AppendRequest) (Submission, error) {
	if worker == nil || worker.store == nil || ValidateAppendRequest(request) != nil || !worker.authorizedSender(owner, account, account.MailboxAddress) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.AppendSubmissionReplacementDelivery(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request)
}

func (worker *Worker) CommitBatchReplace(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (Submission, error) {
	if worker == nil || worker.store == nil || !submissionID.MatchString(id) || revision < 1 || !worker.authorizedSender(owner, account, account.MailboxAddress) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.CommitSubmissionBatchReplacement(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, id, revision)
}

func (worker *Worker) AbortBatchReplace(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (Submission, error) {
	if worker == nil || worker.store == nil || !submissionID.MatchString(id) || revision < 1 || !worker.authorizedSender(owner, account, account.MailboxAddress) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.AbortSubmissionBatchReplacement(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, id, revision)
}

// Replace atomically swaps the MIME source and schedule of a submission that
// has not started delivery. The revision is a fail-closed CAS boundary between
// browser edits and the scheduler-owned worker.
func (worker *Worker) Replace(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request Request, expectedRevision uint64) (Submission, error) {
	if worker == nil || worker.store == nil || worker.clock == nil || expectedRevision < 1 {
		return Submission{}, ErrInvalidSubmission
	}
	now := worker.clock().UTC()
	if ValidateRequest(request, now) != nil || account.Owner.TenantID != owner.TenantID ||
		account.Owner.PrincipalID != owner.PrincipalID || request.EnvelopeFrom != account.MailboxAddress || !account.Capabilities.Send || !sendable(account.State) {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.ReplaceSubmission(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request, expectedRevision)
}

func (worker *Worker) Cancel(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string) (Submission, error) {
	if worker == nil || worker.store == nil || !submissionID.MatchString(id) || account.Owner.TenantID != owner.TenantID || account.Owner.PrincipalID != owner.PrincipalID {
		return Submission{}, ErrInvalidSubmission
	}
	return worker.store.CancelSubmission(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, id)
}

func (worker *Worker) authorizedSender(owner accounts.Owner, account accounts.ExternalAccount, envelopeFrom string) bool {
	return account.Owner.TenantID == owner.TenantID && account.Owner.PrincipalID == owner.PrincipalID &&
		envelopeFrom == account.MailboxAddress && account.Capabilities.Send && sendable(account.State)
}

// RunWithLease is called by the account scheduler after it acquires the same
// per-account lease used for remote mailbox mutations. A record left in
// applying after a crash is made explicitly ambiguous and is never sent twice
// automatically.
func (worker *Worker) RunWithLease(ctx context.Context, lease accountsync.Lease, account accounts.ExternalAccount, credentials accounts.Credentials) error {
	if worker == nil || worker.store == nil || worker.clock == nil {
		return accounts.ErrUnsupported
	}
	connector := worker.connectors[account.Provider]
	if connector == nil {
		return accounts.ErrUnsupported
	}
	pending, err := worker.store.PendingSubmissions(ctx, lease, MaxPending)
	if err != nil {
		return err
	}
	for index := range pending {
		submission := pending[index]
		now := worker.clock().UTC()
		if submission.NotBefore != nil && submission.NotBefore.After(now) {
			continue
		}
		if submission.State == StateApplying {
			submission.State, submission.LastCode, submission.UpdatedAt = StateAmbiguous, "worker_restarted", now
			submission.Revision++
			if err := worker.store.UpdateSubmission(context.WithoutCancel(ctx), lease, submission); err != nil {
				return err
			}
			continue
		}
		submission.State, submission.UpdatedAt = StateApplying, now
		submission.Attempts++
		submission.Revision++
		if err := worker.store.UpdateSubmission(ctx, lease, submission); err != nil {
			return err
		}
		result, submitErr := submitSubmission(ctx, connector, account, credentials, submission)
		if submitErr != nil {
			submission.State, submission.LastCode = StateQueued, "transport_failed"
			notBefore := now.Add(time.Minute)
			submission.NotBefore = &notBefore
			submission.UpdatedAt = worker.clock().UTC()
			submission.Revision++
			if updateErr := worker.store.UpdateSubmission(context.WithoutCancel(ctx), lease, submission); updateErr != nil {
				return errors.Join(submitErr, updateErr)
			}
			return submitErr
		}
		submission.UpdatedAt = worker.clock().UTC()
		submission.LastCode = safeCode(result.Code)
		switch result.Outcome {
		case OutcomeSent:
			submission.State, submission.LastCode, submission.NotBefore = StateSent, "", nil
		case OutcomeRetry:
			submission.State = StateQueued
			delay := result.RetryAfter
			if delay <= 0 {
				delay = time.Minute
			}
			notBefore := submission.UpdatedAt.Add(delay)
			submission.NotBefore = &notBefore
		case OutcomeRejected:
			submission.State = StateFailed
		case OutcomeAmbiguous:
			submission.State = StateAmbiguous
		default:
			return ErrInvalidSubmission
		}
		submission.Revision++
		if err := worker.store.UpdateSubmission(context.WithoutCancel(ctx), lease, submission); err != nil {
			return err
		}
	}
	return nil
}

func submitSubmission(ctx context.Context, connector Connector, account accounts.ExternalAccount, credentials accounts.Credentials, submission Submission) (Result, error) {
	if len(submission.Deliveries) == 0 {
		return connector.Submit(ctx, Context{Account: account, Credentials: credentials, Submission: submission})
	}
	sent := 0
	for _, delivery := range submission.Deliveries {
		child := submission
		child.Recipients = []string{delivery.Recipient}
		child.RawRFC822 = delivery.RawRFC822
		child.Deliveries = nil
		result, err := connector.Submit(ctx, Context{Account: account, Credentials: credentials, Submission: child})
		if err != nil {
			if sent > 0 {
				return Result{Outcome: OutcomeAmbiguous, Code: "batch_partial_delivery"}, nil
			}
			return Result{}, err
		}
		if result.Outcome != OutcomeSent {
			if sent > 0 {
				return Result{Outcome: OutcomeAmbiguous, Code: "batch_partial_delivery"}, nil
			}
			return result, nil
		}
		sent++
	}
	return Result{Outcome: OutcomeSent}, nil
}

func sendable(state accounts.State) bool {
	return state == accounts.StateReady || state == accounts.StateSyncing || state == accounts.StateIdle || state == accounts.StateDegraded
}

func safeCode(value string) string {
	if value == "" || len(value) > 63 {
		return "submission_failed"
	}
	for _, character := range value {
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return "submission_failed"
		}
	}
	return value
}
