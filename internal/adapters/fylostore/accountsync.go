package fylostore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

const rawMessageMaxBytes = accountsync.MaxRawBatchBytes

// AccountSync persists sync state, provider indexes, mutation journals, and
// raw RFC 822 bucket objects through the one Go-owned FYLO process.
type AccountSync struct {
	documents   *Accounts
	stagingRoot string
	mutation    sync.Mutex
}

func NewAccountSync(caller Caller, posixAccess bool, stagingRoot string) (*AccountSync, error) {
	root := filepath.Clean(stagingRoot)
	if root == "." || root == string(filepath.Separator) {
		return nil, errors.New("provider sync staging root must be a dedicated directory")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve provider sync staging root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create provider sync staging root: %w", err)
	}
	return &AccountSync{documents: NewAccounts(caller, posixAccess), stagingRoot: absolute}, nil
}

func (store *AccountSync) AcquireLease(ctx context.Context, scope accountsync.Scope, holder string, now time.Time, ttl time.Duration) (accountsync.Lease, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	if holder == "" || ttl <= 0 {
		return accountsync.Lease{}, accountsync.ErrInvalidBatch
	}
	documentID, state, err := store.findState(ctx, scope)
	if err != nil && !errors.Is(err, accounts.ErrNotFound) {
		return accountsync.Lease{}, err
	}
	if err == nil && state.LeaseToken != "" && state.LeaseExpiresAt != nil && state.LeaseExpiresAt.After(now) {
		return accountsync.Lease{}, accountsync.ErrLeaseHeld
	}
	token, err := opaqueToken()
	if err != nil {
		return accountsync.Lease{}, err
	}
	old := state
	expiresAt := now.Add(ttl).UTC()
	state.AccountID, state.TenantID, state.PrincipalID, state.OwnerAddress = scope.AccountID, scope.Owner.TenantID, scope.Owner.PrincipalID, scope.Owner.Address
	state.LeaseToken, state.LeaseHolder, state.LeaseExpiresAt = token, holder, &expiresAt
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	if documentID == "" {
		if _, err := store.documents.put(ctx, ProviderSyncState, state, access); err != nil {
			return accountsync.Lease{}, err
		}
	} else if err := store.documents.patch(ctx, ProviderSyncState, documentID, state, old, access); err != nil {
		return accountsync.Lease{}, err
	}
	return accountsync.Lease{Scope: scope, Token: token, Holder: holder, ExpiresAt: expiresAt}, nil
}

func (store *AccountSync) ReleaseLease(ctx context.Context, lease accountsync.Lease) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if state.LeaseToken != lease.Token || state.LeaseHolder != lease.Holder {
		return accountsync.ErrLeaseHeld
	}
	old := state
	// FYLO patchDoc is a shallow merge, so omitting optional fields would leave
	// the live lease in place. Expire it explicitly; the next acquire replaces
	// the token and holder under the same serialized store mutation.
	releasedAt := time.Unix(0, 0).UTC()
	state.LeaseExpiresAt = &releasedAt
	return store.documents.patch(ctx, ProviderSyncState, documentID, state, old, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID))
}

func (store *AccountSync) Checkpoint(ctx context.Context, scope accountsync.Scope) (accountsync.Checkpoint, error) {
	_, state, err := store.findState(ctx, scope)
	if errors.Is(err, accounts.ErrNotFound) {
		return accountsync.Checkpoint{}, nil
	}
	if err != nil {
		return accountsync.Checkpoint{}, err
	}
	updatedAt := time.Time{}
	if state.CursorUpdatedAt != nil {
		updatedAt = *state.CursorUpdatedAt
	}
	return accountsync.Checkpoint{Cursor: state.Cursor, Version: state.CursorVersion, UpdatedAt: updatedAt}, nil
}

func (store *AccountSync) CommitBatch(ctx context.Context, lease accountsync.Lease, expected uint64, batch accountsync.Batch) (accountsync.Checkpoint, error) {
	if err := accountsync.ValidateBatch(batch); err != nil {
		return accountsync.Checkpoint{}, err
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return accountsync.Checkpoint{}, err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return accountsync.Checkpoint{}, accountsync.ErrLeaseHeld
	}
	if state.CursorVersion != expected || state.Cursor != batch.CursorBefore {
		return accountsync.Checkpoint{}, accountsync.ErrCheckpointConflict
	}
	for _, folder := range batch.Folders {
		if err := store.upsertFolder(ctx, lease.Scope, folder); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	if batch.FolderSnapshot {
		if err := store.reconcileFolders(ctx, lease.Scope, batch.Folders); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	inboxFolders, err := store.inboxFolderIDs(ctx, lease.Scope)
	if err != nil {
		return accountsync.Checkpoint{}, err
	}
	resetFolders := make(map[string]bool, len(batch.Folders))
	for _, folder := range batch.Folders {
		if folder.ResetMessages {
			resetFolders[folder.StableID] = true
		}
	}
	for _, message := range batch.Messages {
		notify := expected > 0 && messageInFolders(message.FolderStableID, message.FolderStableIDs, inboxFolders) && !resetFolders[message.FolderStableID] && !hasProviderFlag(message.Flags, "\\Seen")
		if err := store.upsertMessage(ctx, lease.Scope, message, notify); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	for _, update := range batch.Updates {
		if err := store.updateMessage(ctx, lease.Scope, update); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	for _, deletion := range batch.Deletions {
		if err := store.deleteMessage(ctx, lease.Scope, deletion); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	for _, window := range batch.Windows {
		if err := store.reconcileWindow(ctx, lease.Scope, window); err != nil {
			return accountsync.Checkpoint{}, err
		}
	}
	old := state
	updatedAt := time.Now().UTC()
	state.Cursor, state.CursorVersion, state.CursorUpdatedAt = batch.CursorAfter, expected+1, &updatedAt
	if err := store.documents.patch(ctx, ProviderSyncState, documentID, state, old, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID)); err != nil {
		return accountsync.Checkpoint{}, err
	}
	return accountsync.Checkpoint{Cursor: state.Cursor, Version: state.CursorVersion, UpdatedAt: updatedAt}, nil
}

func (store *AccountSync) EnqueueMutation(ctx context.Context, scope accountsync.Scope, mutation accountsync.Mutation) (accountsync.Mutation, error) {
	if err := accountsync.ValidateMutation(mutation.MutationRequest); err != nil {
		return accountsync.Mutation{}, err
	}
	if err := accountsync.ValidateMutationResults(mutation); err != nil {
		return accountsync.Mutation{}, err
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	records, err := store.mutationRecords(ctx, scope)
	if err != nil {
		return accountsync.Mutation{}, err
	}
	for _, record := range records {
		if record.ID == mutation.ID || record.IdempotencyKey == mutation.IdempotencyKey {
			if record.Kind != mutation.Kind || record.TargetFolderID != mutation.TargetFolderID || record.FolderStableID != mutation.FolderStableID ||
				record.ParentStableID != mutation.ParentStableID || record.FolderName != mutation.FolderName || record.FolderPath != mutation.FolderPath ||
				record.NewFolderPath != mutation.NewFolderPath || record.Value != mutation.Value || !sameStrings(record.MessageStableIDs, mutation.MessageStableIDs) {
				return accountsync.Mutation{}, accounts.ErrConflict
			}
			return record.Mutation, nil
		}
	}
	_, err = store.documents.put(ctx, ProviderMutations, newMutationRecord(mutation), store.documents.writeAccess(scope.Owner, scope.AccountID))
	return mutation, err
}

func (store *AccountSync) MutationByIdempotencyKey(ctx context.Context, scope accountsync.Scope, key string) (accountsync.Mutation, bool, error) {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	records, err := store.mutationRecords(ctx, scope)
	if err != nil {
		return accountsync.Mutation{}, false, err
	}
	for _, record := range records {
		if record.IdempotencyKey == key {
			return record.Mutation, true, nil
		}
	}
	return accountsync.Mutation{}, false, nil
}

func (store *AccountSync) Mutation(ctx context.Context, scope accountsync.Scope, id string) (accountsync.Mutation, error) {
	if id == "" || len(id) > 96 || !strings.HasPrefix(id, "mut_") || strings.ContainsAny(id, "\x00\r\n") {
		return accountsync.Mutation{}, accounts.ErrInvalid
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	records, err := store.mutationRecords(ctx, scope)
	if err != nil {
		return accountsync.Mutation{}, err
	}
	for _, record := range records {
		if record.ID == id {
			return record.Mutation, nil
		}
	}
	return accountsync.Mutation{}, accounts.ErrNotFound
}

func sameStrings(left, right []string) bool {
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

func (store *AccountSync) PendingMutations(ctx context.Context, lease accountsync.Lease, limit int) ([]accountsync.Mutation, error) {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return nil, err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return nil, accountsync.ErrLeaseHeld
	}
	if limit < 1 {
		return nil, nil
	}
	if limit > 100 {
		limit = 100
	}
	records, err := store.mutationRecords(ctx, lease.Scope)
	if err != nil {
		return nil, err
	}
	result := make([]accountsync.Mutation, 0, limit)
	for _, record := range records {
		if record.State == accountsync.MutationQueued || record.State == accountsync.MutationApplying || record.State == accountsync.MutationReconciling {
			result = append(result, record.Mutation)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].CreatedAt.Before(result[right].CreatedAt) })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (store *AccountSync) UpdateMutation(ctx context.Context, lease accountsync.Lease, replacement accountsync.Mutation) error {
	if err := accountsync.ValidateMutationResults(replacement); err != nil {
		return err
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return accountsync.ErrLeaseHeld
	}
	documents, err := store.documents.find(ctx, ProviderMutations, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current mutationRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider mutation: %w", err)
		}
		if current.ID != replacement.ID || !recordOwner(current.TenantID, current.PrincipalID, lease.Scope) {
			continue
		}
		current.Mutation.Scope = accountsync.Scope{Owner: accounts.Owner{TenantID: current.TenantID, PrincipalID: current.PrincipalID, Address: current.OwnerAddress}, AccountID: current.AccountID}
		if current.AccountID != lease.Scope.AccountID || replacement.Scope != current.Scope || !mutationTransition(current.State, replacement.State) {
			return accounts.ErrInvalid
		}
		return store.documents.patch(ctx, ProviderMutations, id, newMutationRecord(replacement), current, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID))
	}
	return accounts.ErrNotFound
}

func (store *AccountSync) ApplyMutationProjection(ctx context.Context, lease accountsync.Lease, mutation accountsync.Mutation) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) || mutation.Scope != lease.Scope {
		return accountsync.ErrLeaseHeld
	}
	// The authoritative provider folder catalog is replaced by the sync that
	// follows every mutation run. Projecting a guessed local folder tree here
	// would create a second source of truth during ambiguous IMAP outcomes.
	if accountsync.IsFolderMutation(mutation.Kind) {
		return nil
	}
	if err := accountsync.ValidateMutationResults(mutation); err != nil {
		return err
	}
	replacements, err := mutationReplacements(mutation)
	if err != nil {
		return err
	}
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return err
	}
	return store.applyProjectionDocuments(ctx, lease.Scope, mutation, replacements, documents)
}

func (store *AccountSync) applyProjectionDocuments(ctx context.Context, scope accountsync.Scope, mutation accountsync.Mutation, replacements map[string]string, documents map[string]json.RawMessage) error {
	targets := make(map[string]bool, len(mutation.MessageStableIDs))
	for _, stableID := range mutation.MessageStableIDs {
		targets[stableID] = true
	}
	for id, raw := range documents {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if !targets[current.StableID] || !ownedMessageRecord(current, scope) {
			continue
		}
		old := messageRecord{}
		if err := json.Unmarshal(raw, &old); err != nil {
			return err
		}
		deleted, err := store.projectMutationRecord(ctx, scope, id, mutation, replacements, &current)
		if err != nil {
			return err
		}
		if deleted {
			continue
		}
		current.Revision = "mutation:" + mutation.ID
		if err := store.documents.patch(ctx, ProviderMessages, id, current, old, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
			return err
		}
	}
	return nil
}

func mutationReplacements(mutation accountsync.Mutation) (map[string]string, error) {
	replacements := make(map[string]string, len(mutation.MessageStableIDs))
	if mutation.Kind != accountsync.MutationMove {
		return replacements, nil
	}
	if len(mutation.ResultStableIDs) != len(mutation.MessageStableIDs) {
		return nil, accounts.ErrInvalid
	}
	for index, stableID := range mutation.MessageStableIDs {
		replacements[stableID] = mutation.ResultStableIDs[index]
	}
	return replacements, nil
}

func (store *AccountSync) projectMutationRecord(ctx context.Context, scope accountsync.Scope, documentID string, mutation accountsync.Mutation, replacements map[string]string, current *messageRecord) (bool, error) {
	switch mutation.Kind {
	case accountsync.MutationSetRead:
		current.Flags = projectFlag(current.Flags, "\\Seen", mutation.Value)
	case accountsync.MutationSetStarred:
		current.Flags = projectFlag(current.Flags, "\\Flagged", mutation.Value)
	case accountsync.MutationMove:
		return false, store.projectMove(ctx, scope, mutation.TargetFolderID, replacements[current.StableID], current)
	case accountsync.MutationDelete:
		return true, store.deleteMessageRecord(ctx, scope, documentID, *current)
	default:
		return false, accounts.ErrInvalid
	}
	return false, nil
}

func (store *AccountSync) projectMove(ctx context.Context, scope accountsync.Scope, targetID, replacement string, current *messageRecord) error {
	if replacement == current.StableID && strings.HasPrefix(current.StableID, "gmail:") {
		return store.projectGmailMove(ctx, scope, current, targetID)
	}
	folder, _, _, ok := providerMessageIdentity(replacement)
	if !ok || folder != targetID {
		return accounts.ErrInvalid
	}
	current.StableID, current.FolderStableID = replacement, folder
	return nil
}

type gmailFolderCatalog struct {
	target accountsync.Folder
	found  bool
	roles  map[string]string
	allID  string
}

func (store *AccountSync) projectGmailMove(ctx context.Context, scope accountsync.Scope, message *messageRecord, targetID string) error {
	folders, err := store.ListFolders(ctx, scope)
	if err != nil {
		return err
	}
	catalog := newGmailFolderCatalog(folders, targetID)
	if !catalog.found {
		return accounts.ErrInvalid
	}
	memberships := map[string]bool{}
	for _, folder := range recordFolderMemberships(*message) {
		memberships[folder] = true
	}
	removedRoles := gmailProjectionRemovedRoles(catalog.target.Role)
	for folderID := range memberships {
		if removedRoles[catalog.roles[folderID]] {
			delete(memberships, folderID)
		}
	}
	projectAllMailMembership(memberships, catalog)
	memberships[targetID] = true
	message.FolderStableIDs = message.FolderStableIDs[:0]
	for folder := range memberships {
		message.FolderStableIDs = append(message.FolderStableIDs, folder)
	}
	sort.Strings(message.FolderStableIDs)
	message.FolderStableID = targetID
	message.RemoteStableID = ""
	return nil
}

func newGmailFolderCatalog(folders []accountsync.Folder, targetID string) gmailFolderCatalog {
	catalog := gmailFolderCatalog{roles: map[string]string{}}
	for _, folder := range folders {
		role := folder.Role
		if role == "" && strings.EqualFold(folder.Name, "INBOX") {
			role = "inbox"
		}
		catalog.roles[folder.StableID] = role
		if folder.StableID == targetID {
			catalog.target, catalog.found = folder, true
		}
		if role == "all" || role == "archive" {
			catalog.allID = folder.StableID
		}
	}
	return catalog
}

func projectAllMailMembership(memberships map[string]bool, catalog gmailFolderCatalog) {
	if catalog.target.Role == "trash" || catalog.target.Role == "spam" {
		delete(memberships, catalog.allID)
	} else if catalog.allID != "" {
		memberships[catalog.allID] = true
	}
}

func gmailProjectionRemovedRoles(targetRole string) map[string]bool {
	switch targetRole {
	case "inbox":
		return map[string]bool{"trash": true, "spam": true}
	case "trash":
		return map[string]bool{"inbox": true, "spam": true}
	case "spam":
		return map[string]bool{"inbox": true, "trash": true}
	default:
		return map[string]bool{"inbox": true, "trash": true, "spam": true}
	}
}

func (store *AccountSync) DeleteAccountData(ctx context.Context, lease accountsync.Lease) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	stateID, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return accountsync.ErrLeaseHeld
	}
	messageDocuments, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range messageDocuments {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if current.AccountID == lease.Scope.AccountID && recordOwner(current.TenantID, current.PrincipalID, lease.Scope) {
			if err := store.deleteMessageRecord(ctx, lease.Scope, id, current); err != nil {
				return err
			}
		}
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderFolders, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record folderRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderMutations, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record mutationRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteNotificationRecords(ctx, lease.Scope); err != nil {
		return err
	}
	if err := store.deleteSubmissionRecords(ctx, lease.Scope); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderContactBooks, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record contactBookRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderContacts, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record providerContactRecord
		err := json.Unmarshal(raw, &record)
		return record.Contact.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderContactState, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record contactStateRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderContactMutations, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record contactMutationRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderCalendars, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record calendarRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderCalendarResources, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record calendarResourceRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderCalendarState, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record calendarStateRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	if err := store.deleteOwnedDocuments(ctx, ProviderCalendarMutations, lease.Scope, func(raw json.RawMessage) (accounts.AccountID, string, string, error) {
		var record calendarMutationRecord
		err := json.Unmarshal(raw, &record)
		return record.AccountID, record.TenantID, record.PrincipalID, err
	}); err != nil {
		return err
	}
	return store.documents.delete(ctx, ProviderSyncState, stateID, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID))
}

func (store *AccountSync) deleteNotificationRecords(ctx context.Context, scope accountsync.Scope) error {
	documents, err := store.documents.find(ctx, ProviderNotifications, store.documents.schedulerReadAccess())
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var record providerNotificationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID {
			if err := store.documents.delete(ctx, ProviderNotifications, id, store.documents.schedulerWriteAccess()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *AccountSync) deleteOwnedDocuments(ctx context.Context, collection string, scope accountsync.Scope, identify func(json.RawMessage) (accounts.AccountID, string, string, error)) error {
	documents, err := store.documents.find(ctx, collection, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		accountID, tenantID, principalID, err := identify(raw)
		if err != nil {
			return err
		}
		if accountID == scope.AccountID && recordOwner(tenantID, principalID, scope) {
			if err := store.documents.delete(ctx, collection, id, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *AccountSync) DueAccounts(ctx context.Context, now time.Time, limit int) ([]accountsync.ScheduledAccount, error) {
	if limit < 1 {
		return nil, nil
	}
	if limit > 100 {
		limit = 100
	}
	documents, err := store.documents.find(ctx, ProviderSchedule, store.documents.schedulerReadAccess())
	if err != nil {
		return nil, err
	}
	result := make([]accountsync.ScheduledAccount, 0, limit)
	for _, raw := range documents {
		var record scheduleRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider schedule: %w", err)
		}
		if record.NextRunAt.After(now) {
			continue
		}
		result = append(result, scheduledAccount(record))
	}
	sort.Slice(result, func(left, right int) bool { return result[left].NextRunAt.Before(result[right].NextRunAt) })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (store *AccountSync) WatchAccounts(ctx context.Context, limit int) ([]accountsync.ScheduledAccount, error) {
	if limit < 1 {
		return nil, nil
	}
	if limit > 1000 {
		limit = 1000
	}
	documents, err := store.documents.find(ctx, ProviderSchedule, store.documents.schedulerReadAccess())
	if err != nil {
		return nil, err
	}
	result := make([]accountsync.ScheduledAccount, 0, min(limit, len(documents)))
	for _, raw := range documents {
		var record scheduleRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider watch schedule: %w", err)
		}
		result = append(result, scheduledAccount(record))
	}
	sort.Slice(result, func(left, right int) bool {
		return watchScheduleKey(result[left]) < watchScheduleKey(result[right])
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func scheduledAccount(record scheduleRecord) accountsync.ScheduledAccount {
	return accountsync.ScheduledAccount{Scope: accountsync.Scope{Owner: accounts.Owner{TenantID: record.TenantID, PrincipalID: record.PrincipalID, Address: record.OwnerAddress}, AccountID: record.AccountID},
		Provider: record.Provider, NextRunAt: record.NextRunAt, UpdatedAt: record.UpdatedAt, Attempts: record.Attempts, LastCode: record.LastCode}
}

func watchScheduleKey(value accountsync.ScheduledAccount) string {
	return value.Scope.Owner.TenantID + "\x00" + value.Scope.Owner.PrincipalID + "\x00" + string(value.Scope.AccountID)
}

func (store *AccountSync) Reschedule(ctx context.Context, value accountsync.ScheduledAccount) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.documents.findSchedule(ctx, value.Scope.Owner, value.Scope.AccountID)
	if err != nil {
		return err
	}
	// A request may enqueue work while this scheduler pass is running. Retry
	// advances UpdatedAt and moves NextRunAt to now; never overwrite that wake
	// with the stale pass's ordinary success poll.
	if current.UpdatedAt.After(value.UpdatedAt) {
		return nil
	}
	replacement := current
	replacement.NextRunAt, replacement.Attempts, replacement.LastCode, replacement.UpdatedAt = value.NextRunAt.UTC(), value.Attempts, value.LastCode, time.Now().UTC()
	return store.documents.patch(ctx, ProviderSchedule, documentID, replacement, current, store.documents.schedulerWriteAccess())
}

func (store *AccountSync) Retry(ctx context.Context, scope accountsync.Scope) error {
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documentID, current, err := store.documents.findSchedule(ctx, scope.Owner, scope.AccountID)
	if err != nil {
		return err
	}
	replacement := current
	replacement.NextRunAt, replacement.Attempts, replacement.LastCode, replacement.UpdatedAt = time.Now().UTC(), 0, "", time.Now().UTC()
	return store.documents.patch(ctx, ProviderSchedule, documentID, replacement, current, store.documents.schedulerWriteAccess())
}

func (store *AccountSync) Diagnostics(ctx context.Context, scope accountsync.Scope) (accountsync.Diagnostics, error) {
	_, schedule, err := store.documents.findSchedule(ctx, scope.Owner, scope.AccountID)
	if err != nil {
		return accountsync.Diagnostics{}, err
	}
	checkpoint, err := store.Checkpoint(ctx, scope)
	if err != nil {
		return accountsync.Diagnostics{}, err
	}
	var cursorUpdatedAt *time.Time
	if !checkpoint.UpdatedAt.IsZero() {
		value := checkpoint.UpdatedAt
		cursorUpdatedAt = &value
	}
	return accountsync.Diagnostics{AccountID: scope.AccountID, CursorVersion: checkpoint.Version, CursorUpdatedAt: cursorUpdatedAt,
		NextRunAt: schedule.NextRunAt, Attempts: schedule.Attempts, LastCode: schedule.LastCode, ScheduleUpdatedAt: schedule.UpdatedAt}, nil
}

func (store *AccountSync) ListFolders(ctx context.Context, scope accountsync.Scope) ([]accountsync.Folder, error) {
	documents, err := store.documents.find(ctx, ProviderFolders, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []accountsync.Folder{}
	for _, raw := range documents {
		var record folderRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider folder: %w", err)
		}
		if record.AccountID == scope.AccountID && recordOwner(record.TenantID, record.PrincipalID, scope) {
			result = append(result, record.Folder)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Role != result[right].Role {
			return result[left].Role < result[right].Role
		}
		return result[left].Name < result[right].Name
	})
	return result, nil
}

func (store *AccountSync) ListMessages(ctx context.Context, scope accountsync.Scope, folder string, limit int) ([]accountsync.Message, error) {
	limit = boundedMessageLimit(limit)
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []accountsync.Message{}
	for _, raw := range documents {
		var record messageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if !ownedMessageRecord(record, scope) || folder != "" && !recordInFolder(record, folder) {
			continue
		}
		result = append(result, record.message())
	}
	sort.Slice(result, func(left, right int) bool {
		if !result[left].ReceivedAt.Equal(result[right].ReceivedAt) {
			return result[left].ReceivedAt.After(result[right].ReceivedAt)
		}
		return result[left].StableID < result[right].StableID
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func boundedMessageLimit(limit int) int {
	if limit < 1 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func ownedMessageRecord(record messageRecord, scope accountsync.Scope) bool {
	return record.AccountID == scope.AccountID && recordOwner(record.TenantID, record.PrincipalID, scope)
}

type providerFolderKey struct {
	accountID accounts.AccountID
	stableID  string
}

func (store *AccountSync) ListUnifiedMessages(ctx context.Context, owner accounts.Owner, filter accountsync.UnifiedMessageFilter) ([]accountsync.UnifiedMessage, error) {
	filter, valid := normalizeUnifiedFilter(filter)
	if !valid {
		return nil, accounts.ErrInvalid
	}
	ownedAccounts, err := store.ownedProviderAccounts(ctx, owner, filter.AccountID)
	if err != nil {
		return nil, err
	}
	folders, err := store.providerFolders(ctx, owner, ownedAccounts)
	if err != nil {
		return nil, err
	}
	result, err := store.unifiedProviderMessages(ctx, owner, filter, ownedAccounts, folders)
	if err != nil {
		return nil, err
	}
	sortUnifiedMessages(result, filter.OldestFirst)
	return pageUnifiedMessages(result, filter.Offset, filter.Limit), nil
}

func normalizeUnifiedFilter(filter accountsync.UnifiedMessageFilter) (accountsync.UnifiedMessageFilter, bool) {
	if filter.Limit < 1 {
		filter.Limit = 100
	}
	valid := filter.Limit <= 1000 && filter.Offset >= 0 && filter.Offset <= 100_000 && len(filter.Query) <= 2000 && len(filter.FolderRole) <= 60 &&
		len(filter.FolderStableID) <= 512 && !strings.ContainsAny(filter.FolderStableID, "\x00\r\n")
	return filter, valid
}

func (store *AccountSync) ownedProviderAccounts(ctx context.Context, owner accounts.Owner, accountID accounts.AccountID) (map[accounts.AccountID]accounts.ExternalAccount, error) {
	accountDocuments, err := store.documents.find(ctx, ProviderAccounts, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	ownedAccounts := map[accounts.AccountID]accounts.ExternalAccount{}
	for _, raw := range accountDocuments {
		record, err := decodeAccountRecord(raw)
		if err != nil {
			return nil, err
		}
		if record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID && record.State != accounts.StateDeleting &&
			(accountID == "" || record.ID == accountID) {
			ownedAccounts[record.ID] = record.ExternalAccount
		}
	}
	if accountID != "" && ownedAccounts[accountID].ID == "" {
		return nil, accounts.ErrNotFound
	}
	return ownedAccounts, nil
}

func (store *AccountSync) providerFolders(ctx context.Context, owner accounts.Owner, ownedAccounts map[accounts.AccountID]accounts.ExternalAccount) (map[providerFolderKey]accountsync.Folder, error) {
	folderDocuments, err := store.documents.find(ctx, ProviderFolders, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	folders := map[providerFolderKey]accountsync.Folder{}
	for _, raw := range folderDocuments {
		var record folderRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider folder: %w", err)
		}
		if ownedAccounts[record.AccountID].ID != "" && record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID {
			folders[providerFolderKey{record.AccountID, record.StableID}] = record.Folder
		}
	}
	return folders, nil
}

func (store *AccountSync) unifiedProviderMessages(ctx context.Context, owner accounts.Owner, filter accountsync.UnifiedMessageFilter, ownedAccounts map[accounts.AccountID]accounts.ExternalAccount, folders map[providerFolderKey]accountsync.Folder) ([]accountsync.UnifiedMessage, error) {
	messageDocuments, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	role := strings.ToLower(strings.TrimSpace(filter.FolderRole))
	result := []accountsync.UnifiedMessage{}
	for _, raw := range messageDocuments {
		var record messageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider message: %w", err)
		}
		message, include := unifiedProviderMessage(record, owner, filter, query, role, ownedAccounts, folders)
		if include {
			result = append(result, message)
		}
	}
	return result, nil
}

func unifiedProviderMessage(record messageRecord, owner accounts.Owner, filter accountsync.UnifiedMessageFilter, query, role string, accountsByID map[accounts.AccountID]accounts.ExternalAccount, folders map[providerFolderKey]accountsync.Folder) (accountsync.UnifiedMessage, bool) {
	account := accountsByID[record.AccountID]
	if account.ID == "" || record.TenantID != owner.TenantID || record.PrincipalID != owner.PrincipalID {
		return accountsync.UnifiedMessage{}, false
	}
	message := record.message()
	if !unifiedMessageMatches(record, message, filter, query, role, folders) {
		return accountsync.UnifiedMessage{}, false
	}
	folder := folders[providerFolderKey{record.AccountID, presentationFolder(record, filter.FolderStableID, role, folders)}]
	folderRole := strings.ToLower(folder.Role)
	if folderRole == "" {
		folderRole = strings.ToLower(folder.Name)
	}
	return accountsync.UnifiedMessage{Message: message, AccountID: account.ID, MailboxAddress: account.MailboxAddress,
		AccountLabel: account.Label, AccountColor: account.Color, FolderName: folder.Name, FolderRole: folderRole, Labels: providerLabelNames(record, folders)}, true
}

func unifiedMessageMatches(record messageRecord, message accountsync.Message, filter accountsync.UnifiedMessageFilter, query, role string, folders map[providerFolderKey]accountsync.Folder) bool {
	read, starred := hasProviderFlag(message.Flags, "\\Seen"), hasProviderFlag(message.Flags, "\\Flagged")
	return !(filter.FolderStableID != "" && !recordInFolder(record, filter.FolderStableID) ||
		role != "" && !recordInRole(record, record.AccountID, role, folders) ||
		filter.Read != nil && read != *filter.Read || filter.Starred != nil && starred != *filter.Starred ||
		filter.ReceivedAtFrom != nil && message.ReceivedAt.Before(*filter.ReceivedAtFrom) || query != "" && !messageContains(message, query))
}

func sortUnifiedMessages(result []accountsync.UnifiedMessage, oldestFirst bool) {
	sort.Slice(result, func(left, right int) bool {
		if !result[left].ReceivedAt.Equal(result[right].ReceivedAt) {
			if oldestFirst {
				return result[left].ReceivedAt.Before(result[right].ReceivedAt)
			}
			return result[left].ReceivedAt.After(result[right].ReceivedAt)
		}
		if result[left].AccountID != result[right].AccountID {
			return result[left].AccountID < result[right].AccountID
		}
		return result[left].StableID < result[right].StableID
	})
}

func pageUnifiedMessages(result []accountsync.UnifiedMessage, offset, limit int) []accountsync.UnifiedMessage {
	if offset >= len(result) {
		return []accountsync.UnifiedMessage{}
	}
	result = result[offset:]
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (store *AccountSync) GetMessageSource(ctx context.Context, scope accountsync.Scope, stableID string) (accountsync.Message, []byte, error) {
	message, rawBucketID, err := store.getMessageRecord(ctx, scope, stableID)
	if err != nil {
		return accountsync.Message{}, nil, err
	}
	payload, err := store.getRaw(ctx, scope, rawBucketID)
	return message, payload, err
}

func recordInFolder(record messageRecord, folder string) bool {
	if len(record.FolderStableIDs) == 0 {
		return record.FolderStableID == folder
	}
	return slices.Contains(record.FolderStableIDs, folder)
}

func recordInRole(record messageRecord, accountID accounts.AccountID, role string, folders map[providerFolderKey]accountsync.Folder) bool {
	for _, folderID := range recordFolderMemberships(record) {
		folder := folders[providerFolderKey{accountID, folderID}]
		candidate := strings.ToLower(strings.TrimSpace(folder.Role))
		if candidate == "" {
			candidate = strings.ToLower(strings.TrimSpace(folder.Name))
		}
		if candidate == role {
			return true
		}
	}
	return false
}

func presentationFolder(record messageRecord, requested, role string, folders map[providerFolderKey]accountsync.Folder) string {
	if requested != "" && recordInFolder(record, requested) {
		return requested
	}
	if role != "" {
		for _, folderID := range recordFolderMemberships(record) {
			folder := folders[providerFolderKey{record.AccountID, folderID}]
			if strings.EqualFold(folder.Role, role) || folder.Role == "" && strings.EqualFold(folder.Name, role) {
				return folderID
			}
		}
	}
	return record.FolderStableID
}

func providerLabelNames(record messageRecord, folders map[providerFolderKey]accountsync.Folder) []string {
	result := []string{}
	for _, folderID := range recordFolderMemberships(record) {
		folder := folders[providerFolderKey{record.AccountID, folderID}]
		if folder.Role == "" && folder.Name != "" {
			result = append(result, folder.Name)
		}
	}
	sort.Strings(result)
	return result
}

func recordFolderMemberships(record messageRecord) []string {
	if len(record.FolderStableIDs) > 0 {
		return record.FolderStableIDs
	}
	return []string{record.FolderStableID}
}

func messageInFolders(primary string, memberships []string, folders map[string]bool) bool {
	if folders[primary] {
		return true
	}
	for _, folder := range memberships {
		if folders[folder] {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func (store *AccountSync) GetMessage(ctx context.Context, scope accountsync.Scope, stableID string) (accountsync.Message, error) {
	message, _, err := store.getMessageRecord(ctx, scope, stableID)
	return message, err
}

func (store *AccountSync) getMessageRecord(ctx context.Context, scope accountsync.Scope, stableID string) (accountsync.Message, string, error) {
	if stableID == "" || len(stableID) > 512 || strings.ContainsAny(stableID, "\x00\r\n") {
		return accountsync.Message{}, "", accounts.ErrInvalid
	}
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return accountsync.Message{}, "", err
	}
	for _, raw := range documents {
		var record messageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return accountsync.Message{}, "", fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if record.StableID != stableID || record.AccountID != scope.AccountID || !recordOwner(record.TenantID, record.PrincipalID, scope) {
			continue
		}
		return record.message(), record.RawBucketID, nil
	}
	return accountsync.Message{}, "", accounts.ErrNotFound
}

func (store *AccountSync) ensureNewMailNotification(ctx context.Context, scope accountsync.Scope, message accountsync.Message) error {
	id := providerNotificationID(scope, message.StableID)
	documents, err := store.documents.find(ctx, ProviderNotifications, store.documents.schedulerReadAccess())
	if err != nil {
		return err
	}
	for _, raw := range documents {
		var current providerNotificationRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider notification: %w", err)
		}
		if current.ID == id {
			return nil
		}
	}
	now := time.Now().UTC()
	record := providerNotificationRecord{
		ID: id, AccountID: scope.AccountID, TenantID: scope.Owner.TenantID,
		PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address,
		StableID: message.StableID, Sender: message.Sender, Subject: message.Subject,
		ReceivedAt: message.ReceivedAt, State: "queued", NotBefore: now,
		CreatedAt: now, UpdatedAt: now,
	}
	_, err = store.documents.put(ctx, ProviderNotifications, record, store.documents.schedulerWriteAccess())
	return err
}

// ClaimNewMailNotifications leases a bounded cross-owner batch to the signed
// Tachyon push worker. Notification records use a scheduler-only uid/gid and
// never enter the public FYLO bridge collection allowlist.
func (store *AccountSync) ClaimNewMailNotifications(ctx context.Context, holder string, now time.Time, ttl time.Duration, limit int) (accountsync.NewMailNotificationClaim, error) {
	if strings.TrimSpace(holder) == "" || len(holder) > 128 || strings.ContainsAny(holder, "\x00\r\n") || ttl < time.Second || ttl > 10*time.Minute || limit < 1 || limit > 100 {
		return accountsync.NewMailNotificationClaim{}, accounts.ErrInvalid
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderNotifications, store.documents.schedulerReadAccess())
	if err != nil {
		return accountsync.NewMailNotificationClaim{}, err
	}
	type candidate struct {
		documentID string
		record     providerNotificationRecord
	}
	candidates := make([]candidate, 0, len(documents))
	for documentID, raw := range documents {
		var record providerNotificationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return accountsync.NewMailNotificationClaim{}, fmt.Errorf("decode FYLO provider notification: %w", err)
		}
		if record.State == "delivered" && record.CompletedAt != nil && record.CompletedAt.Before(now.Add(-30*24*time.Hour)) {
			if err := store.documents.delete(ctx, ProviderNotifications, documentID, store.documents.schedulerWriteAccess()); err != nil {
				return accountsync.NewMailNotificationClaim{}, err
			}
			continue
		}
		if (record.State == "queued" && !record.NotBefore.After(now)) ||
			(record.State == "claimed" && record.ClaimExpiresAt != nil && !record.ClaimExpiresAt.After(now)) {
			candidates = append(candidates, candidate{documentID: documentID, record: record})
		}
	}
	sort.Slice(candidates, func(left, right int) bool {
		return candidates[left].record.CreatedAt.Before(candidates[right].record.CreatedAt)
	})
	token, err := opaqueToken()
	if err != nil {
		return accountsync.NewMailNotificationClaim{}, err
	}
	claim := accountsync.NewMailNotificationClaim{Token: token, Notifications: []accountsync.NewMailNotification{}}
	for _, candidate := range candidates {
		if len(claim.Notifications) >= limit {
			break
		}
		relevant, ready, err := store.notificationStillRelevant(ctx, candidate.record)
		if err != nil {
			return accountsync.NewMailNotificationClaim{}, err
		}
		if !ready {
			continue
		}
		current := candidate.record
		replacement := current
		if !relevant {
			replacement.State, replacement.ClaimToken, replacement.ClaimHolder = "delivered", "", ""
			replacement.ClaimExpiresAt, replacement.CompletedAt = nil, timePointer(now)
			replacement.UpdatedAt = now
			if err := store.documents.patch(ctx, ProviderNotifications, candidate.documentID, replacement, current, store.documents.schedulerWriteAccess()); err != nil {
				return accountsync.NewMailNotificationClaim{}, err
			}
			continue
		}
		expires := now.Add(ttl).UTC()
		replacement.State, replacement.ClaimToken, replacement.ClaimHolder = "claimed", token, holder
		replacement.ClaimExpiresAt, replacement.UpdatedAt = &expires, now
		replacement.Attempts++
		if err := store.documents.patch(ctx, ProviderNotifications, candidate.documentID, replacement, current, store.documents.schedulerWriteAccess()); err != nil {
			return accountsync.NewMailNotificationClaim{}, err
		}
		claim.Notifications = append(claim.Notifications, replacement.notification())
	}
	if len(claim.Notifications) == 0 {
		claim.Token = ""
	}
	return claim, nil
}

func (store *AccountSync) CompleteNewMailNotifications(ctx context.Context, token string, ids []string, delivered bool, code string, now time.Time) error {
	if token == "" || len(token) > 128 || len(ids) < 1 || len(ids) > 100 || !delivered && providerDeliveryCode(code, "") == "" {
		return accounts.ErrInvalid
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !strings.HasPrefix(id, "pnot_") || len(id) > 64 || wanted[id] {
			return accounts.ErrInvalid
		}
		wanted[id] = true
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderNotifications, store.documents.schedulerReadAccess())
	if err != nil {
		return err
	}
	completed := 0
	for documentID, raw := range documents {
		var current providerNotificationRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider notification: %w", err)
		}
		if !wanted[current.ID] {
			continue
		}
		if current.State != "claimed" || current.ClaimToken != token {
			return accounts.ErrConflict
		}
		replacement := current
		replacement.ClaimToken, replacement.ClaimHolder, replacement.LastCode = "", "", ""
		replacement.ClaimExpiresAt, replacement.UpdatedAt = nil, now
		if delivered {
			replacement.State, replacement.CompletedAt = "delivered", timePointer(now)
		} else {
			replacement.State, replacement.LastCode = "queued", providerDeliveryCode(code, "delivery_failed")
			delay := 30 * time.Second * time.Duration(1<<min(int(replacement.Attempts), 7))
			if delay > time.Hour {
				delay = time.Hour
			}
			replacement.NotBefore = now.Add(delay).UTC()
		}
		if err := store.documents.patch(ctx, ProviderNotifications, documentID, replacement, current, store.documents.schedulerWriteAccess()); err != nil {
			return err
		}
		completed++
	}
	if completed != len(wanted) {
		return accounts.ErrNotFound
	}
	return nil
}

func (store *AccountSync) notificationStillRelevant(ctx context.Context, notification providerNotificationRecord) (bool, bool, error) {
	owner := accounts.Owner{TenantID: notification.TenantID, PrincipalID: notification.PrincipalID, Address: notification.OwnerAddress}
	scope := accountsync.Scope{Owner: owner, AccountID: notification.AccountID}
	message, _, err := store.getMessageRecord(ctx, scope, notification.StableID)
	if errors.Is(err, accounts.ErrNotFound) {
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	if hasProviderFlag(message.Flags, "\\Seen") {
		return false, true, nil
	}
	inbox, err := store.inboxFolderIDs(ctx, scope)
	if err != nil {
		return false, false, err
	}
	return messageInFolders(message.FolderStableID, message.FolderStableIDs, inbox), true, nil
}

func providerNotificationID(scope accountsync.Scope, stableID string) string {
	digest := sha256.Sum256([]byte(scope.Owner.TenantID + "\x00" + scope.Owner.PrincipalID + "\x00" + string(scope.AccountID) + "\x00" + stableID))
	return "pnot_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func providerDeliveryCode(value, fallback string) string {
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

func (store *AccountSync) upsertFolder(ctx context.Context, scope accountsync.Scope, folder accountsync.Folder) error {
	documents, err := store.documents.find(ctx, ProviderFolders, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	replacement := newFolderRecord(scope, folder)
	for id, raw := range documents {
		var current folderRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.StableID == folder.StableID && current.AccountID == scope.AccountID && recordOwner(current.TenantID, current.PrincipalID, scope) {
			if current.Revision == folder.Revision {
				return nil
			}
			if folder.ResetMessages {
				if err := store.deleteFolderMessages(ctx, scope, folder.StableID); err != nil {
					return err
				}
			}
			return store.documents.patch(ctx, ProviderFolders, id, replacement, current, store.documents.writeAccess(scope.Owner, scope.AccountID))
		}
	}
	_, err = store.documents.put(ctx, ProviderFolders, replacement, store.documents.writeAccess(scope.Owner, scope.AccountID))
	return err
}

func (store *AccountSync) reconcileFolders(ctx context.Context, scope accountsync.Scope, folders []accountsync.Folder) error {
	present := make(map[string]bool, len(folders))
	for _, folder := range folders {
		present[folder.StableID] = true
	}
	documents, err := store.documents.find(ctx, ProviderFolders, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current folderRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) || present[current.StableID] {
			continue
		}
		if err := store.deleteFolderMessages(ctx, scope, current.StableID); err != nil {
			return err
		}
		if err := store.documents.delete(ctx, ProviderFolders, id, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
			return err
		}
	}
	return nil
}

func (store *AccountSync) updateMessage(ctx context.Context, scope accountsync.Scope, update accountsync.MessageUpdate) error {
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if current.StableID != update.StableID || current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) {
			continue
		}
		if current.Revision == update.Revision {
			return nil
		}
		old := current
		current.Revision, current.Flags = update.Revision, append([]string(nil), update.Flags...)
		if update.RemoteStableID != "" {
			current.RemoteStableID = update.RemoteStableID
		}
		if len(update.FolderStableIDs) > 0 {
			current.FolderStableIDs = append([]string(nil), update.FolderStableIDs...)
		}
		return store.documents.patch(ctx, ProviderMessages, id, current, old, store.documents.writeAccess(scope.Owner, scope.AccountID))
	}
	// A UID can be discovered by the delta scan before its bounded raw import.
	// The ordinary new-message pass will create it on this or the next run.
	return nil
}

func (store *AccountSync) reconcileWindow(ctx context.Context, scope accountsync.Scope, window accountsync.ReconcileWindow) error {
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	present := make(map[string]bool, len(window.PresentIDs))
	for _, stableID := range window.PresentIDs {
		present[stableID] = true
	}
	for id, raw := range documents {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return fmt.Errorf("decode FYLO provider message: %w", err)
		}
		if !messageMissingFromWindow(current, scope, window, present) {
			continue
		}
		if err := store.deleteMessageRecord(ctx, scope, id, current); err != nil {
			return err
		}
	}
	return nil
}

func messageMissingFromWindow(current messageRecord, scope accountsync.Scope, window accountsync.ReconcileWindow, present map[string]bool) bool {
	identity := current.StableID
	if current.RemoteStableID != "" {
		identity = current.RemoteStableID
	}
	uidValidity, uid, ok := providerMessageUID(identity)
	return current.AccountID == scope.AccountID && current.FolderStableID == window.FolderStableID && recordOwner(current.TenantID, current.PrincipalID, scope) &&
		ok && uidValidity == window.UIDValidity && uid >= window.StartUID && uid <= window.EndUID && !present[identity]
}

func (store *AccountSync) upsertMessage(ctx context.Context, scope accountsync.Scope, message accountsync.Message, notify bool) error {
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	documentID, current, err := findMessageDocument(documents, scope, message.StableID)
	if err != nil {
		return err
	}
	if documentID != "" && current.Revision == message.Revision {
		return nil
	}
	// Queue first so a storage failure cannot create a projected message with
	// no durable notification. The deterministic outbox identity makes a retry
	// harmless; claim processing waits until the message projection exists.
	if documentID == "" && notify {
		if err := store.ensureNewMailNotification(ctx, scope, message); err != nil {
			return err
		}
	}
	rawID, err := store.putRaw(ctx, scope, message)
	if err != nil {
		return err
	}
	replacement := newMessageRecord(scope, message, rawID)
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	if err := store.persistMessageRecord(ctx, access, documentID, current, replacement); err != nil {
		return err
	}
	if current.RawBucketID != "" {
		_ = store.documents.delete(ctx, ProviderRawFiles, current.RawBucketID, access)
	}
	return nil
}

func findMessageDocument(documents map[string]json.RawMessage, scope accountsync.Scope, stableID string) (string, messageRecord, error) {
	for id, raw := range documents {
		var candidate messageRecord
		if err := json.Unmarshal(raw, &candidate); err != nil {
			return "", messageRecord{}, err
		}
		if candidate.StableID == stableID && ownedMessageRecord(candidate, scope) {
			return id, candidate, nil
		}
	}
	return "", messageRecord{}, nil
}

func (store *AccountSync) persistMessageRecord(ctx context.Context, access map[string]any, documentID string, current, replacement messageRecord) error {
	if documentID == "" {
		_, err := store.documents.put(ctx, ProviderMessages, replacement, access)
		return err
	}
	return store.documents.patch(ctx, ProviderMessages, documentID, replacement, current, access)
}

func (store *AccountSync) inboxFolderIDs(ctx context.Context, scope accountsync.Scope) (map[string]bool, error) {
	documents, err := store.documents.find(ctx, ProviderFolders, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, raw := range documents {
		var record folderRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider folder: %w", err)
		}
		if record.AccountID != scope.AccountID || !recordOwner(record.TenantID, record.PrincipalID, scope) {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(record.Role))
		if role == "inbox" || role == "" && strings.EqualFold(record.Name, "INBOX") {
			result[record.StableID] = true
		}
	}
	return result, nil
}

func (store *AccountSync) deleteMessage(ctx context.Context, scope accountsync.Scope, deletion accountsync.Deletion) error {
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		matches := current.StableID == deletion.StableID || deletion.RemoteStableID != "" && current.RemoteStableID == deletion.RemoteStableID
		if !matches || current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) {
			continue
		}
		return store.deleteMessageRecord(ctx, scope, id, current)
	}
	return nil
}

func (store *AccountSync) deleteMessageRecord(ctx context.Context, scope accountsync.Scope, documentID string, current messageRecord) error {
	access := store.documents.writeAccess(scope.Owner, scope.AccountID)
	if err := store.documents.delete(ctx, ProviderMessages, documentID, access); err != nil {
		return err
	}
	if current.RawBucketID != "" {
		return store.documents.delete(ctx, ProviderRawFiles, current.RawBucketID, access)
	}
	return nil
}

func (store *AccountSync) deleteFolderMessages(ctx context.Context, scope accountsync.Scope, folder string) error {
	documents, err := store.documents.find(ctx, ProviderMessages, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current messageRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.AccountID != scope.AccountID || !recordOwner(current.TenantID, current.PrincipalID, scope) || !recordInFolder(current, folder) {
			continue
		}
		if len(current.FolderStableIDs) > 1 {
			old := current
			current.FolderStableIDs = removeString(current.FolderStableIDs, folder)
			if current.FolderStableID == folder {
				current.FolderStableID = current.FolderStableIDs[0]
			}
			if err := store.documents.patch(ctx, ProviderMessages, id, current, old, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
				return err
			}
			continue
		}
		if err := store.deleteMessageRecord(ctx, scope, id, current); err != nil {
			return err
		}
	}
	return nil
}

func providerMessageUID(stableID string) (uint64, uint64, bool) {
	_, uidValidity, uid, ok := providerMessageIdentity(stableID)
	return uidValidity, uid, ok
}

func providerMessageIdentity(stableID string) (string, uint64, uint64, bool) {
	parts := strings.Split(stableID, ":")
	if len(parts) < 3 {
		return "", 0, 0, false
	}
	uidValidity, errA := strconv.ParseUint(parts[len(parts)-2], 10, 64)
	uid, errB := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	folder := strings.Join(parts[:len(parts)-2], ":")
	return folder, uidValidity, uid, errA == nil && errB == nil && folder != "" && uidValidity > 0 && uid > 0
}

func projectFlag(flags []string, target string, enabled bool) []string {
	result := make([]string, 0, len(flags)+1)
	found := false
	for _, flag := range flags {
		if strings.EqualFold(flag, target) {
			found = true
			if enabled {
				result = append(result, target)
			}
			continue
		}
		result = append(result, flag)
	}
	if enabled && !found {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func (store *AccountSync) putRaw(ctx context.Context, scope accountsync.Scope, message accountsync.Message) (string, error) {
	file, err := os.CreateTemp(store.stagingRoot, "provider-message-*.eml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return "", err
	}
	if _, err := file.Write(message.RawRFC822); err != nil {
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
	request := map[string]any{
		"op": "putData", "collection": ProviderRawFiles, "file": map[string]any{"path": path},
		"fileOptions": map[string]any{"maxBytes": rawMessageMaxBytes},
		"meta":        map[string]any{"accountId": scope.AccountID, "stableId": message.StableID, "revision": message.Revision},
	}
	addAccess(request, store.documents.writeAccess(scope.Owner, scope.AccountID))
	result, err := store.documents.call(ctx, request)
	if err != nil {
		return "", err
	}
	var id string
	if err := json.Unmarshal(result, &id); err != nil || id == "" {
		return "", fmt.Errorf("decode FYLO provider raw identifier: %w", err)
	}
	return id, nil
}

func (store *AccountSync) getRaw(ctx context.Context, scope accountsync.Scope, id string) ([]byte, error) {
	if id == "" {
		return nil, accounts.ErrNotFound
	}
	request := map[string]any{"op": "getFileData", "collection": ProviderRawFiles, "id": id}
	addAccess(request, store.documents.readAccess(scope.Owner))
	result, err := store.documents.call(ctx, request)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data          string `json:"data"`
		ContentLength int    `json:"contentLength"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.Data == "" || payload.ContentLength < 1 || payload.ContentLength > rawMessageMaxBytes {
		return nil, errors.New("invalid FYLO provider message payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil || len(decoded) != payload.ContentLength {
		return nil, errors.New("invalid FYLO provider message encoding")
	}
	return decoded, nil
}

func (store *AccountSync) findState(ctx context.Context, scope accountsync.Scope) (string, syncStateRecord, error) {
	documents, err := store.documents.find(ctx, ProviderSyncState, store.documents.readAccess(scope.Owner))
	if err != nil {
		return "", syncStateRecord{}, err
	}
	for id, raw := range documents {
		var state syncStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return "", syncStateRecord{}, err
		}
		if state.AccountID == scope.AccountID && recordOwner(state.TenantID, state.PrincipalID, scope) {
			return id, state, nil
		}
	}
	return "", syncStateRecord{}, accounts.ErrNotFound
}

func (store *AccountSync) mutationRecords(ctx context.Context, scope accountsync.Scope) ([]mutationRecord, error) {
	documents, err := store.documents.find(ctx, ProviderMutations, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []mutationRecord{}
	for _, raw := range documents {
		var record mutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		record.Mutation.Scope = accountsync.Scope{Owner: accounts.Owner{TenantID: record.TenantID, PrincipalID: record.PrincipalID, Address: record.OwnerAddress}, AccountID: record.AccountID}
		if recordOwner(record.TenantID, record.PrincipalID, scope) && record.AccountID == scope.AccountID {
			result = append(result, record)
		}
	}
	return result, nil
}

type syncStateRecord struct {
	AccountID       accounts.AccountID `json:"accountId"`
	TenantID        string             `json:"tenantId"`
	PrincipalID     string             `json:"principalId"`
	OwnerAddress    string             `json:"ownerAddress"`
	LeaseToken      string             `json:"leaseToken,omitempty"`
	LeaseHolder     string             `json:"leaseHolder,omitempty"`
	LeaseExpiresAt  *time.Time         `json:"leaseExpiresAt,omitempty"`
	Cursor          string             `json:"cursor"`
	CursorVersion   uint64             `json:"cursorVersion"`
	CursorUpdatedAt *time.Time         `json:"cursorUpdatedAt,omitempty"`
}

type folderRecord struct {
	accountsync.Folder
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
}

func newFolderRecord(scope accountsync.Scope, folder accountsync.Folder) folderRecord {
	return folderRecord{Folder: folder, AccountID: scope.AccountID, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address}
}

type messageRecord struct {
	AccountID       accounts.AccountID `json:"accountId"`
	TenantID        string             `json:"tenantId"`
	PrincipalID     string             `json:"principalId"`
	OwnerAddress    string             `json:"ownerAddress"`
	StableID        string             `json:"stableId"`
	RemoteStableID  string             `json:"remoteStableId,omitempty"`
	Revision        string             `json:"revision"`
	FolderStableID  string             `json:"folderStableId"`
	FolderStableIDs []string           `json:"folderStableIds,omitempty"`
	RFCMessageID    string             `json:"rfcMessageId,omitempty"`
	InReplyTo       string             `json:"inReplyTo,omitempty"`
	References      []string           `json:"references,omitempty"`
	Subject         string             `json:"subject"`
	Sender          string             `json:"sender"`
	To              []string           `json:"to,omitempty"`
	CC              []string           `json:"cc,omitempty"`
	BCC             []string           `json:"bcc,omitempty"`
	Snippet         string             `json:"snippet,omitempty"`
	AttachmentNames []string           `json:"attachmentNames,omitempty"`
	ReceivedAt      time.Time          `json:"receivedAt"`
	Flags           []string           `json:"flags"`
	Size            uint64             `json:"size"`
	RawBucketID     string             `json:"rawBucketId"`
}

type providerNotificationRecord struct {
	ID             string             `json:"id"`
	AccountID      accounts.AccountID `json:"accountId"`
	TenantID       string             `json:"tenantId"`
	PrincipalID    string             `json:"principalId"`
	OwnerAddress   string             `json:"ownerAddress"`
	StableID       string             `json:"stableId"`
	Sender         string             `json:"sender"`
	Subject        string             `json:"subject"`
	ReceivedAt     time.Time          `json:"receivedAt"`
	State          string             `json:"state"`
	Attempts       uint16             `json:"attempts"`
	NotBefore      time.Time          `json:"notBefore"`
	ClaimToken     string             `json:"claimToken,omitempty"`
	ClaimHolder    string             `json:"claimHolder,omitempty"`
	ClaimExpiresAt *time.Time         `json:"claimExpiresAt,omitempty"`
	LastCode       string             `json:"lastCode,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	CompletedAt    *time.Time         `json:"completedAt,omitempty"`
}

func (record providerNotificationRecord) notification() accountsync.NewMailNotification {
	return accountsync.NewMailNotification{
		ID: record.ID, AccountID: record.AccountID, OwnerAddress: record.OwnerAddress,
		StableID: record.StableID, Sender: record.Sender, Subject: record.Subject,
		ReceivedAt: record.ReceivedAt, Attempt: record.Attempts,
	}
}

func newMessageRecord(scope accountsync.Scope, message accountsync.Message, rawID string) messageRecord {
	return messageRecord{AccountID: scope.AccountID, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address,
		StableID: message.StableID, RemoteStableID: message.RemoteStableID, Revision: message.Revision, FolderStableID: message.FolderStableID, FolderStableIDs: append([]string{}, message.FolderStableIDs...), RFCMessageID: message.RFCMessageID,
		InReplyTo: message.InReplyTo, References: append([]string{}, message.References...),
		Subject: message.Subject, Sender: message.Sender, To: append([]string{}, message.To...), CC: append([]string{}, message.CC...), BCC: append([]string{}, message.BCC...),
		Snippet: message.Snippet, AttachmentNames: append([]string{}, message.AttachmentNames...), ReceivedAt: message.ReceivedAt, Flags: message.Flags, Size: message.Size, RawBucketID: rawID}
}

func (record messageRecord) message() accountsync.Message {
	return accountsync.Message{StableID: record.StableID, RemoteStableID: record.RemoteStableID, Revision: record.Revision, FolderStableID: record.FolderStableID, FolderStableIDs: append([]string{}, record.FolderStableIDs...),
		RFCMessageID: record.RFCMessageID, InReplyTo: record.InReplyTo, References: append([]string{}, record.References...), Subject: record.Subject, Sender: record.Sender, To: append([]string{}, record.To...),
		CC: append([]string{}, record.CC...), BCC: append([]string{}, record.BCC...), Snippet: record.Snippet,
		AttachmentNames: append([]string{}, record.AttachmentNames...), ReceivedAt: record.ReceivedAt, Flags: append([]string{}, record.Flags...), Size: record.Size}
}

func hasProviderFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if strings.EqualFold(flag, target) {
			return true
		}
	}
	return false
}

func messageContains(message accountsync.Message, query string) bool {
	values := []string{message.Sender, message.Subject, message.Snippet, strings.Join(message.To, " "), strings.Join(message.CC, " "), strings.Join(message.BCC, " "), strings.Join(message.AttachmentNames, " ")}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

type mutationRecord struct {
	accountsync.Mutation
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
}

func newMutationRecord(mutation accountsync.Mutation) mutationRecord {
	return mutationRecord{Mutation: mutation, AccountID: mutation.Scope.AccountID, TenantID: mutation.Scope.Owner.TenantID, PrincipalID: mutation.Scope.Owner.PrincipalID, OwnerAddress: mutation.Scope.Owner.Address}
}

func validLease(state syncStateRecord, lease accountsync.Lease, now time.Time) bool {
	return state.LeaseToken == lease.Token && state.LeaseHolder == lease.Holder && state.LeaseExpiresAt != nil && state.LeaseExpiresAt.After(now)
}

func recordOwner(tenantID, principalID string, scope accountsync.Scope) bool {
	return tenantID == scope.Owner.TenantID && principalID == scope.Owner.PrincipalID
}

func mutationTransition(current, next accountsync.MutationState) bool {
	allowed := map[accountsync.MutationState]map[accountsync.MutationState]bool{
		accountsync.MutationQueued:      {accountsync.MutationApplying: true},
		accountsync.MutationApplying:    {accountsync.MutationQueued: true, accountsync.MutationReconciling: true, accountsync.MutationApplied: true, accountsync.MutationFailed: true},
		accountsync.MutationReconciling: {accountsync.MutationReconciling: true, accountsync.MutationApplied: true, accountsync.MutationFailed: true},
	}
	return allowed[current][next]
}

func opaqueToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
