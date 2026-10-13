package microsoftcontacts

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

func TestConnectorSynchronizesMicrosoftRootAndFolderContacts(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/users/alice@example.com/contactFolders":
			_, _ = fmt.Fprint(response, `{"value":[{"id":"work","displayName":"Work"}]}`)
		case "/v1.0/users/alice@example.com/contacts":
			_, _ = fmt.Fprint(response, `{"value":[{"id":"root-1","@odata.etag":"W/\"1\"","displayName":"Alice Example","givenName":"Alice","surname":"Example","emailAddresses":[{"address":"alice@example.com","name":"Alice"}],"businessPhones":["+15551234567"],"businessAddress":{"street":"Suite 2\n1 Main St","city":"Toronto","state":"ON","postalCode":"M5V 1A1","countryOrRegion":"Canada"},"birthday":"1990-08-22T00:00:00Z","personalNotes":"Trusted"}]}`)
		case "/v1.0/users/alice@example.com/contactFolders/work/contacts/delta":
			_, _ = fmt.Fprintf(response, `{"@odata.deltaLink":%q,"value":[{"id":"work-1","@odata.etag":"W/\"2\"","displayName":"Bob Work","emailAddresses":[{"address":"bob@example.com"}]}]}`, server.URL+"/v1.0/users/alice@example.com/contactFolders/work/contacts/delta?$deltatoken=one")
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1.0/", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft, MailboxAddress: "Alice@Example.com", Capabilities: accounts.Capabilities{Contacts: true}}
	snapshot, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, contactsync.State{})
	if err != nil || len(snapshot.AddressBooks) != 2 || len(snapshot.Contacts) != 2 {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
	foundRoot, foundFolder := false, false
	for _, contact := range snapshot.Contacts {
		if contact.UID == "root-1" {
			foundRoot = contact.Birthday == "1990-08-22" && contact.ETag == `W/"1"` && strings.Contains(contact.RawVCard, "UID:root-1") && contact.Addresses[0].StreetAddress == "Suite 2, 1 Main St"
		}
		if contact.UID == "work-1" {
			foundFolder = contact.Name == "Bob Work"
		}
	}
	if !foundRoot || !foundFolder {
		t.Fatalf("contacts=%#v", snapshot.Contacts)
	}
}

func TestConnectorAppliesConditionalMicrosoftContactMutations(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1.0/users/alice@example.com/contacts":
			_, _ = fmt.Fprint(response, `{"value":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/users/alice@example.com/contacts":
			posts++
			var payload graphContact
			if json.NewDecoder(request.Body).Decode(&payload) != nil || payload.DisplayName != "Alice" || len(payload.Extensions) != 1 || payload.Extensions[0].ExtensionName != extensionName || payload.Extensions[0].Idempotency == "" {
				t.Fatalf("create payload=%#v", payload)
			}
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(response, `{"id":"created"}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/v1.0/users/alice@example.com/contacts/one":
			if request.Header.Get("If-Match") != `W/"7"` {
				t.Fatalf("If-Match=%q", request.Header.Get("If-Match"))
			}
			_, _ = fmt.Fprint(response, `{"id":"one"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1.0/users/alice@example.com/contacts/one":
			if request.Header.Get("If-Match") != `W/"7"` {
				t.Fatalf("If-Match=%q", request.Header.Get("If-Match"))
			}
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1.0/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft, MailboxAddress: "alice@example.com"}
	scope := accountsync.Scope{AccountID: account.ID}
	collection := server.URL + "/v1.0/users/alice@example.com/contacts"
	desired := contactsync.Contact{Name: "Alice", Emails: []contactsync.Value{{Value: "alice@example.com"}}}
	placeholder := contactsync.Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: collection + "/pending", UID: "pending", Name: desired.Name, Emails: desired.Emails}
	create := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "microsoft-create-1", Kind: contactsync.MutationCreate, ContactID: placeholder.ID, Desired: placeholder}, Scope: scope, State: contactsync.MutationApplying, Attempts: 1}
	if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, create, placeholder); err != nil || outcome != contactsync.OutcomeApplied || posts != 1 {
		t.Fatalf("create=%q posts=%d err=%v", outcome, posts, err)
	}
	current := contactsync.Contact{ID: "carddav_zyxwvutsrqponmlkjihgfedc", AccountID: account.ID, AddressBookID: placeholder.AddressBookID, Href: collection + "/one", ETag: `W/"7"`, UID: "one", Name: "Old"}
	for _, kind := range []contactsync.MutationKind{contactsync.MutationUpdate, contactsync.MutationDelete} {
		mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "microsoft-" + string(kind), Kind: kind, ContactID: current.ID, ExpectedETag: current.ETag, Desired: desired}, Scope: scope, State: contactsync.MutationApplying, Attempts: 1}
		if kind == contactsync.MutationDelete {
			mutation.Desired = contactsync.Contact{}
		}
		if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current); err != nil || outcome != contactsync.OutcomeApplied {
			t.Fatalf("%s=%q err=%v", kind, outcome, err)
		}
	}
}

func TestConnectorReconcilesRetriedMicrosoftCreate(t *testing.T) {
	posts := 0
	marker := markerValue("microsoft-create-1")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			posts++
		}
		_, _ = fmt.Fprintf(response, `{"value":[{"id":"created","extensions":[{"id":"%s","idempotencyKey":"%s"}]}]}`, extensionName, marker)
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1.0/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft}
	current := contactsync.Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, Href: server.URL + "/v1.0/users/alice/contacts/pending"}
	mutation := contactsync.Mutation{MutationRequest: contactsync.MutationRequest{IdempotencyKey: "microsoft-create-1", Kind: contactsync.MutationCreate, ContactID: current.ID}, Scope: accountsync.Scope{AccountID: account.ID}, State: contactsync.MutationApplying, Attempts: 2}
	outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current)
	if err != nil || outcome != contactsync.OutcomeApplied || posts != 0 {
		t.Fatalf("outcome=%q posts=%d err=%v", outcome, posts, err)
	}
}
