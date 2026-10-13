package contactsync

import (
	"context"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestJobJournalsConditionalConflictThenRefreshesRemoteSnapshot(t *testing.T) {
	now := time.Now().UTC()
	scope := accountsync.Scope{Owner: accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}
	current := Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: scope.AccountID, AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/a.vcf", ETag: `"8"`, Name: "Remote", RawVCard: "BEGIN:VCARD\r\nFN:Remote\r\nEND:VCARD\r\n", RemoteUpdatedAt: now}
	mutation := Mutation{MutationRequest: MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "edit-1", Kind: MutationUpdate, ContactID: current.ID, ExpectedETag: `"7"`, Desired: Contact{Name: "Local"}}, Scope: scope, State: MutationQueued, CreatedAt: now, UpdatedAt: now}
	store := &jobStore{current: current, pending: []Mutation{mutation}}
	connector := &jobConnector{outcome: OutcomeConflict, snapshot: Snapshot{PrincipalURL: "https://dav.example/principal/", HomeURL: "https://dav.example/books/", SyncedAt: now,
		AddressBooks: []AddressBook{{StableID: current.AddressBookID, URL: "https://dav.example/books/default/", Name: "Contacts"}}, Contacts: []Contact{current}}}
	job := NewJob(store, connector)
	account := accounts.ExternalAccount{ID: scope.AccountID, Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}}
	if err := job.RunWithLease(context.Background(), accountsync.Lease{Scope: scope, Token: "lease"}, account, accounts.Credentials{Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if len(store.updates) != 2 || store.updates[0].State != MutationApplying || store.updates[1].State != MutationConflict || store.updates[1].LastCode != "remote_changed" || store.commits != 1 || connector.syncCalls != 1 {
		t.Fatalf("updates=%#v commits=%d sync=%d", store.updates, store.commits, connector.syncCalls)
	}
}

func TestJobCreatesContactInAnActiveAddressBookBeforeRefreshing(t *testing.T) {
	now := time.Now().UTC()
	scope := accountsync.Scope{Owner: accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}
	book := AddressBook{StableID: "book_abcdefghijklmnopqrstuvwx", URL: "https://dav.example/books/default/", Name: "Contacts"}
	desired := Contact{AccountID: scope.AccountID, AddressBookID: book.StableID, Href: book.URL + "caduceus-a.vcf", UID: "uid-a", Name: "Created"}
	desired.ID = RemoteContactID(scope.AccountID, desired.Href)
	mutation := Mutation{MutationRequest: MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "create-1", Kind: MutationCreate, ContactID: desired.ID, Desired: desired}, Scope: scope, State: MutationQueued, CreatedAt: now, UpdatedAt: now}
	store := &jobStore{state: State{AddressBooks: []AddressBook{book}}, pending: []Mutation{mutation}}
	connector := &jobConnector{outcome: OutcomeApplied, snapshot: Snapshot{PrincipalURL: "https://dav.example/principal/", HomeURL: "https://dav.example/books/", SyncedAt: now, AddressBooks: []AddressBook{book}}}
	job := NewJob(store, connector)
	account := accounts.ExternalAccount{ID: scope.AccountID, Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}}
	if err := job.RunWithLease(context.Background(), accountsync.Lease{Scope: scope, Token: "lease"}, account, accounts.Credentials{Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if store.providerLoads != 0 || connector.current.ID != desired.ID || connector.current.AddressBookID != book.StableID || len(store.updates) != 2 || store.updates[1].State != MutationApplied {
		t.Fatalf("loads=%d current=%#v updates=%#v", store.providerLoads, connector.current, store.updates)
	}
}

type jobStore struct {
	current       Contact
	state         State
	pending       []Mutation
	updates       []Mutation
	commits       int
	providerLoads int
}

func (store *jobStore) ContactState(context.Context, accountsync.Scope) (State, error) {
	return store.state, nil
}
func (store *jobStore) CommitContactSnapshot(context.Context, accountsync.Lease, Snapshot) error {
	store.commits++
	return nil
}
func (*jobStore) ListProviderContacts(context.Context, accounts.Owner) ([]Contact, error) {
	return nil, nil
}
func (*jobStore) ListProviderAddressBooks(context.Context, accounts.Owner) ([]OwnedAddressBook, error) {
	return nil, nil
}
func (store *jobStore) ProviderContact(context.Context, accountsync.Scope, string) (Contact, error) {
	store.providerLoads++
	return store.current, nil
}
func (*jobStore) EnqueueContactMutation(context.Context, accountsync.Scope, Mutation) (Mutation, error) {
	return Mutation{}, nil
}
func (store *jobStore) PendingContactMutations(context.Context, accountsync.Lease, int) ([]Mutation, error) {
	return store.pending, nil
}
func (store *jobStore) UpdateContactMutation(_ context.Context, _ accountsync.Lease, mutation Mutation) error {
	store.updates = append(store.updates, mutation)
	return nil
}
func (*jobStore) ContactMutation(context.Context, accountsync.Scope, string) (Mutation, error) {
	return Mutation{}, nil
}

type jobConnector struct {
	outcome   MutationOutcome
	snapshot  Snapshot
	syncCalls int
	current   Contact
}

func (connector *jobConnector) Sync(context.Context, accounts.ExternalAccount, accounts.Credentials, State) (Snapshot, error) {
	connector.syncCalls++
	return connector.snapshot, nil
}
func (connector *jobConnector) ApplyMutation(_ context.Context, _ accounts.ExternalAccount, _ accounts.Credentials, _ Mutation, current Contact) (MutationOutcome, error) {
	connector.current = current
	return connector.outcome, nil
}
