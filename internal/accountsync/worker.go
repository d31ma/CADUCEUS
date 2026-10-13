package accountsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

type WorkerOptions struct {
	Clock    func() time.Time
	LeaseTTL time.Duration
	Holder   string
	Jobs     []AccountJob
	SyncJobs []AccountJob
}

type Worker struct {
	store      Store
	lifecycle  Lifecycle
	secrets    CredentialsSource
	connectors map[accounts.Provider]Connector
	clock      func() time.Time
	leaseTTL   time.Duration
	holder     string
	jobs       []AccountJob
	syncJobs   []AccountJob
}

func NewWorker(store Store, lifecycle Lifecycle, secrets CredentialsSource, connectors map[accounts.Provider]Connector, options WorkerOptions) *Worker {
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	ttl := options.LeaseTTL
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	holder := options.Holder
	if holder == "" {
		holder = "caduceus"
	}
	return &Worker{store: store, lifecycle: lifecycle, secrets: secrets, connectors: connectors, clock: clock, leaseTTL: ttl, holder: holder,
		jobs: append([]AccountJob(nil), options.Jobs...), syncJobs: append([]AccountJob(nil), options.SyncJobs...)}
}

func (worker *Worker) RunAccount(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount) error {
	if worker == nil || worker.store == nil || worker.lifecycle == nil || worker.secrets == nil {
		return errors.New("external account sync worker is not configured")
	}
	connector := worker.connectors[account.Provider]
	if connector == nil {
		return accounts.ErrUnsupported
	}
	scope := Scope{Owner: owner, AccountID: account.ID}
	lease, err := worker.store.AcquireLease(ctx, scope, worker.holder, worker.clock().UTC(), worker.leaseTTL)
	if err != nil {
		return err
	}
	defer worker.store.ReleaseLease(context.WithoutCancel(ctx), lease) // best-effort; expiry remains the crash boundary

	current, err := worker.lifecycle.BeginSync(ctx, owner, account.ID, account.Revision)
	if err != nil {
		return err
	}
	credentials, err := worker.secrets.Credentials(ctx, current)
	if err != nil {
		return worker.fail(ctx, current, err)
	}
	defer clearCredentials(&credentials)
	checkpoint, err := worker.store.Checkpoint(ctx, scope)
	if err != nil {
		return worker.fail(ctx, current, err)
	}
	sink := &sessionSink{worker: worker, lease: lease, account: current, checkpoint: checkpoint}
	err = connector.Sync(ctx, SyncRequest{Account: current, Credentials: credentials, Checkpoint: checkpoint}, sink)
	if err != nil {
		return worker.fail(ctx, sink.account, err)
	}
	for _, job := range worker.syncJobs {
		if job == nil {
			continue
		}
		if err := job.RunWithLease(ctx, lease, sink.account, credentials); err != nil {
			return worker.fail(ctx, sink.account, err)
		}
	}
	_, err = worker.lifecycle.CompleteSync(ctx, owner, account.ID, sink.account.Revision)
	return err
}

type sessionSink struct {
	worker     *Worker
	lease      Lease
	account    accounts.ExternalAccount
	checkpoint Checkpoint
}

func (sink *sessionSink) Commit(ctx context.Context, batch Batch) error {
	if err := ValidateBatch(batch); err != nil {
		return err
	}
	if batch.CursorBefore != sink.checkpoint.Cursor {
		return ErrCheckpointConflict
	}
	checkpoint, err := sink.worker.store.CommitBatch(ctx, sink.lease, sink.checkpoint.Version, batch)
	if err != nil {
		return err
	}
	updated, err := sink.worker.lifecycle.ReportProgress(ctx, sink.account.Owner, sink.account.ID, sink.account.Revision, accounts.SyncProgressUpdate{
		Phase: batch.Phase, Completed: batch.Completed, Total: batch.Total,
	})
	if err != nil {
		return err
	}
	sink.checkpoint, sink.account = checkpoint, updated
	return nil
}

func (worker *Worker) fail(ctx context.Context, account accounts.ExternalAccount, cause error) error {
	failure := accounts.SyncFailure{Code: "sync_failed", Message: "External account synchronization failed.", Retryable: true}
	var connector ConnectorError
	if errors.As(cause, &connector) {
		failure.Code = connector.Code
		failure.Message = connector.Message
		failure.Authentication = connector.Kind == FailureAuthentication
		failure.Retryable = connector.Kind == FailureTransient || connector.Kind == FailureThrottled
		if failure.Retryable {
			delay := connector.RetryAfter
			if delay <= 0 {
				delay = time.Minute
			}
			retryAt := worker.clock().UTC().Add(delay)
			failure.RetryAt = &retryAt
		}
	}
	_, recordErr := worker.lifecycle.RecordFailure(context.WithoutCancel(ctx), account.Owner, account.ID, account.Revision, failure)
	return errors.Join(cause, recordErr)
}

func (worker *Worker) EnqueueMutation(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, request MutationRequest) (Mutation, error) {
	if err := ValidateMutation(request); err != nil {
		return Mutation{}, err
	}
	now := worker.clock().UTC()
	mutation := Mutation{MutationRequest: request, Scope: Scope{Owner: owner, AccountID: account.ID}, State: MutationQueued, CreatedAt: now, UpdatedAt: now}
	return worker.store.EnqueueMutation(ctx, mutation.Scope, mutation)
}

func (worker *Worker) MutationByIdempotencyKey(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, key string) (Mutation, bool, error) {
	return worker.store.MutationByIdempotencyKey(ctx, Scope{Owner: owner, AccountID: account.ID}, key)
}

// Mutation returns one exact-owner journal entry without exposing provider
// credentials or allowing caller-controlled account filters to widen scope.
func (worker *Worker) Mutation(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string) (Mutation, error) {
	return worker.store.Mutation(ctx, Scope{Owner: owner, AccountID: account.ID}, id)
}

func (worker *Worker) RunMutations(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount) error {
	connector := worker.connectors[account.Provider]
	if connector == nil {
		return accounts.ErrUnsupported
	}
	scope := Scope{Owner: owner, AccountID: account.ID}
	lease, err := worker.store.AcquireLease(ctx, scope, worker.holder, worker.clock().UTC(), worker.leaseTTL)
	if err != nil {
		return err
	}
	defer worker.store.ReleaseLease(context.WithoutCancel(ctx), lease)
	credentials, err := worker.secrets.Credentials(ctx, account)
	if err != nil {
		return err
	}
	defer clearCredentials(&credentials)
	for _, job := range worker.jobs {
		if job == nil {
			continue
		}
		if err := job.RunWithLease(ctx, lease, account, credentials); err != nil {
			return err
		}
	}
	mutations, err := worker.store.PendingMutations(ctx, lease, 50)
	if err != nil {
		return err
	}
	for index := range mutations {
		mutation := mutations[index]
		if mutation.NotBefore != nil && mutation.NotBefore.After(worker.clock().UTC()) {
			continue
		}
		mutation.Attempts++
		mutation.UpdatedAt = worker.clock().UTC()
		context := MutationContext{Account: account, Credentials: credentials, Mutation: mutation}
		var result MutationResult
		if mutation.State == MutationReconciling || mutation.State == MutationApplying {
			mutation.State = MutationReconciling
			if err = worker.store.UpdateMutation(ctx, lease, mutation); err != nil {
				return err
			}
			context.Mutation = mutation
			result, err = connector.ReconcileMutation(ctx, context)
		} else {
			mutation.State = MutationApplying
			if err = worker.store.UpdateMutation(ctx, lease, mutation); err != nil {
				return err
			}
			context.Mutation = mutation
			result, err = connector.ApplyMutation(ctx, context)
		}
		if err != nil {
			if mutation.State != MutationReconciling {
				mutation.State = MutationQueued
			}
			mutation.LastCode = "remote_mutation_failed"
			notBefore := worker.clock().UTC().Add(time.Minute)
			mutation.NotBefore = &notBefore
			if updateErr := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); updateErr != nil {
				return errors.Join(err, updateErr)
			}
			return err
		}
		switch result.Outcome {
		case OutcomeApplied:
			if len(result.ResultStableIDs) > 0 {
				mutation.ResultStableIDs = append([]string(nil), result.ResultStableIDs...)
			}
			if mutation.Kind == MutationMove && len(mutation.ResultStableIDs) == 0 {
				mutation.State, mutation.LastCode = MutationReconciling, "move_identity_missing"
				if err := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); err != nil {
					return err
				}
				return ErrAmbiguousOutcome
			}
			if err := ValidateMutationResults(mutation); err != nil {
				mutation.State, mutation.LastCode = MutationReconciling, "move_identity_invalid"
				if updateErr := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); updateErr != nil {
					return errors.Join(err, updateErr)
				}
				return errors.Join(ErrAmbiguousOutcome, err)
			}
			// Persist provider-assigned identities before touching the local
			// projection. A crash can then replay projection without guessing
			// which destination UID the server assigned.
			mutation.State, mutation.LastCode, mutation.NotBefore = MutationReconciling, "projection_pending", nil
			mutation.UpdatedAt = worker.clock().UTC()
			if err := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); err != nil {
				return err
			}
			if projectionErr := worker.store.ApplyMutationProjection(ctx, lease, mutation); projectionErr != nil {
				mutation.State, mutation.LastCode = MutationReconciling, "projection_failed"
				mutation.UpdatedAt = worker.clock().UTC()
				if updateErr := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); updateErr != nil {
					return errors.Join(projectionErr, updateErr)
				}
				return projectionErr
			}
			mutation.State, mutation.LastCode, mutation.NotBefore = MutationApplied, "", nil
		case OutcomeNotApplied:
			mutation.State, mutation.LastCode = MutationFailed, safeCode(result.Code, "remote_rejected")
		case OutcomeUnknown:
			mutation.State, mutation.LastCode = MutationReconciling, "ambiguous_outcome"
			if err := worker.store.UpdateMutation(contextWithoutCancel(ctx), lease, mutation); err != nil {
				return err
			}
			return ErrAmbiguousOutcome
		default:
			return fmt.Errorf("invalid remote mutation outcome %q", result.Outcome)
		}
		mutation.UpdatedAt = worker.clock().UTC()
		if result.RetryAfter > 0 {
			notBefore := mutation.UpdatedAt.Add(result.RetryAfter)
			mutation.NotBefore = &notBefore
		}
		if err := worker.store.UpdateMutation(ctx, lease, mutation); err != nil {
			return err
		}
	}
	return nil
}

func (worker *Worker) CleanupAccount(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount) error {
	if worker == nil || worker.store == nil {
		return errors.New("external account sync worker is not configured")
	}
	scope := Scope{Owner: owner, AccountID: account.ID}
	lease, err := worker.store.AcquireLease(ctx, scope, worker.holder, worker.clock().UTC(), worker.leaseTTL)
	if err != nil {
		return err
	}
	defer worker.store.ReleaseLease(context.WithoutCancel(ctx), lease)
	return worker.store.DeleteAccountData(ctx, lease)
}

func clearCredentials(credentials *accounts.Credentials) {
	credentials.Username, credentials.Password, credentials.AccessToken, credentials.RefreshToken = "", "", "", ""
	credentials.ExpiresAt = time.Time{}
}

func safeCode(value, fallback string) string {
	if value == "" || len(value) > 63 {
		return fallback
	}
	for _, character := range value {
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return fallback
		}
	}
	return value
}

func contextWithoutCancel(ctx context.Context) context.Context { return context.WithoutCancel(ctx) }
