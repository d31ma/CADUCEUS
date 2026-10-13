// Package fylostore contains typed FYLO adapters. Adapters in this package use
// the one process supervised by the Go runtime; they never open a FYLO root.
package fylostore

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/platform/fylo"
)

const (
	ProviderAccounts          = "provider-accounts"
	ProviderCredentials       = "provider-credentials"
	ProviderOAuthTransactions = "provider-oauth-transactions"
	ProviderSyncState         = "provider-sync-state"
	ProviderFolders           = "provider-folders"
	ProviderMessages          = "provider-messages"
	ProviderNotifications     = "provider-notifications"
	ProviderMutations         = "provider-mutations"
	ProviderSubmissions       = "provider-submissions"
	ProviderSchedule          = "provider-sync-schedule"
	ProviderRawFiles          = "provider-raw-files"
	ProviderSubmitFiles       = "provider-submission-files"
	ProviderContactState      = "provider-contact-state"
	ProviderContactBooks      = "provider-contact-books"
	ProviderContacts          = "provider-contacts"
	ProviderContactMutations  = "provider-contact-mutations"
	ProviderCalendarState     = "provider-calendar-state"
	ProviderCalendars         = "provider-calendars"
	ProviderCalendarResources = "provider-calendar-resources"
	ProviderCalendarMutations = "provider-calendar-mutations"
	posixIDMin                = 100_000
	posixIDSpan               = 2_000_000_000
	providerMode              = 0o660
)

type Caller interface {
	Call(context.Context, map[string]any) (fylo.Response, error)
}

type Accounts struct {
	caller      Caller
	posixAccess bool
	mutation    sync.Mutex
}

func NewAccounts(caller Caller, posixAccess ...bool) *Accounts {
	enabled := true
	if len(posixAccess) > 0 {
		enabled = posixAccess[0]
	}
	return &Accounts{caller: caller, posixAccess: enabled}
}

// EnsureProviderCollections creates the private collections consumed only by
// Go account and sync adapters. They are intentionally absent from the legacy
// JavaScript bridge allowlist.
func EnsureProviderCollections(ctx context.Context, caller Caller) error {
	repository := NewAccounts(caller)
	for _, collection := range []string{ProviderAccounts, ProviderCredentials, ProviderOAuthTransactions, ProviderSyncState, ProviderFolders, ProviderMessages, ProviderNotifications, ProviderMutations, ProviderSubmissions, ProviderSchedule, ProviderContactState, ProviderContactBooks, ProviderContacts, ProviderContactMutations, ProviderCalendarState, ProviderCalendars, ProviderCalendarResources, ProviderCalendarMutations} {
		_, err := repository.call(ctx, map[string]any{
			"op": "createCollection", "collection": collection, "kind": "document",
		})
		if err != nil {
			return fmt.Errorf("ensure FYLO collection %s: %w", collection, err)
		}
	}
	if _, err := repository.call(ctx, map[string]any{
		"op": "createCollection", "collection": ProviderRawFiles, "kind": "file",
	}); err != nil {
		return fmt.Errorf("ensure FYLO collection %s: %w", ProviderRawFiles, err)
	}
	if _, err := repository.call(ctx, map[string]any{
		"op": "createCollection", "collection": ProviderSubmitFiles, "kind": "file",
	}); err != nil {
		return fmt.Errorf("ensure FYLO collection %s: %w", ProviderSubmitFiles, err)
	}
	return nil
}

func (repository *Accounts) Create(ctx context.Context, account accounts.ExternalAccount, secret accounts.EncryptedSecret) (accounts.ExternalAccount, error) {
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	existing, err := repository.List(ctx, account.Owner)
	if err != nil {
		return accounts.ExternalAccount{}, err
	}
	for _, candidate := range existing {
		if candidate.ID == account.ID || candidate.Provider == account.Provider && candidate.MailboxAddress == account.MailboxAddress && candidate.State != accounts.StateDeleting {
			return accounts.ExternalAccount{}, accounts.ErrConflict
		}
	}
	access := repository.writeAccess(account.Owner, account.ID)
	credentialID, err := repository.put(ctx, ProviderCredentials, newCredentialRecord(account, secret), access)
	if err != nil {
		return accounts.ExternalAccount{}, err
	}
	accountDocumentID, err := repository.put(ctx, ProviderAccounts, newAccountRecord(account), access)
	if err != nil {
		cleanupErr := repository.delete(ctx, ProviderCredentials, credentialID, access)
		return accounts.ExternalAccount{}, errors.Join(err, cleanupErr)
	}
	if _, err := repository.put(ctx, ProviderSchedule, newScheduleRecord(account), repository.schedulerWriteAccess()); err != nil {
		cleanupAccount := repository.delete(ctx, ProviderAccounts, accountDocumentID, access)
		cleanupCredential := repository.delete(ctx, ProviderCredentials, credentialID, access)
		return accounts.ExternalAccount{}, errors.Join(err, cleanupAccount, cleanupCredential)
	}
	return account, nil
}

func (repository *Accounts) Get(ctx context.Context, owner accounts.Owner, id accounts.AccountID) (accounts.ExternalAccount, error) {
	_, account, err := repository.findAccount(ctx, owner, id)
	return account, err
}

func (repository *Accounts) List(ctx context.Context, owner accounts.Owner) ([]accounts.ExternalAccount, error) {
	documents, err := repository.find(ctx, ProviderAccounts, repository.readAccess(owner))
	if err != nil {
		return nil, err
	}
	result := make([]accounts.ExternalAccount, 0, len(documents))
	for _, raw := range documents {
		record, decodeErr := decodeAccountRecord(raw)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if exactOwner(record.ExternalAccount.Owner, owner) {
			result = append(result, record.ExternalAccount)
		}
	}
	return result, nil
}

func (repository *Accounts) Update(ctx context.Context, replacement accounts.ExternalAccount, expected uint64) (accounts.ExternalAccount, error) {
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	documentID, current, err := repository.findAccount(ctx, replacement.Owner, replacement.ID)
	if err != nil {
		return accounts.ExternalAccount{}, err
	}
	if current.Revision != expected {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	if replacement.Revision != expected+1 || !exactOwner(current.Owner, replacement.Owner) {
		return accounts.ExternalAccount{}, accounts.ErrInvalid
	}
	request := map[string]any{
		"op": "patchDoc", "collection": ProviderAccounts, "id": documentID,
		"newDoc": newAccountRecord(replacement), "oldDoc": newAccountRecord(current),
	}
	addAccess(request, repository.writeAccess(replacement.Owner, replacement.ID))
	if _, err := repository.call(ctx, request); err != nil {
		return accounts.ExternalAccount{}, err
	}
	return replacement, nil
}

func (repository *Accounts) ReplaceCredential(ctx context.Context, replacement accounts.ExternalAccount, secret accounts.EncryptedSecret, expected uint64) (accounts.ExternalAccount, error) {
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	accountDocumentID, current, err := repository.findAccount(ctx, replacement.Owner, replacement.ID)
	if err != nil {
		return accounts.ExternalAccount{}, err
	}
	if current.Revision != expected {
		return accounts.ExternalAccount{}, accounts.ErrConflict
	}
	if replacement.Revision != expected+1 || !exactOwner(current.Owner, replacement.Owner) || current.Provider != replacement.Provider {
		return accounts.ExternalAccount{}, accounts.ErrInvalid
	}
	credentialDocumentID, currentCredential, err := repository.findCredential(ctx, replacement.Owner, replacement.ID, replacement.Provider)
	if err != nil {
		return accounts.ExternalAccount{}, err
	}
	access := repository.writeAccess(replacement.Owner, replacement.ID)
	oldCredential := credentialRecordFromEnvelope(replacement, currentCredential)
	newCredential := credentialRecordFromEnvelope(replacement, secret)
	if err := repository.patch(ctx, ProviderCredentials, credentialDocumentID, newCredential, oldCredential, access); err != nil {
		return accounts.ExternalAccount{}, err
	}
	if err := repository.patch(ctx, ProviderAccounts, accountDocumentID, newAccountRecord(replacement), newAccountRecord(current), access); err != nil {
		rollback := repository.patch(ctx, ProviderCredentials, credentialDocumentID, oldCredential, newCredential, access)
		return accounts.ExternalAccount{}, errors.Join(err, rollback)
	}
	return replacement, nil
}

func (repository *Accounts) MarkDeleting(ctx context.Context, replacement accounts.ExternalAccount, expected uint64) error {
	if replacement.State != accounts.StateDeleting {
		return accounts.ErrInvalid
	}
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	accountDocumentID, current, err := repository.findAccount(ctx, replacement.Owner, replacement.ID)
	if err != nil {
		return err
	}
	if current.Revision != expected {
		return accounts.ErrConflict
	}
	if replacement.Revision != expected+1 || !exactOwner(current.Owner, replacement.Owner) {
		return accounts.ErrInvalid
	}
	credentialDocumentID, _, err := repository.findCredential(ctx, replacement.Owner, replacement.ID, replacement.Provider)
	if err != nil {
		return err
	}
	scheduleDocumentID, currentSchedule, err := repository.findSchedule(ctx, replacement.Owner, replacement.ID)
	if err != nil {
		return err
	}
	replacementSchedule := currentSchedule
	now := time.Now().UTC()
	replacementSchedule.NextRunAt = now
	replacementSchedule.Attempts = 0
	replacementSchedule.LastCode = ""
	replacementSchedule.UpdatedAt = now
	if err := repository.patch(ctx, ProviderSchedule, scheduleDocumentID, replacementSchedule, currentSchedule, repository.schedulerWriteAccess()); err != nil {
		return err
	}
	access := repository.writeAccess(replacement.Owner, replacement.ID)
	if err := repository.patch(ctx, ProviderAccounts, accountDocumentID, newAccountRecord(replacement), newAccountRecord(current), access); err != nil {
		rollback := repository.patch(ctx, ProviderSchedule, scheduleDocumentID, currentSchedule, replacementSchedule, repository.schedulerWriteAccess())
		return errors.Join(err, rollback)
	}
	return repository.delete(ctx, ProviderCredentials, credentialDocumentID, access)
}

// FinalizeDelete removes the account control records after the sync worker has
// purged all account-owned data under a valid lease. The public account record
// is deleted last so an interrupted cleanup remains discoverable and retryable.
func (repository *Accounts) FinalizeDelete(ctx context.Context, owner accounts.Owner, id accounts.AccountID) error {
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	accountDocumentID, current, err := repository.findAccount(ctx, owner, id)
	if err != nil {
		return err
	}
	if current.State != accounts.StateDeleting {
		return accounts.ErrConflict
	}
	access := repository.writeAccess(owner, id)
	if credentialDocumentID, _, findErr := repository.findCredential(ctx, owner, id, current.Provider); findErr == nil {
		if err := repository.delete(ctx, ProviderCredentials, credentialDocumentID, access); err != nil {
			return err
		}
	} else if !errors.Is(findErr, accounts.ErrNotFound) {
		return findErr
	}
	if scheduleDocumentID, _, findErr := repository.findSchedule(ctx, owner, id); findErr == nil {
		if err := repository.delete(ctx, ProviderSchedule, scheduleDocumentID, repository.schedulerWriteAccess()); err != nil {
			return err
		}
	} else if !errors.Is(findErr, accounts.ErrNotFound) {
		return findErr
	}
	return repository.delete(ctx, ProviderAccounts, accountDocumentID, access)
}

func (repository *Accounts) GetSecret(ctx context.Context, owner accounts.Owner, id accounts.AccountID, provider accounts.Provider) (accounts.EncryptedSecret, error) {
	_, secret, err := repository.findCredential(ctx, owner, id, provider)
	return secret, err
}

// RotateSecret compare-and-swaps only the encrypted credential envelope. OAuth
// refresh is worker-owned and must not create a browser-visible account
// revision or race a reconnect that supplied a different envelope.
func (repository *Accounts) RotateSecret(ctx context.Context, account accounts.ExternalAccount, oldSecret, replacement accounts.EncryptedSecret) error {
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	documentID, current, err := repository.findCredential(ctx, account.Owner, account.ID, account.Provider)
	if err != nil {
		return err
	}
	if current != oldSecret {
		return accounts.ErrConflict
	}
	return repository.patch(ctx, ProviderCredentials, documentID,
		credentialRecordFromEnvelope(account, replacement), credentialRecordFromEnvelope(account, oldSecret),
		repository.writeAccess(account.Owner, account.ID))
}

// Put persists one short-lived owner-bound OAuth transaction. Only the state
// digest and encrypted PKCE verifier are retained.
func (repository *Accounts) Put(ctx context.Context, transaction accounts.OAuthTransaction) error {
	if !oauthProvider(transaction.Provider) || len(transaction.StateHash) != 64 || transaction.VerifierID == "" ||
		transaction.CreatedAt.IsZero() || !transaction.ExpiresAt.After(transaction.CreatedAt) {
		return accounts.ErrOAuthTransaction
	}
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	documents, err := repository.find(ctx, ProviderOAuthTransactions, repository.readAccess(transaction.Owner))
	if err != nil {
		return err
	}
	// Keep at most one pending authorization per exact principal. Starting a
	// new flow invalidates older browser tabs and bounds abandoned transaction
	// storage without waiting for an external cleanup service.
	for documentID, raw := range documents {
		record, decodeErr := decodeOAuthTransactionRecord(raw)
		if decodeErr != nil {
			return decodeErr
		}
		if !exactOwner(record.Owner, transaction.Owner) {
			continue
		}
		if err := repository.delete(ctx, ProviderOAuthTransactions, documentID, repository.oauthWriteAccess(record.Owner, record.StateHash)); err != nil {
			return err
		}
	}
	_, err = repository.put(ctx, ProviderOAuthTransactions, newOAuthTransactionRecord(transaction), repository.oauthWriteAccess(transaction.Owner, transaction.StateHash))
	return err
}

// Take atomically consumes a transaction for the exact SESAME principal. A
// foreign, expired, unknown, or replayed state is intentionally indistinct.
func (repository *Accounts) Take(ctx context.Context, owner accounts.Owner, provider accounts.Provider, stateHash string, now time.Time) (accounts.OAuthTransaction, error) {
	if !oauthProvider(provider) || len(stateHash) != 64 {
		return accounts.OAuthTransaction{}, accounts.ErrOAuthTransaction
	}
	repository.mutation.Lock()
	defer repository.mutation.Unlock()
	documents, err := repository.find(ctx, ProviderOAuthTransactions, repository.readAccess(owner))
	if err != nil {
		return accounts.OAuthTransaction{}, err
	}
	for documentID, raw := range documents {
		record, decodeErr := decodeOAuthTransactionRecord(raw)
		if decodeErr != nil {
			return accounts.OAuthTransaction{}, decodeErr
		}
		transaction := record.OAuthTransaction
		if transaction.StateHash != stateHash || transaction.Provider != provider || !exactOwner(transaction.Owner, owner) {
			continue
		}
		if err := repository.delete(ctx, ProviderOAuthTransactions, documentID, repository.oauthWriteAccess(owner, stateHash)); err != nil {
			return accounts.OAuthTransaction{}, err
		}
		if !transaction.ExpiresAt.After(now) {
			return accounts.OAuthTransaction{}, accounts.ErrOAuthTransaction
		}
		return transaction, nil
	}
	return accounts.OAuthTransaction{}, accounts.ErrOAuthTransaction
}

func (repository *Accounts) findCredential(ctx context.Context, owner accounts.Owner, id accounts.AccountID, provider accounts.Provider) (string, accounts.EncryptedSecret, error) {
	documents, err := repository.find(ctx, ProviderCredentials, repository.readAccess(owner))
	if err != nil {
		return "", accounts.EncryptedSecret{}, err
	}
	for documentID, raw := range documents {
		record, decodeErr := decodeCredentialRecord(raw)
		if decodeErr != nil {
			return "", accounts.EncryptedSecret{}, decodeErr
		}
		if record.AccountID == id && record.Provider == provider && record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID {
			return documentID, accounts.EncryptedSecret{KeyVersion: record.KeyVersion, Nonce: record.Nonce, Ciphertext: record.Ciphertext}, nil
		}
	}
	return "", accounts.EncryptedSecret{}, accounts.ErrNotFound
}

func (repository *Accounts) findAccount(ctx context.Context, owner accounts.Owner, id accounts.AccountID) (string, accounts.ExternalAccount, error) {
	documents, err := repository.find(ctx, ProviderAccounts, repository.readAccess(owner))
	if err != nil {
		return "", accounts.ExternalAccount{}, err
	}
	for documentID, raw := range documents {
		record, decodeErr := decodeAccountRecord(raw)
		if decodeErr != nil {
			return "", accounts.ExternalAccount{}, decodeErr
		}
		if record.ID == id && exactOwner(record.ExternalAccount.Owner, owner) {
			return documentID, record.ExternalAccount, nil
		}
	}
	return "", accounts.ExternalAccount{}, accounts.ErrNotFound
}

func (repository *Accounts) findSchedule(ctx context.Context, owner accounts.Owner, id accounts.AccountID) (string, scheduleRecord, error) {
	documents, err := repository.find(ctx, ProviderSchedule, repository.schedulerReadAccess())
	if err != nil {
		return "", scheduleRecord{}, err
	}
	for documentID, raw := range documents {
		var record scheduleRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return "", scheduleRecord{}, fmt.Errorf("decode FYLO provider schedule: %w", err)
		}
		if record.AccountID == id && record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID {
			return documentID, record, nil
		}
	}
	return "", scheduleRecord{}, accounts.ErrNotFound
}

func (repository *Accounts) put(ctx context.Context, collection string, data any, access map[string]any) (string, error) {
	request := map[string]any{"op": "putData", "collection": collection, "data": data}
	addAccess(request, access)
	result, err := repository.call(ctx, request)
	if err != nil {
		return "", err
	}
	var id string
	if err := json.Unmarshal(result, &id); err != nil || id == "" {
		return "", fmt.Errorf("decode FYLO %s identifier: %w", collection, err)
	}
	return id, nil
}

func (repository *Accounts) find(ctx context.Context, collection string, access map[string]any) (map[string]json.RawMessage, error) {
	request := map[string]any{"op": "findDocs", "collection": collection, "query": map[string]any{"$ops": []any{}}}
	addAccess(request, access)
	result, err := repository.call(ctx, request)
	if err != nil {
		return nil, err
	}
	documents := map[string]json.RawMessage{}
	if err := json.Unmarshal(result, &documents); err != nil {
		return nil, fmt.Errorf("decode FYLO %s documents: %w", collection, err)
	}
	return documents, nil
}

func (repository *Accounts) delete(ctx context.Context, collection, id string, access map[string]any) error {
	request := map[string]any{"op": "delDoc", "collection": collection, "id": id}
	addAccess(request, access)
	_, err := repository.call(ctx, request)
	return err
}

func (repository *Accounts) patch(ctx context.Context, collection, id string, newDoc, oldDoc any, access map[string]any) error {
	request := map[string]any{
		"op": "patchDoc", "collection": collection, "id": id,
		"newDoc": newDoc, "oldDoc": oldDoc,
	}
	addAccess(request, access)
	_, err := repository.call(ctx, request)
	return err
}

func (repository *Accounts) call(ctx context.Context, request map[string]any) (json.RawMessage, error) {
	if repository == nil || repository.caller == nil {
		return nil, errors.New("FYLO account repository caller is required")
	}
	response, err := repository.caller.Call(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.OK {
		return response.Result, nil
	}
	if response.Error != nil {
		return nil, response.Error
	}
	return nil, errors.New("FYLO account operation failed")
}

type accountRecord struct {
	accounts.ExternalAccount
	TenantID     string `json:"tenantId"`
	PrincipalID  string `json:"principalId"`
	OwnerAddress string `json:"ownerAddress"`
}

func newAccountRecord(account accounts.ExternalAccount) accountRecord {
	return accountRecord{
		ExternalAccount: account,
		TenantID:        account.Owner.TenantID, PrincipalID: account.Owner.PrincipalID, OwnerAddress: account.Owner.Address,
	}
}

func decodeAccountRecord(raw json.RawMessage) (accountRecord, error) {
	var record accountRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return accountRecord{}, fmt.Errorf("decode FYLO provider account: %w", err)
	}
	record.ExternalAccount.Owner = accounts.Owner{
		TenantID: record.TenantID, PrincipalID: record.PrincipalID, Address: record.OwnerAddress,
	}
	return record, nil
}

type credentialRecord struct {
	AccountID   accounts.AccountID `json:"accountId"`
	TenantID    string             `json:"tenantId"`
	PrincipalID string             `json:"principalId"`
	Provider    accounts.Provider  `json:"provider"`
	KeyVersion  uint32             `json:"keyVersion"`
	Nonce       string             `json:"nonce"`
	Ciphertext  string             `json:"ciphertext"`
}

type oauthTransactionRecord struct {
	accounts.OAuthTransaction
	TenantID     string `json:"tenantId"`
	PrincipalID  string `json:"principalId"`
	OwnerAddress string `json:"ownerAddress"`
}

func newOAuthTransactionRecord(transaction accounts.OAuthTransaction) oauthTransactionRecord {
	return oauthTransactionRecord{OAuthTransaction: transaction, TenantID: transaction.Owner.TenantID, PrincipalID: transaction.Owner.PrincipalID, OwnerAddress: transaction.Owner.Address}
}

func decodeOAuthTransactionRecord(raw json.RawMessage) (oauthTransactionRecord, error) {
	var record oauthTransactionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return oauthTransactionRecord{}, fmt.Errorf("decode FYLO provider OAuth transaction: %w", err)
	}
	record.OAuthTransaction.Owner = accounts.Owner{TenantID: record.TenantID, PrincipalID: record.PrincipalID, Address: record.OwnerAddress}
	return record, nil
}

type scheduleRecord struct {
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
	Provider     accounts.Provider  `json:"provider"`
	NextRunAt    time.Time          `json:"nextRunAt"`
	Attempts     uint16             `json:"attempts"`
	LastCode     string             `json:"lastCode,omitempty"`
	CreatedAt    time.Time          `json:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt"`
}

func newScheduleRecord(account accounts.ExternalAccount) scheduleRecord {
	return scheduleRecord{AccountID: account.ID, TenantID: account.Owner.TenantID, PrincipalID: account.Owner.PrincipalID, OwnerAddress: account.Owner.Address,
		Provider: account.Provider, NextRunAt: account.CreatedAt, CreatedAt: account.CreatedAt, UpdatedAt: account.CreatedAt}
}

func newCredentialRecord(account accounts.ExternalAccount, secret accounts.EncryptedSecret) credentialRecord {
	return credentialRecordFromEnvelope(account, secret)
}

func credentialRecordFromEnvelope(account accounts.ExternalAccount, secret accounts.EncryptedSecret) credentialRecord {
	return credentialRecord{
		AccountID: account.ID, TenantID: account.Owner.TenantID, PrincipalID: account.Owner.PrincipalID,
		Provider: account.Provider, KeyVersion: secret.KeyVersion, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext,
	}
}

func decodeCredentialRecord(raw json.RawMessage) (credentialRecord, error) {
	var record credentialRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return credentialRecord{}, fmt.Errorf("decode FYLO provider credential: %w", err)
	}
	return record, nil
}

func (repository *Accounts) readAccess(owner accounts.Owner) map[string]any {
	if !repository.posixAccess {
		return nil
	}
	return map[string]any{"uid": principalUID(owner)}
}

func (repository *Accounts) writeAccess(owner accounts.Owner, id accounts.AccountID) map[string]any {
	if !repository.posixAccess {
		return nil
	}
	return map[string]any{
		"uid":  principalUID(owner),
		"gid":  virtualID("provider-account", owner.TenantID+"\x00"+string(id)),
		"mode": providerMode,
	}
}

func (repository *Accounts) oauthWriteAccess(owner accounts.Owner, stateHash string) map[string]any {
	if !repository.posixAccess {
		return nil
	}
	uid := principalUID(owner)
	return map[string]any{"uid": uid, "gid": virtualID("provider-oauth", owner.TenantID+"\x00"+owner.PrincipalID+"\x00"+stateHash), "mode": 0o600}
}

func (repository *Accounts) schedulerReadAccess() map[string]any {
	if !repository.posixAccess {
		return nil
	}
	return map[string]any{"uid": schedulerUID()}
}

func (repository *Accounts) schedulerWriteAccess() map[string]any {
	if !repository.posixAccess {
		return nil
	}
	id := schedulerUID()
	return map[string]any{"uid": id, "gid": id, "mode": 0o600}
}

func schedulerUID() int { return virtualID("caduceus-scheduler", "provider-sync") }

func addAccess(request map[string]any, access map[string]any) {
	if access != nil {
		request["access"] = access
	}
}

func principalUID(owner accounts.Owner) int {
	return virtualID("sesame-principal", owner.TenantID+"\x00"+owner.PrincipalID)
}

func virtualID(namespace, key string) int {
	digest := sha256.Sum256([]byte(namespace + "\x00" + key))
	return posixIDMin + int(binary.BigEndian.Uint32(digest[:4])%posixIDSpan)
}

func exactOwner(left, right accounts.Owner) bool {
	return left.TenantID == right.TenantID && left.PrincipalID == right.PrincipalID
}

func oauthProvider(provider accounts.Provider) bool {
	return provider == accounts.ProviderMicrosoft || provider == accounts.ProviderGmail
}
