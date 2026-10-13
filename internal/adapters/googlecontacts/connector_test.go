package googlecontacts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestConnectorSynchronizesGooglePeopleIncrementally(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.URL.Path != "/v1/people/me/connections" {
			t.Fatalf("request=%s %s headers=%#v", request.Method, request.URL, request.Header)
		}
		requests++
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("syncToken") == "sync-1" {
			_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-2","connections":[{"resourceName":"people/123","metadata":{"sources":[{"type":"CONTACT","etag":"etag-2"}]},"names":[{"displayName":"Alice Updated","givenName":"Alice","familyName":"Example"}],"emailAddresses":[{"value":"alice@example.com","type":"work"}]}]}`)
			return
		}
		_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-1","connections":[{"resourceName":"people/123","metadata":{"sources":[{"type":"CONTACT","etag":"etag-1"}]},"names":[{"displayName":"Alice Example","givenName":"Alice","familyName":"Example"}],"emailAddresses":[{"value":"alice@example.com","type":"work"}],"phoneNumbers":[{"value":"+15551234567","type":"mobile"}],"organizations":[{"name":"Example","title":"Director","current":true}],"birthdays":[{"date":{"year":1990,"month":8,"day":22}}],"biographies":[{"value":"Trusted contact"}]}]}`)
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1/", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Contacts: true}}
	first, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, contactsync.State{})
	if err != nil || len(first.AddressBooks) != 1 || !first.AddressBooks[0].Writable || first.AddressBooks[0].SyncToken != "sync-1" || len(first.Contacts) != 1 || first.Contacts[0].ETag != "etag-1" || first.Contacts[0].Birthday != "1990-08-22" || !strings.Contains(first.Contacts[0].RawVCard, "UID:people/123") {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, contactsync.State{AddressBooks: first.AddressBooks, Contacts: first.Contacts})
	if err != nil || requests != 2 || second.AddressBooks[0].SyncToken != "sync-2" || len(second.Contacts) != 1 || second.Contacts[0].Name != "Alice Updated" || second.Contacts[0].ETag != "etag-2" {
		t.Fatalf("second=%#v requests=%d err=%v", second, requests, err)
	}
}

func TestConnectorAppliesGooglePeopleMutationsWithSourceETag(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/people/me/connections":
			_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-1","connections":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/people:createContact":
			posts++
			var payload person
			if json.NewDecoder(request.Body).Decode(&payload) != nil || len(payload.ClientData) != 1 || payload.ClientData[0].Key != markerKey || len(payload.EmailAddresses) != 1 {
				t.Fatalf("create payload=%#v", payload)
			}
			_, _ = fmt.Fprint(response, `{"resourceName":"people/created"}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/v1/people/123:updateContact":
			var payload person
			if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.ResourceName != "people/123" || len(payload.Metadata.Sources) != 1 || payload.Metadata.Sources[0].ETag != "etag-7" || request.URL.Query().Get("updatePersonFields") == "" {
				t.Fatalf("update payload=%#v query=%#v", payload, request.URL.Query())
			}
			_, _ = fmt.Fprint(response, `{"resourceName":"people/123"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/people/123":
			_, _ = fmt.Fprint(response, `{"resourceName":"people/123","metadata":{"sources":[{"type":"CONTACT","etag":"etag-7"}]}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1/people/123:deleteContact":
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Contacts: true}}
	scope := accountsync.Scope{AccountID: account.ID}
	desired := contactsync.Contact{Name: "Alice", Emails: []contactsync.Value{{Value: "alice@example.com", Type: "work"}}}
	placeholder := contactsync.Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: server.URL + "/v1/people/pending", UID: "pending", Name: desired.Name, Emails: desired.Emails}
	create := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "google-create-1", Kind: contactsync.MutationCreate, ContactID: placeholder.ID, Desired: placeholder}, Scope: scope, State: contactsync.MutationApplying, Attempts: 1}
	if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, create, placeholder); err != nil || outcome != contactsync.OutcomeApplied || posts != 1 {
		t.Fatalf("create=%q posts=%d err=%v", outcome, posts, err)
	}
	current := contactsync.Contact{ID: "carddav_zyxwvutsrqponmlkjihgfedc", AccountID: account.ID, AddressBookID: placeholder.AddressBookID, Href: server.URL + "/v1/people/123", ETag: "etag-7", UID: "people/123", Name: "Old"}
	for _, kind := range []contactsync.MutationKind{contactsync.MutationUpdate, contactsync.MutationDelete} {
		mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "google-" + string(kind), Kind: kind, ContactID: current.ID, ExpectedETag: current.ETag, Desired: desired}, Scope: scope, State: contactsync.MutationApplying, Attempts: 1}
		if kind == contactsync.MutationDelete {
			mutation.Desired = contactsync.Contact{}
		}
		if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current); err != nil || outcome != contactsync.OutcomeApplied {
			t.Fatalf("%s=%q err=%v", kind, outcome, err)
		}
	}
}

func TestConnectorReconcilesRetriedGoogleCreateWithoutPosting(t *testing.T) {
	posts := 0
	marker := markerValue("google-create-1")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			posts++
		}
		_, _ = fmt.Fprintf(response, `{"nextSyncToken":"sync-1","connections":[{"resourceName":"people/created","clientData":[{"key":"%s","value":"%s"}]}]}`, markerKey, marker)
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail}
	current := contactsync.Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, Href: server.URL + "/v1/people/pending"}
	mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{IdempotencyKey: "google-create-1", Kind: contactsync.MutationCreate, ContactID: current.ID, Desired: contactsync.Contact{Name: "Alice"}}, Scope: accountsync.Scope{AccountID: account.ID}, State: contactsync.MutationApplying, Attempts: 2}
	outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current)
	if err != nil || outcome != contactsync.OutcomeApplied || posts != 0 {
		t.Fatalf("outcome=%q posts=%d err=%v", outcome, posts, err)
	}
}

func TestConnectorRejectsMalformedGoogleResourceIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-1","connections":[{"resourceName":"https://attacker.invalid/people/123","names":[{"displayName":"Alice"}]}]}`)
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Contacts: true}}
	if _, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, contactsync.State{}); err == nil {
		t.Fatal("malformed provider resource identity was accepted")
	}
}
