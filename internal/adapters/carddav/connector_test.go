package carddav

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestConnectorAppliesIncrementalChangesAndDeletionsToActiveSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	bookURL := "https://dav.example/books/default/"
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/", Username: "alice"}}}
	bookID := stableID("book", bookURL)
	client := &fakeProtocolClient{discovery: Discovery{PrincipalURL: "https://dav.example/principals/alice/", HomeURL: "https://dav.example/books/", AddressBooks: []AddressBook{{URL: bookURL, Name: "Contacts", SyncToken: "server-current"}}},
		delta: Delta{SyncToken: "sync-8", Deleted: []string{bookURL + "gone.vcf"}, Changed: []RemoteContact{{Href: bookURL + "alice.vcf", ETag: `"8"`, VCard: "BEGIN:VCARD\r\nFN:Alice Updated\r\nEMAIL:alice@example.com\r\nEND:VCARD\r\n"}}}}
	state := contactsync.State{AddressBooks: []contactsync.AddressBook{{StableID: bookID, URL: bookURL, Name: "Contacts", SyncToken: "sync-7"}}, Contacts: []contactsync.Contact{
		{ID: stableID("carddav", string(account.ID)+"\x00"+bookURL+"alice.vcf"), AccountID: account.ID, AddressBookID: bookID, Href: bookURL + "alice.vcf", Name: "Alice", RawVCard: "old", RemoteUpdatedAt: now.Add(-time.Hour)},
		{ID: stableID("carddav", string(account.ID)+"\x00"+bookURL+"gone.vcf"), AccountID: account.ID, AddressBookID: bookID, Href: bookURL + "gone.vcf", Name: "Gone", RawVCard: "old", RemoteUpdatedAt: now.Add(-time.Hour)},
	}}
	snapshot, err := NewConnector(client, func() time.Time { return now }).Sync(context.Background(), account, accounts.Credentials{Password: "secret"}, state)
	if err != nil {
		t.Fatal(err)
	}
	if client.incrementalCalls != 1 || client.initialCalls != 0 || client.previousToken != "sync-7" || len(snapshot.AddressBooks) != 1 || snapshot.AddressBooks[0].SyncToken != "sync-8" || len(snapshot.Contacts) != 1 || snapshot.Contacts[0].Name != "Alice Updated" || snapshot.Contacts[0].ETag != `"8"` {
		t.Fatalf("snapshot=%#v incremental=%d initial=%d token=%q", snapshot, client.incrementalCalls, client.initialCalls, client.previousToken)
	}
}

func TestConnectorCreatesWithIfNoneMatchAndRecognizesItsOwnCrashRetry(t *testing.T) {
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/", Username: "alice"}}}
	desired := contactsync.Contact{AccountID: account.ID, AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/books/default/caduceus-a.vcf", UID: "uid-a", Name: "Alice", Emails: []contactsync.Value{{Value: "alice@example.com"}}}
	desired.ID = contactsync.RemoteContactID(account.ID, desired.Href)
	mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "create-1", Kind: contactsync.MutationCreate, ContactID: desired.ID, Desired: desired}, Scope: accountsync.Scope{AccountID: account.ID}, State: contactsync.MutationQueued}
	client := &fakeProtocolClient{}
	outcome, err := NewConnector(client).ApplyMutation(context.Background(), account, accounts.Credentials{Password: "secret"}, mutation, desired)
	if err != nil || outcome != contactsync.OutcomeApplied || !client.putCreate || client.putTarget != desired.Href || client.putETag != "" || !strings.Contains(client.putBody, "UID:uid-a") {
		t.Fatalf("outcome=%q err=%v create=%t target=%q etag=%q body=%q", outcome, err, client.putCreate, client.putTarget, client.putETag, client.putBody)
	}

	client.putErr = ErrPrecondition
	client.fetched = RemoteContact{Href: desired.Href, ETag: `"8"`, VCard: "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:uid-a\r\nFN:Alice\r\nEMAIL:alice@example.com\r\nEND:VCARD\r\n"}
	mutation.State = contactsync.MutationApplying
	outcome, err = NewConnector(client).ApplyMutation(context.Background(), account, accounts.Credentials{Password: "secret"}, mutation, desired)
	if err != nil || outcome != contactsync.OutcomeApplied || client.fetchCalls != 1 {
		t.Fatalf("retry outcome=%q err=%v fetches=%d", outcome, err, client.fetchCalls)
	}
}

type fakeProtocolClient struct {
	discovery        Discovery
	delta            Delta
	discoveryErr     error
	deltaErr         error
	initial          []RemoteContact
	initialCalls     int
	incrementalCalls int
	previousToken    string
	putETag          string
	putTarget        string
	putBody          string
	putCreate        bool
	putErr           error
	deleteETag       string
	fetched          RemoteContact
	fetchErr         error
	fetchCalls       int
}

func (client *fakeProtocolClient) Discover(context.Context, *accounts.CardDAVEndpoint, accounts.Credentials) (Discovery, error) {
	return client.discovery, client.discoveryErr
}

func (client *fakeProtocolClient) InitialSync(context.Context, AddressBook, Auth) ([]RemoteContact, error) {
	client.initialCalls++
	return client.initial, nil
}

func (client *fakeProtocolClient) IncrementalSync(_ context.Context, _ AddressBook, token string, _ Auth) (Delta, error) {
	client.incrementalCalls++
	client.previousToken = token
	return client.delta, client.deltaErr
}

func (client *fakeProtocolClient) Put(_ context.Context, target string, body string, etag string, create bool, _ Auth) (string, error) {
	client.putTarget, client.putBody, client.putETag, client.putCreate = target, body, etag, create
	return `"next"`, client.putErr
}

func (client *fakeProtocolClient) Delete(_ context.Context, _ string, etag string, _ Auth) error {
	client.deleteETag = etag
	return nil
}

func (client *fakeProtocolClient) Fetch(context.Context, string, Auth) (RemoteContact, error) {
	client.fetchCalls++
	return client.fetched, client.fetchErr
}
