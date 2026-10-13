package contactsync

import (
	"context"
	"errors"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

type Job struct {
	store     Store
	connector Connector
}

func NewJob(store Store, connector Connector) *Job {
	return &Job{store: store, connector: connector}
}

// RunWithLease uses the account worker's lease and decrypted credential
// lifetime. An account without a contact transport is a deliberate no-op.
func (job *Job) RunWithLease(ctx context.Context, lease accountsync.Lease, account accounts.ExternalAccount, credentials accounts.Credentials) error {
	if !Enabled(account) {
		return nil
	}
	if job == nil || job.store == nil || job.connector == nil {
		return errors.New("provider contact sync is not configured")
	}
	state, err := job.store.ContactState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	mutations, err := job.store.PendingContactMutations(ctx, lease, 50)
	if err != nil {
		return err
	}
	for _, mutation := range mutations {
		mutation.State, mutation.Attempts, mutation.UpdatedAt = MutationApplying, mutation.Attempts+1, time.Now().UTC()
		if err := job.store.UpdateContactMutation(ctx, lease, mutation); err != nil {
			return err
		}
		current, loadErr := job.currentContact(ctx, lease.Scope, state, mutation)
		if loadErr != nil {
			mutation.State, mutation.LastCode, mutation.UpdatedAt = MutationFailed, mutationFailureCode(mutation.Kind), time.Now().UTC()
			if updateErr := job.store.UpdateContactMutation(ctx, lease, mutation); updateErr != nil {
				return updateErr
			}
			continue
		}
		outcome, applyErr := job.connector.ApplyMutation(ctx, account, credentials, mutation, current)
		if applyErr != nil {
			return applyErr
		}
		switch outcome {
		case OutcomeApplied:
			mutation.State, mutation.LastCode = MutationApplied, ""
		case OutcomeConflict:
			mutation.State, mutation.LastCode = MutationConflict, "remote_changed"
		default:
			return errors.New("invalid provider contact mutation outcome")
		}
		mutation.UpdatedAt = time.Now().UTC()
		if err := job.store.UpdateContactMutation(ctx, lease, mutation); err != nil {
			return err
		}
	}
	snapshot, err := job.connector.Sync(ctx, account, credentials, state)
	if err != nil {
		return err
	}
	if err := ValidateSnapshot(lease.Scope, snapshot); err != nil {
		return err
	}
	return job.store.CommitContactSnapshot(ctx, lease, snapshot)
}

func (job *Job) currentContact(ctx context.Context, scope accountsync.Scope, state State, mutation Mutation) (Contact, error) {
	if mutation.Kind != MutationCreate {
		return job.store.ProviderContact(ctx, scope, mutation.ContactID)
	}
	for _, book := range state.AddressBooks {
		if book.StableID == mutation.Desired.AddressBookID && mutation.Desired.AccountID == scope.AccountID {
			return mutation.Desired, nil
		}
	}
	return Contact{}, ErrContactNotFound
}

func mutationFailureCode(kind MutationKind) string {
	if kind == MutationCreate {
		return "address_book_missing"
	}
	return "contact_missing"
}
