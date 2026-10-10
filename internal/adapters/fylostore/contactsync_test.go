package fylostore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestContactSnapshotGenerationIsolatedAndReplacesStaleContacts(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, err := store.AcquireLease(context.Background(), scope, "contact-worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	first := contactSnapshot(scope.AccountID, now, "carddav_alice", "Alice")
	if err := store.CommitContactSnapshot(context.Background(), lease, first); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListProviderContacts(context.Background(), scope.Owner)
	if err != nil || len(listed) != 1 || listed[0].Name != "Alice" || listed[0].RawVCard != "" {
		t.Fatalf("first list=%#v err=%v", listed, err)
	}
	second := contactSnapshot(scope.AccountID, now.Add(time.Second), "carddav_bob", "Bob")
	if err := store.CommitContactSnapshot(context.Background(), lease, second); err != nil {
		t.Fatal(err)
	}
	listed, err = store.ListProviderContacts(context.Background(), scope.Owner)
	if err != nil || len(listed) != 1 || listed[0].ID != "carddav_bob" {
		t.Fatalf("replacement list=%#v err=%v", listed, err)
	}
	if countCollection(machine, ProviderContactState) != 1 || countCollection(machine, ProviderContactBooks) != 1 || countCollection(machine, ProviderContacts) != 1 {
		t.Fatalf("collections state=%d books=%d contacts=%d", countCollection(machine, ProviderContactState), countCollection(machine, ProviderContactBooks), countCollection(machine, ProviderContacts))
	}
	books, err := store.ListProviderAddressBooks(context.Background(), scope.Owner)
	if err != nil || len(books) != 1 || books[0].AccountID != scope.AccountID || books[0].StableID != "book_default" || books[0].URL != "https://dav.example/books/alice/default/" {
		t.Fatalf("redacted address books=%#v err=%v", books, err)
	}
	foreign := scope.Owner
	foreign.PrincipalID = "mallory"
	foreign.Address = "mallory@example.com"
	if contacts, err := store.ListProviderContacts(context.Background(), foreign); err != nil || len(contacts) != 0 {
		t.Fatalf("foreign list=%#v err=%v", contacts, err)
	}
	state, err := store.ContactState(context.Background(), scope)
	if err != nil || len(state.AddressBooks) != 1 || len(state.Contacts) != 1 || state.Contacts[0].RawVCard == "" || state.LastSyncedAt == nil || state.AddressBooks[0].StableID != "book_default" {
		t.Fatalf("state=%#v err=%v", state, err)
	}
}

func TestContactMutationJournalIsIdempotentOwnerScopedAndLeaseGuarded(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "contact-worker", now, time.Minute)
	if err := store.CommitContactSnapshot(context.Background(), lease, contactSnapshot(scope.AccountID, now, "carddav_alice", "Alice")); err != nil {
		t.Fatal(err)
	}
	mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "contact-edit-1", Kind: contactsync.MutationUpdate, ContactID: "carddav_alice", ExpectedETag: `"7"`, Desired: contactsync.Contact{Name: "Alice Updated"}}, State: contactsync.MutationQueued, CreatedAt: now, UpdatedAt: now}
	queued, err := store.EnqueueContactMutation(context.Background(), scope, mutation)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.EnqueueContactMutation(context.Background(), scope, mutation)
	if err != nil || replayed.ID != queued.ID || countCollection(machine, ProviderContactMutations) != 1 {
		t.Fatalf("replay=%#v err=%v count=%d", replayed, err, countCollection(machine, ProviderContactMutations))
	}
	retry := mutation
	retry.ID = "cmut_zyxwvutsrqponmlkjihgfedc"
	replayed, err = store.EnqueueContactMutation(context.Background(), scope, retry)
	if err != nil || replayed.ID != queued.ID || countCollection(machine, ProviderContactMutations) != 1 {
		t.Fatalf("semantic replay=%#v err=%v count=%d", replayed, err, countCollection(machine, ProviderContactMutations))
	}
	pending, err := store.PendingContactMutations(context.Background(), lease, 10)
	if err != nil || len(pending) != 1 || pending[0].Scope != scope {
		t.Fatalf("pending=%#v err=%v", pending, err)
	}
	pending[0].State = contactsync.MutationApplying
	if err := store.UpdateContactMutation(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	pending[0].State = contactsync.MutationApplied
	if err := store.UpdateContactMutation(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	foreign := scope
	foreign.Owner.PrincipalID = "mallory"
	foreign.Owner.Address = "mallory@example.com"
	if _, err := store.ProviderContact(context.Background(), foreign, "carddav_alice"); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign contact error=%v", err)
	}
	if books, err := store.ListProviderAddressBooks(context.Background(), foreign.Owner); err != nil || len(books) != 0 {
		t.Fatalf("foreign address books=%#v err=%v", books, err)
	}
}

func contactSnapshot(accountID accounts.AccountID, now time.Time, id, name string) contactsync.Snapshot {
	return contactsync.Snapshot{PrincipalURL: "https://dav.example/principals/alice/", HomeURL: "https://dav.example/books/alice/", SyncedAt: now,
		AddressBooks: []contactsync.AddressBook{{StableID: "book_default", URL: "https://dav.example/books/alice/default/", Name: "Contacts", SyncToken: "sync-7"}},
		Contacts: []contactsync.Contact{{ID: id, AccountID: accountID, AddressBookID: "book_default", Href: "/books/alice/default/" + id + ".vcf", ETag: `"7"`, Name: name,
			Emails: []contactsync.Value{{Value: name + "@example.com"}}, RawVCard: "BEGIN:VCARD\r\nFN:" + name + "\r\nEND:VCARD\r\n", RemoteUpdatedAt: now}},
	}
}
