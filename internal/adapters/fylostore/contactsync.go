package fylostore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/contactsync"
)

// ContactState returns the active CardDAV discovery and collection cursors for
// one exact account owner. A never-synced account has an empty state.
func (store *AccountSync) ContactState(ctx context.Context, scope accountsync.Scope) (contactsync.State, error) {
	_, record, err := store.findContactState(ctx, scope)
	if errors.Is(err, accounts.ErrNotFound) {
		return contactsync.State{}, nil
	}
	if err != nil {
		return contactsync.State{}, err
	}
	books, err := store.contactBooks(ctx, scope, record.ActiveGeneration)
	if err != nil {
		return contactsync.State{}, err
	}
	contacts, err := store.providerContacts(ctx, scope, record.ActiveGeneration, false)
	if err != nil {
		return contactsync.State{}, err
	}
	return contactsync.State{PrincipalURL: record.PrincipalURL, HomeURL: record.HomeURL, AddressBooks: books, Contacts: contacts, LastSyncedAt: record.LastSyncedAt, LastCode: record.LastCode}, nil
}

// CommitContactSnapshot uses a generation switch: all replacement documents
// are written first, then one state patch atomically makes the generation
// visible. Cleanup of older generations happens only after that visibility
// boundary, so readers never observe a partial address book.
func (store *AccountSync) CommitContactSnapshot(ctx context.Context, lease accountsync.Lease, snapshot contactsync.Snapshot) error {
	if err := contactsync.ValidateSnapshot(lease.Scope, snapshot); err != nil {
		return err
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	_, syncState, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(syncState, lease, time.Now().UTC()) {
		return accountsync.ErrLeaseHeld
	}
	stateID, current, err := store.findContactState(ctx, lease.Scope)
	if err != nil && !errors.Is(err, accounts.ErrNotFound) {
		return err
	}
	generation, err := opaqueToken()
	if err != nil {
		return err
	}
	access := store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID)
	for _, book := range snapshot.AddressBooks {
		if _, err := store.documents.put(ctx, ProviderContactBooks, newContactBookRecord(lease.Scope, generation, book), access); err != nil {
			return err
		}
	}
	for _, contact := range snapshot.Contacts {
		if _, err := store.documents.put(ctx, ProviderContacts, newProviderContactRecord(lease.Scope, generation, contact), access); err != nil {
			return err
		}
	}
	replacement := contactStateRecord{AccountID: lease.Scope.AccountID, TenantID: lease.Scope.Owner.TenantID, PrincipalID: lease.Scope.Owner.PrincipalID, OwnerAddress: lease.Scope.Owner.Address,
		PrincipalURL: snapshot.PrincipalURL, HomeURL: snapshot.HomeURL, ActiveGeneration: generation, LastSyncedAt: timePointer(snapshot.SyncedAt.UTC())}
	if stateID == "" {
		if _, err := store.documents.put(ctx, ProviderContactState, replacement, access); err != nil {
			return err
		}
	} else if err := store.documents.patch(ctx, ProviderContactState, stateID, replacement, current, access); err != nil {
		return err
	}
	if err := store.deleteInactiveContactGeneration(ctx, ProviderContactBooks, lease.Scope, generation); err != nil {
		return err
	}
	return store.deleteInactiveContactGeneration(ctx, ProviderContacts, lease.Scope, generation)
}

func (store *AccountSync) ListProviderContacts(ctx context.Context, owner accounts.Owner) ([]contactsync.Contact, error) {
	stateDocuments, err := store.documents.find(ctx, ProviderContactState, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	active := make(map[accounts.AccountID]string)
	for _, raw := range stateDocuments {
		var state contactStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, fmt.Errorf("decode FYLO provider contact state: %w", err)
		}
		if state.TenantID == owner.TenantID && state.PrincipalID == owner.PrincipalID {
			active[state.AccountID] = state.ActiveGeneration
		}
	}
	documents, err := store.documents.find(ctx, ProviderContacts, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	result := make([]contactsync.Contact, 0)
	for _, raw := range documents {
		var record providerContactRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider contact: %w", err)
		}
		if record.TenantID != owner.TenantID || record.PrincipalID != owner.PrincipalID || active[record.Contact.AccountID] != record.Generation {
			continue
		}
		contact := record.Contact
		contact.RawVCard = ""
		result = append(result, contact)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Name == result[right].Name {
			return result[left].ID < result[right].ID
		}
		return result[left].Name < result[right].Name
	})
	return result, nil
}

// ListProviderAddressBooks returns active CardDAV collections for one exact
// authenticated owner. The bridge projects only stable IDs and display names;
// collection URLs and sync cursors never cross the server boundary.
func (store *AccountSync) ListProviderAddressBooks(ctx context.Context, owner accounts.Owner) ([]contactsync.OwnedAddressBook, error) {
	stateDocuments, err := store.documents.find(ctx, ProviderContactState, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	active := make(map[accounts.AccountID]string)
	for _, raw := range stateDocuments {
		var state contactStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, fmt.Errorf("decode FYLO provider contact state: %w", err)
		}
		if state.TenantID == owner.TenantID && state.PrincipalID == owner.PrincipalID {
			active[state.AccountID] = state.ActiveGeneration
		}
	}
	documents, err := store.documents.find(ctx, ProviderContactBooks, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	result := make([]contactsync.OwnedAddressBook, 0)
	for _, raw := range documents {
		var record contactBookRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider contact address book: %w", err)
		}
		if record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID && active[record.AccountID] == record.Generation {
			result = append(result, contactsync.OwnedAddressBook{AccountID: record.AccountID, AddressBook: record.AddressBook})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].AccountID == result[right].AccountID {
			if result[left].Name == result[right].Name {
				return result[left].StableID < result[right].StableID
			}
			return result[left].Name < result[right].Name
		}
		return result[left].AccountID < result[right].AccountID
	})
	return result, nil
}

func (store *AccountSync) ProviderContact(ctx context.Context, scope accountsync.Scope, contactID string) (contactsync.Contact, error) {
	_, state, err := store.findContactState(ctx, scope)
	if err != nil {
		return contactsync.Contact{}, err
	}
	contacts, err := store.providerContacts(ctx, scope, state.ActiveGeneration, false)
	if err != nil {
		return contactsync.Contact{}, err
	}
	for _, contact := range contacts {
		if contact.ID == contactID {
			return contact, nil
		}
	}
	return contactsync.Contact{}, contactsync.ErrContactNotFound
}

func (store *AccountSync) EnqueueContactMutation(ctx context.Context, scope accountsync.Scope, mutation contactsync.Mutation) (contactsync.Mutation, error) {
	if err := contactsync.ValidateMutation(mutation.MutationRequest); err != nil {
		return contactsync.Mutation{}, err
	}
	if mutation.Kind == contactsync.MutationCreate && mutation.Desired.AccountID != scope.AccountID {
		return contactsync.Mutation{}, contactsync.ErrInvalidMutation
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderContactMutations, store.documents.readAccess(scope.Owner))
	if err != nil {
		return contactsync.Mutation{}, err
	}
	for _, raw := range documents {
		var record contactMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return contactsync.Mutation{}, err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID && record.IdempotencyKey == mutation.IdempotencyKey {
			record.Mutation.Scope = scope
			storedRequest, candidateRequest := record.Mutation.MutationRequest, mutation.MutationRequest
			storedRequest.ID, candidateRequest.ID = "", ""
			if reflect.DeepEqual(storedRequest, candidateRequest) {
				return record.Mutation, nil
			}
			return contactsync.Mutation{}, accounts.ErrConflict
		}
	}
	mutation.Scope = scope
	if mutation.State == "" {
		mutation.State = contactsync.MutationQueued
	}
	if _, err := store.documents.put(ctx, ProviderContactMutations, newContactMutationRecord(mutation), store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return contactsync.Mutation{}, err
	}
	return mutation, nil
}

func (store *AccountSync) PendingContactMutations(ctx context.Context, lease accountsync.Lease, limit int) ([]contactsync.Mutation, error) {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return nil, err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return nil, accountsync.ErrLeaseHeld
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	documents, err := store.documents.find(ctx, ProviderContactMutations, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []contactsync.Mutation{}
	for _, raw := range documents {
		var record contactMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		if record.AccountID == lease.Scope.AccountID && record.TenantID == lease.Scope.Owner.TenantID && record.PrincipalID == lease.Scope.Owner.PrincipalID && (record.State == contactsync.MutationQueued || record.State == contactsync.MutationApplying) {
			record.Mutation.Scope = lease.Scope
			result = append(result, record.Mutation)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].CreatedAt.Before(result[right].CreatedAt) })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (store *AccountSync) UpdateContactMutation(ctx context.Context, lease accountsync.Lease, replacement contactsync.Mutation) error {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) || replacement.Scope != lease.Scope {
		return accountsync.ErrLeaseHeld
	}
	documents, err := store.documents.find(ctx, ProviderContactMutations, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current contactMutationRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.ID != replacement.ID || current.AccountID != lease.Scope.AccountID || current.TenantID != lease.Scope.Owner.TenantID || current.PrincipalID != lease.Scope.Owner.PrincipalID {
			continue
		}
		if !validContactMutationTransition(current.State, replacement.State) {
			return contactsync.ErrInvalidMutation
		}
		return store.documents.patch(ctx, ProviderContactMutations, id, newContactMutationRecord(replacement), current, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID))
	}
	return contactsync.ErrContactNotFound
}

func (store *AccountSync) ContactMutation(ctx context.Context, scope accountsync.Scope, mutationID string) (contactsync.Mutation, error) {
	documents, err := store.documents.find(ctx, ProviderContactMutations, store.documents.readAccess(scope.Owner))
	if err != nil {
		return contactsync.Mutation{}, err
	}
	for _, raw := range documents {
		var record contactMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return contactsync.Mutation{}, err
		}
		if record.ID == mutationID && record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID {
			record.Mutation.Scope = scope
			return record.Mutation, nil
		}
	}
	return contactsync.Mutation{}, contactsync.ErrContactNotFound
}

func (store *AccountSync) providerContacts(ctx context.Context, scope accountsync.Scope, generation string, redact bool) ([]contactsync.Contact, error) {
	documents, err := store.documents.find(ctx, ProviderContacts, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := make([]contactsync.Contact, 0)
	for _, raw := range documents {
		var record providerContactRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider contact: %w", err)
		}
		if record.TenantID != scope.Owner.TenantID || record.PrincipalID != scope.Owner.PrincipalID || record.Contact.AccountID != scope.AccountID || record.Generation != generation {
			continue
		}
		contact := record.Contact
		contact.RawVCard = record.RawVCard
		if redact {
			contact.RawVCard = ""
		}
		result = append(result, contact)
	}
	return result, nil
}

func (store *AccountSync) findContactState(ctx context.Context, scope accountsync.Scope) (string, contactStateRecord, error) {
	documents, err := store.documents.find(ctx, ProviderContactState, store.documents.readAccess(scope.Owner))
	if err != nil {
		return "", contactStateRecord{}, err
	}
	for id, raw := range documents {
		var state contactStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return "", contactStateRecord{}, err
		}
		if state.AccountID == scope.AccountID && state.TenantID == scope.Owner.TenantID && state.PrincipalID == scope.Owner.PrincipalID {
			return id, state, nil
		}
	}
	return "", contactStateRecord{}, accounts.ErrNotFound
}

func (store *AccountSync) contactBooks(ctx context.Context, scope accountsync.Scope, generation string) ([]contactsync.AddressBook, error) {
	documents, err := store.documents.find(ctx, ProviderContactBooks, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []contactsync.AddressBook{}
	for _, raw := range documents {
		var record contactBookRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID && record.Generation == generation {
			result = append(result, record.AddressBook)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].StableID < result[right].StableID })
	return result, nil
}

func (store *AccountSync) deleteInactiveContactGeneration(ctx context.Context, collection string, scope accountsync.Scope, active string) error {
	documents, err := store.documents.find(ctx, collection, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var identity struct {
			AccountID   accounts.AccountID `json:"accountId"`
			TenantID    string             `json:"tenantId"`
			PrincipalID string             `json:"principalId"`
			Generation  string             `json:"generation"`
		}
		if err := json.Unmarshal(raw, &identity); err != nil {
			return err
		}
		if identity.AccountID == scope.AccountID && identity.TenantID == scope.Owner.TenantID && identity.PrincipalID == scope.Owner.PrincipalID && identity.Generation != active {
			if err := store.documents.delete(ctx, collection, id, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
				return err
			}
		}
	}
	return nil
}

type contactStateRecord struct {
	AccountID        accounts.AccountID `json:"accountId"`
	TenantID         string             `json:"tenantId"`
	PrincipalID      string             `json:"principalId"`
	OwnerAddress     string             `json:"ownerAddress"`
	PrincipalURL     string             `json:"principalUrl"`
	HomeURL          string             `json:"homeUrl"`
	ActiveGeneration string             `json:"activeGeneration"`
	LastSyncedAt     *time.Time         `json:"lastSyncedAt,omitempty"`
	LastCode         string             `json:"lastCode,omitempty"`
}

type contactBookRecord struct {
	contactsync.AddressBook
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
	Generation   string             `json:"generation"`
}

type providerContactRecord struct {
	contactsync.Contact
	RawVCard     string `json:"rawVCard"`
	TenantID     string `json:"tenantId"`
	PrincipalID  string `json:"principalId"`
	OwnerAddress string `json:"ownerAddress"`
	Generation   string `json:"generation"`
}

type contactMutationRecord struct {
	contactsync.Mutation
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
}

func newContactBookRecord(scope accountsync.Scope, generation string, book contactsync.AddressBook) contactBookRecord {
	return contactBookRecord{AddressBook: book, AccountID: scope.AccountID, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address, Generation: generation}
}

func newProviderContactRecord(scope accountsync.Scope, generation string, contact contactsync.Contact) providerContactRecord {
	return providerContactRecord{Contact: contact, RawVCard: contact.RawVCard, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address, Generation: generation}
}

func newContactMutationRecord(mutation contactsync.Mutation) contactMutationRecord {
	return contactMutationRecord{Mutation: mutation, AccountID: mutation.Scope.AccountID, TenantID: mutation.Scope.Owner.TenantID, PrincipalID: mutation.Scope.Owner.PrincipalID, OwnerAddress: mutation.Scope.Owner.Address}
}

func validContactMutationTransition(current, next contactsync.MutationState) bool {
	allowed := map[contactsync.MutationState]map[contactsync.MutationState]bool{
		contactsync.MutationQueued:   {contactsync.MutationApplying: true},
		contactsync.MutationApplying: {contactsync.MutationApplying: true, contactsync.MutationApplied: true, contactsync.MutationConflict: true, contactsync.MutationFailed: true},
	}
	return allowed[current][next]
}

func timePointer(value time.Time) *time.Time { return &value }
