package bridgeapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/auth"
	"github.com/d31ma/caduceus/internal/calendarsync"
	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestAccountDispatcherDerivesOwnerFromAuthenticatedToken(t *testing.T) {
	service := &recordingService{listed: []accounts.ExternalAccount{{ID: "ext_AAAAAAAAAAAAAAAAAAAA"}}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.list", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	listed := result.([]accounts.ExternalAccount)
	if len(listed) != 1 || service.owner.TenantID != "tenant-1" || service.owner.PrincipalID != "principal-alice" {
		t.Fatalf("Dispatch() = %#v owner=%#v", listed, service.owner)
	}
}

func TestAccountDispatcherClaimsAndCompletesInternalProviderNotifications(t *testing.T) {
	queue := &recordingNotificationQueue{claim: accountsync.NewMailNotificationClaim{Token: "claim-token", Notifications: []accountsync.NewMailNotification{{ID: "pnot_abcdefghijklmnopqrstuvwx"}}}}
	dispatcher := NewAccountsDispatcher(nil, nil, nil, AccountExtensions{Notifications: queue})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.notification_claim", json.RawMessage(`{"holder":"tachyon-1","limit":25}`))
	if err != nil || result.(accountsync.NewMailNotificationClaim).Token != "claim-token" || queue.holder != "tachyon-1" || queue.limit != 25 {
		t.Fatalf("claim=%#v err=%v holder=%q limit=%d", result, err, queue.holder, queue.limit)
	}
	result, err = dispatcher.Dispatch(context.Background(), "accounts.notification_complete", json.RawMessage(`{"token":"claim-token","ids":["pnot_abcdefghijklmnopqrstuvwx"],"delivered":false,"code":"push_delivery_failed"}`))
	if err != nil || result.(map[string]any)["count"] != 1 || queue.token != "claim-token" || queue.delivered || queue.code != "push_delivery_failed" {
		t.Fatalf("complete=%#v err=%v queue=%#v", result, err, queue)
	}
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.notification_claim", json.RawMessage(`{"holder":"tachyon-1","limit":25,"token":"must-not-be-accepted"}`)); stableCode(err) != "EBADREQUEST" {
		t.Fatalf("privileged claim accepted an unknown field: %v", err)
	}
}

func TestAccountDispatcherReturnsRedactedOwnerScopedProviderContacts(t *testing.T) {
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, Capabilities: accounts.Capabilities{Contacts: true}, Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}}
	reader := &recordingContactReader{contacts: []contactsync.Contact{{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: "ext_AAAAAAAAAAAAAAAAAAAA", AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: "/a.vcf", ETag: `"7"`, Name: "Alice", RawVCard: "BEGIN:VCARD\r\nNOTE:private\r\nEND:VCARD\r\n", RemoteUpdatedAt: time.Now().UTC()}}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{listed: []accounts.ExternalAccount{account}}, nil, AccountExtensions{Contacts: reader})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.contacts", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatal(err)
	}
	contacts := result.(map[string]any)["contacts"].([]map[string]any)
	encoded, _ := json.Marshal(contacts)
	if len(contacts) != 1 || contacts[0]["source"] != "carddav" || contacts[0]["readOnly"] != false || reader.owner.TenantID != "tenant-1" || containsText(string(encoded), "BEGIN:VCARD") || containsText(string(encoded), "private") {
		t.Fatalf("contacts=%s owner=%#v", encoded, reader.owner)
	}
}

func TestAccountDispatcherReturnsOnlyOwnedEnabledCalendarResources(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		Capabilities: accounts.Capabilities{Calendar: true},
		Connection:   accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://calendar.example/"}},
	}
	reader := &recordingCalendarReader{calendars: []calendarsync.OwnedCalendar{{AccountID: account.ID, Calendar: calendarsync.Calendar{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://calendar.example/work/", Name: "Work calendar", Color: "#112233"}}}, resources: []calendarsync.OwnedResource{
		{OwnerAddress: owner.Address, CalendarName: "Work calendar", CalendarColor: "#112233", Resource: calendarsync.Resource{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://calendar.example/work/one.ics", ETag: `"7"`, RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:one\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: time.Now().UTC()}},
		{OwnerAddress: owner.Address, CalendarName: "Hidden", Resource: calendarsync.Resource{ID: "caldav_zyxwvutsrqponmlkjihgfedc", AccountID: "ext_BBBBBBBBBBBBBBBBBBBB", CalendarID: "calendar_zyxwvutsrqponmlkjihgfedc", Href: "https://calendar.example/hidden.ics", RawICalendar: "private"}},
	}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{listed: []accounts.ExternalAccount{account}}, nil, AccountExtensions{Calendars: reader})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_resources", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatal(err)
	}
	resources := result.(map[string]any)["resources"].([]map[string]any)
	calendars := result.(map[string]any)["calendars"].([]map[string]any)
	if len(resources) != 1 || len(calendars) != 1 || calendars[0]["calendarId"] != "calendar_abcdefghijklmnopqrstuvwx" || calendars[0]["writable"] != true || resources[0]["accountId"] != account.ID || resources[0]["accountLabel"] != "Work" || resources[0]["calendarName"] != "Work calendar" || resources[0]["rawICalendar"] == "" || reader.owner != owner {
		t.Fatalf("resources=%#v owner=%#v", resources, reader.owner)
	}
}

func TestAccountDispatcherProjectsNativeProviderCalendar(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderGmail,
		MailboxAddress: "alice@gmail.com", Label: "Google", Capabilities: accounts.Capabilities{Calendar: true},
	}
	calendar := calendarsync.Calendar{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://www.googleapis.com/calendar/v3/calendars/primary/", Name: "Primary", Writable: true}
	resource := calendarsync.Resource{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: calendar.StableID, Href: calendar.URL + "events/event-1", ETag: `"7"`, RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:one\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: time.Now().UTC()}
	reader := &recordingCalendarReader{
		calendars: []calendarsync.OwnedCalendar{{AccountID: account.ID, Calendar: calendar}},
		resources: []calendarsync.OwnedResource{{OwnerAddress: owner.Address, CalendarName: calendar.Name, Resource: resource}},
	}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{listed: []accounts.ExternalAccount{account}}, nil, AccountExtensions{Calendars: reader})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_resources", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatal(err)
	}
	calendars := result.(map[string]any)["calendars"].([]map[string]any)
	resources := result.(map[string]any)["resources"].([]map[string]any)
	if len(calendars) != 1 || calendars[0]["writable"] != true || len(resources) != 1 || resources[0]["source"] != "gmail-calendar" {
		t.Fatalf("calendars=%#v resources=%#v", calendars, resources)
	}
}

func TestAccountDispatcherQueuesOwnerScopedConditionalCalendarMutation(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Capabilities: accounts.Capabilities{Calendar: true}, Connection: accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://calendar.example/"}}}
	raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:Updated\r\nDTSTART:20260823T120000Z\r\nDTEND:20260823T130000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	access := &recordingCalendarAccess{resource: calendarsync.Resource{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://calendar.example/work/one.ics", ETag: `"7"`, RawICalendar: raw}}
	reader := &recordingCalendarReader{calendars: []calendarsync.OwnedCalendar{{AccountID: account.ID, Calendar: calendarsync.Calendar{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://calendar.example/work/", Name: "Work"}}}}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Schedule: schedule, Calendars: reader, CalendarMutations: access})
	payload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "idempotencyKey": "edit-1", "kind": "update", "resourceId": access.resource.ID, "expectedEtag": access.resource.ETag, "rawICalendar": raw})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_mutate", payload)
	if err != nil {
		t.Fatal(err)
	}
	mutation := result.(calendarsync.Mutation)
	if mutation.State != calendarsync.MutationQueued || mutation.ResourceID != access.resource.ID || mutation.Desired.RawICalendar != raw || access.scope.Owner != owner || schedule.retries != 1 {
		t.Fatalf("mutation=%#v scope=%#v retries=%d", mutation, access.scope, schedule.retries)
	}
	statusPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "mutationId": mutation.ID})
	status, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_mutation", statusPayload)
	if err != nil || status.(calendarsync.Mutation).ID != mutation.ID {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	createPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "idempotencyKey": "create-1", "kind": "create", "calendarId": "calendar_abcdefghijklmnopqrstuvwx", "rawICalendar": raw})
	created, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_mutate", createPayload)
	if err != nil {
		t.Fatal(err)
	}
	createMutation := created.(calendarsync.Mutation)
	if createMutation.Kind != calendarsync.MutationCreate || createMutation.Desired.AccountID != account.ID || createMutation.Desired.CalendarID != "calendar_abcdefghijklmnopqrstuvwx" || createMutation.Desired.Href == "" || createMutation.Desired.ID != createMutation.ResourceID {
		t.Fatalf("create mutation=%#v", createMutation)
	}
}

func TestAccountDispatcherRejectsMutationForReadOnlyNativeCalendar(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderMicrosoft, Capabilities: accounts.Capabilities{Calendar: true}}
	raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:Updated\r\nDTSTART:20260823T120000Z\r\nDTEND:20260823T130000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	access := &recordingCalendarAccess{resource: calendarsync.Resource{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://graph.microsoft.com/v1.0/users/alice/calendars/work/events/one", ETag: `"7"`, RawICalendar: raw}}
	reader := &recordingCalendarReader{calendars: []calendarsync.OwnedCalendar{{AccountID: account.ID, Calendar: calendarsync.Calendar{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://graph.microsoft.com/v1.0/users/alice/calendars/work/", Name: "Shared", Writable: false}}}}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Schedule: schedule, Calendars: reader, CalendarMutations: access})
	payload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "idempotencyKey": "edit-1", "kind": "update", "resourceId": access.resource.ID, "expectedEtag": access.resource.ETag, "rawICalendar": raw})
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.calendar_mutate", payload); stableCode(err) != "ENOTFOUND" {
		t.Fatalf("read-only mutation error = %v", err)
	}
	if access.mutation.ID != "" || schedule.retries != 0 {
		t.Fatalf("mutation=%#v retries=%d", access.mutation, schedule.retries)
	}
}

func TestAccountDispatcherQueuesOwnerScopedConditionalContactMutation(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Capabilities: accounts.Capabilities{Contacts: true}, Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}}
	book := contactsync.AddressBook{StableID: "book_abcdefghijklmnopqrstuvwx", URL: "https://dav.example/books/default/", Name: "Contacts"}
	access := &recordingContactAccess{contact: contactsync.Contact{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, AddressBookID: book.StableID, ETag: `"7"`, Name: "Alice"}}
	reader := &recordingContactReader{books: []contactsync.OwnedAddressBook{{AccountID: account.ID, AddressBook: book}}}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Schedule: schedule, Contacts: reader, ContactMutations: access})
	payload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","idempotencyKey":"edit-1","kind":"update","contactId":"carddav_abcdefghijklmnopqrstuvwx","expectedEtag":"\"7\"","contact":{"name":"Alice Updated","givenName":"Alice","familyName":"Updated","company":"","title":"","emails":[{"value":"alice@example.com","type":"work"}],"phones":[],"addresses":[],"urls":[],"nicknames":[],"birthday":"","notes":"","groupIds":[]}}`)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.contact_mutate", payload)
	if err != nil {
		t.Fatal(err)
	}
	mutation := result.(contactsync.Mutation)
	if mutation.State != contactsync.MutationQueued || mutation.ContactID != access.contact.ID || mutation.Desired.Name != "Alice Updated" || access.scope.Owner != owner || schedule.retries != 1 {
		t.Fatalf("mutation=%#v scope=%#v retries=%d", mutation, access.scope, schedule.retries)
	}
	statusPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "mutationId": mutation.ID})
	status, err := dispatcher.Dispatch(context.Background(), "accounts.contact_mutation", statusPayload)
	if err != nil || status.(contactsync.Mutation).ID != mutation.ID {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}

func TestAccountDispatcherListsRedactedBooksAndQueuesRemoteContactCreation(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, MailboxAddress: "alice@remote.example", Label: "Work", Capabilities: accounts.Capabilities{Contacts: true}, Connection: accounts.ConnectionSettings{CardDAV: &accounts.CardDAVEndpoint{URL: "https://dav.example/"}}}
	reader := &recordingContactReader{books: []contactsync.OwnedAddressBook{{AccountID: account.ID, AddressBook: contactsync.AddressBook{StableID: "book_abcdefghijklmnopqrstuvwx", URL: "https://dav.example/books/alice/default/", Name: "Contacts", SyncToken: "private-token"}}}}
	access := &recordingContactAccess{}
	schedule := &recordingSchedule{}
	service := &recordingService{got: account, listed: []accounts.ExternalAccount{account}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil, AccountExtensions{Contacts: reader, Schedule: schedule, ContactMutations: access})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.contact_books", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	books := result.(map[string]any)["addressBooks"].([]map[string]any)
	if len(books) != 1 || books[0]["accountId"] != account.ID || books[0]["addressBookId"] != "book_abcdefghijklmnopqrstuvwx" || books[0]["accountLabel"] != "Work" || containsText(string(encoded), "https://dav.example") || containsText(string(encoded), "private-token") {
		t.Fatalf("books=%s", encoded)
	}

	payload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","idempotencyKey":"create-1","kind":"create","addressBookId":"book_abcdefghijklmnopqrstuvwx","contact":{"name":"Created Remotely","emails":[{"value":"created@example.net","type":"work"}]}}`)
	created, err := dispatcher.Dispatch(context.Background(), "accounts.contact_mutate", payload)
	if err != nil {
		t.Fatal(err)
	}
	mutation := created.(contactsync.Mutation)
	if mutation.Kind != contactsync.MutationCreate || mutation.ExpectedETag != "" || mutation.ContactID == "" || mutation.Desired.ID != mutation.ContactID || mutation.Desired.AccountID != account.ID || mutation.Desired.AddressBookID != "book_abcdefghijklmnopqrstuvwx" || mutation.Desired.Href == "" || mutation.Desired.UID == "" || access.scope.Owner != owner || schedule.retries != 1 {
		t.Fatalf("mutation=%#v scope=%#v retries=%d", mutation, access.scope, schedule.retries)
	}
}

func TestAccountDispatcherProjectsOnlyEnabledNativeProviderContacts(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	google := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderGmail, MailboxAddress: "alice@gmail.com", Capabilities: accounts.Capabilities{Contacts: true}}
	disabled := accounts.ExternalAccount{ID: "ext_BBBBBBBBBBBBBBBBBBBB", Owner: owner, Provider: accounts.ProviderMicrosoft, MailboxAddress: "alice@example.net"}
	reader := &recordingContactReader{contacts: []contactsync.Contact{
		{ID: "carddav_abcdefghijklmnopqrstuvwx", AccountID: google.ID, AddressBookID: "book_abcdefghijklmnopqrstuvwx", Href: "https://people.googleapis.com/v1/people/one", Name: "Google Contact", RemoteUpdatedAt: time.Now().UTC()},
		{ID: "carddav_zyxwvutsrqponmlkjihgfedc", AccountID: disabled.ID, AddressBookID: "book_zyxwvutsrqponmlkjihgfedc", Href: "https://graph.microsoft.com/v1.0/users/alice/contacts/one", Name: "Stale Contact", RemoteUpdatedAt: time.Now().UTC()},
	}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{listed: []accounts.ExternalAccount{google, disabled}}, nil, AccountExtensions{Contacts: reader})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.contacts", json.RawMessage(`{"token":"valid"}`))
	if err != nil {
		t.Fatal(err)
	}
	projected := result.(map[string]any)["contacts"].([]map[string]any)
	if len(projected) != 1 || projected[0]["accountId"] != google.ID || projected[0]["name"] != "Google Contact" {
		t.Fatalf("contacts=%#v", projected)
	}
}

func TestAccountDispatcherRejectsMutationForReadOnlyNativeAddressBook(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderMicrosoft, Capabilities: accounts.Capabilities{Contacts: true}}
	book := contactsync.AddressBook{StableID: "book_abcdefghijklmnopqrstuvwx", URL: "https://graph.microsoft.com/v1.0/users/alice/contactFolders/shared/contacts", Name: "Shared", Writable: false}
	reader := &recordingContactReader{books: []contactsync.OwnedAddressBook{{AccountID: account.ID, AddressBook: book}}}
	access := &recordingContactAccess{}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Contacts: reader, ContactMutations: access, Schedule: schedule})
	payload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","idempotencyKey":"create-1","kind":"create","addressBookId":"book_abcdefghijklmnopqrstuvwx","contact":{"name":"Alice","emails":[{"value":"alice@example.net"}]}}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.contact_mutate", payload); stableCode(err) != "ENOTFOUND" {
		t.Fatalf("read-only mutation error=%v", err)
	}
	if access.mutation.ID != "" || schedule.retries != 0 {
		t.Fatalf("mutation=%#v retries=%d", access.mutation, schedule.retries)
	}
}

func TestAccountDispatcherExposesCatalogAndDiscoveryOnlyAfterAuthentication(t *testing.T) {
	discovery := &fakeDiscovery{result: accounts.DiscoveryResult{ChoiceID: "imap", Source: accounts.DiscoveryFallback}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{}, discovery)
	providers, err := dispatcher.Dispatch(context.Background(), "accounts.providers", json.RawMessage(`{"token":"valid"}`))
	if err != nil || len(providers.([]accounts.ProviderChoice)) != 10 {
		t.Fatalf("providers = %#v, %v", providers, err)
	}
	result, err := dispatcher.Dispatch(context.Background(), "accounts.discover", json.RawMessage(`{"token":"valid","address":"alice@example.com"}`))
	if err != nil || result.(accounts.DiscoveryResult).ChoiceID != "imap" || discovery.address != "alice@example.com" {
		t.Fatalf("discover = %#v, %v", result, err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.providers", json.RawMessage(`{"token":"expired"}`)); stableCode(err) != "EUNAUTHORIZED" {
		t.Fatalf("unauthenticated providers error = %v", err)
	}
}

func TestAccountDispatcherCompletesOwnerBoundMicrosoftOAuth(t *testing.T) {
	oauth := &recordingOAuthConnection{authorization: accounts.OAuthAuthorization{AuthorizationURL: "https://login.microsoftonline.com/authorize", State: "opaque-state"}, account: accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{}, nil, AccountExtensions{OAuth: oauth})
	providers, err := dispatcher.Dispatch(context.Background(), "accounts.providers", json.RawMessage(`{"token":"valid"}`))
	if err != nil || !providers.([]accounts.ProviderChoice)[2].Configured {
		t.Fatalf("Microsoft provider availability = %#v, %v", providers, err)
	}
	begin := json.RawMessage(`{"token":"valid","provider":"microsoft","mailboxAddress":"support@example.com","authIdentity":"delegate@example.com","sharedMailboxAddress":"support@example.com","label":"Support","color":"#336699","order":2}`)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.microsoft_oauth_begin", begin)
	if err != nil || result.(accounts.OAuthAuthorization).State != "opaque-state" || oauth.owner.PrincipalID != "principal-alice" || oauth.request.SharedMailboxAddress != "support@example.com" {
		t.Fatalf("begin=%#v err=%v owner=%#v request=%#v", result, err, oauth.owner, oauth.request)
	}
	result, err = dispatcher.Dispatch(context.Background(), "accounts.microsoft_oauth_complete", json.RawMessage(`{"token":"valid","state":"opaque-state","code":"authorization-code"}`))
	if err != nil || result.(accounts.ExternalAccount).ID == "" || oauth.state != "opaque-state" || oauth.code != "authorization-code" {
		t.Fatalf("complete=%#v err=%v state=%q code=%q", result, err, oauth.state, oauth.code)
	}
}

func TestAccountDispatcherCompletesOwnerBoundGoogleOAuth(t *testing.T) {
	oauth := &recordingOAuthConnection{authorization: accounts.OAuthAuthorization{AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth", State: "opaque-state"}, account: accounts.ExternalAccount{ID: "ext_GGGGGGGGGGGGGGGGGGGG", Provider: accounts.ProviderGmail}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{}, nil, AccountExtensions{GoogleOAuth: oauth})
	providers, err := dispatcher.Dispatch(context.Background(), "accounts.providers", json.RawMessage(`{"token":"valid"}`))
	if err != nil || !providers.([]accounts.ProviderChoice)[0].Configured || !providers.([]accounts.ProviderChoice)[1].Configured || providers.([]accounts.ProviderChoice)[2].Configured {
		t.Fatalf("Google provider availability = %#v, %v", providers, err)
	}
	begin := json.RawMessage(`{"token":"valid","provider":"gmail","mailboxAddress":"alice@gmail.com","authIdentity":"alice@gmail.com","label":"Gmail","color":"#4285F4","order":1}`)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.google_oauth_begin", begin)
	if err != nil || result.(accounts.OAuthAuthorization).State != "opaque-state" || oauth.request.Provider != accounts.ProviderGmail {
		t.Fatalf("begin=%#v err=%v request=%#v", result, err, oauth.request)
	}
	result, err = dispatcher.Dispatch(context.Background(), "accounts.google_oauth_complete", json.RawMessage(`{"token":"valid","state":"opaque-state","code":"authorization-code"}`))
	if err != nil || result.(accounts.ExternalAccount).Provider != accounts.ProviderGmail {
		t.Fatalf("complete=%#v err=%v", result, err)
	}
}

func TestAccountDispatcherStrictlyDecodesCreationAndNeverReturnsCredentials(t *testing.T) {
	service := &recordingService{created: accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil)
	payload := json.RawMessage(`{
      "token":"valid","provider":"imap","mailboxAddress":"alice@remote.example","label":"Work","color":"#336699",
      "credentials":{"username":"alice@remote.example","password":"top-secret"},
      "imap":{"host":"imap.remote.example","port":993,"security":"tls","allowInsecureTLS":false},
      "smtp":{"host":"smtp.remote.example","port":587,"security":"starttls","allowInsecureTLS":false}
    }`)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.create", payload)
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if service.create.Credentials.Password != "top-secret" {
		t.Fatal("credential did not reach account service")
	}
	encoded, _ := json.Marshal(result)
	if containsText(string(encoded), "top-secret") {
		t.Fatalf("create response leaked credential: %s", encoded)
	}
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.create", append(payload[:len(payload)-1], []byte(`,"admin":true}`)...)); stableCode(err) != "EBADREQUEST" {
		t.Fatalf("unknown-field create error = %v", err)
	}
}

func TestAccountDispatcherUpdatesPresentationMetadataForAuthenticatedOwner(t *testing.T) {
	service := &recordingService{created: accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Label: "Personal", Color: "#AABBCC", Order: 0, Revision: 4}}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.update_presentation", json.RawMessage(`{
      "token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":3,"label":"Personal","color":"#AABBCC","order":0
    }`))
	if err != nil || result.(accounts.ExternalAccount).Revision != 4 || service.update.Label == nil || *service.update.Label != "Personal" || service.update.Order == nil || *service.update.Order != 0 {
		t.Fatalf("update result=%#v update=%#v err=%v", result, service.update, err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.update_presentation", json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":3,"admin":true}`)); stableCode(err) != "EBADREQUEST" {
		t.Fatalf("unknown-field update error = %v", err)
	}
}

func TestAccountDispatcherReturnsSafeActionableProviderValidationCodes(t *testing.T) {
	tests := []struct {
		failure accounts.ValidationFailure
		code    string
	}{
		{accounts.ValidationFailure{Service: accounts.ValidationIMAP, Code: accounts.ValidationAuthentication}, "EAUTHREQUIRED"},
		{accounts.ValidationFailure{Service: accounts.ValidationSMTP, Code: accounts.ValidationConnection}, "EUNAVAILABLE"},
		{accounts.ValidationFailure{Service: accounts.ValidationIMAP, Code: accounts.ValidationMailbox}, "EBADREQUEST"},
		{accounts.ValidationFailure{Service: accounts.ValidationSMTP, Code: accounts.ValidationProtocol}, "EUNAVAILABLE"},
	}
	for _, test := range tests {
		err := classify(test.failure)
		if stableCode(err) != test.code {
			t.Fatalf("classify(%v) = %v, want %s", test.failure, err, test.code)
		}
		if containsText(err.Error(), "%!") || containsText(err.Error(), "password") {
			t.Fatalf("classify(%v) returned unsafe detail: %v", test.failure, err)
		}
	}
}

func TestAccountDispatcherAuthenticatesAndJournalsExternalSubmission(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderIMAP, MailboxAddress: "remote@remote.example",
		Revision: 7, State: accounts.StateIdle, Capabilities: accounts.Capabilities{Send: true}}
	service := &recordingService{got: account}
	submissions := &recordingSubmissions{}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil, AccountExtensions{Schedule: schedule, Submissions: submissions, SubmissionReader: submissions})
	raw := []byte("From: remote@remote.example\r\nTo: recipient@example.net\r\nMessage-ID: <one@remote.example>\r\n\r\nhello\r\n")
	payload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "idempotencyKey": "compose-1", "envelopeFrom": account.MailboxAddress,
		"recipients": []string{"recipient@example.net"}, "rawRFC822Base64": base64.StdEncoding.EncodeToString(raw), "notBefore": nil})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.submit", payload)
	if err != nil {
		t.Fatal(err)
	}
	queued := result.(accountsend.Submission)
	if queued.State != accountsend.StateQueued || submissions.request.ID == "" || string(submissions.request.RawRFC822) != string(raw) || schedule.retries != 1 {
		t.Fatalf("queued=%#v request=%#v retries=%d", queued, submissions.request, schedule.retries)
	}
	encoded, _ := json.Marshal(result)
	if containsText(string(encoded), "hello") {
		t.Fatalf("submission response leaked RFC 822 source: %s", encoded)
	}
	updatePayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": queued.ID, "revision": queued.Revision,
		"idempotencyKey": "compose-1", "envelopeFrom": account.MailboxAddress, "recipients": []string{"updated@example.net"},
		"rawRFC822Base64": base64.StdEncoding.EncodeToString([]byte("updated")), "notBefore": nil})
	result, err = dispatcher.Dispatch(context.Background(), "accounts.update_submission", updatePayload)
	if err != nil || result.(accountsend.Submission).Revision != queued.Revision+1 || submissions.replaceRevision != queued.Revision || string(submissions.replaceRequest.RawRFC822) != "updated" || schedule.retries != 2 {
		t.Fatalf("update = %#v, %v request=%#v revision=%d retries=%d", result, err, submissions.replaceRequest, submissions.replaceRevision, schedule.retries)
	}
	cancelPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": queued.ID})
	result, err = dispatcher.Dispatch(context.Background(), "accounts.cancel_submission", cancelPayload)
	if err != nil || result.(accountsend.Submission).State != accountsend.StateCancelled || submissions.cancelID != queued.ID {
		t.Fatalf("cancel = %#v, %v id=%s", result, err, submissions.cancelID)
	}
	status, err := dispatcher.Dispatch(context.Background(), "accounts.submission", cancelPayload)
	if err != nil || status.(accountsend.Submission).ID != queued.ID || submissions.readID != queued.ID {
		t.Fatalf("status = %#v, %v id=%s", status, err, submissions.readID)
	}
}

func TestAccountDispatcherStagesTrackedSubmissionBatchBeforeScheduling(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderIMAP,
		MailboxAddress: "remote@remote.example", Revision: 7, State: accounts.StateIdle, Capabilities: accounts.Capabilities{Send: true}}
	submissions := &recordingSubmissions{}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil,
		AccountExtensions{Schedule: schedule, Submissions: submissions, SubmissionReader: submissions})
	beginPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "idempotencyKey": "tracked-group-1",
		"envelopeFrom": account.MailboxAddress, "recipients": []string{"one@example.net", "two@example.net"}, "notBefore": nil})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.submit_batch_begin", beginPayload)
	if err != nil || result.(accountsend.Submission).State != accountsend.StateBuilding || schedule.retries != 0 {
		t.Fatalf("begin = %#v, %v retries=%d", result, err, schedule.retries)
	}
	batch := result.(accountsend.Submission)
	for index, recipient := range []string{"one@example.net", "two@example.net"} {
		raw := []byte("delivery-" + recipient)
		appendPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": batch.ID,
			"revision": batch.Revision, "index": index, "recipient": recipient, "rawRFC822Base64": base64.StdEncoding.EncodeToString(raw)})
		result, err = dispatcher.Dispatch(context.Background(), "accounts.submit_batch_append", appendPayload)
		if err != nil || string(submissions.appendRequests[index].RawRFC822) != string(raw) {
			t.Fatalf("append %d = %#v, %v request=%#v", index, result, err, submissions.appendRequests)
		}
		batch = result.(accountsend.Submission)
	}
	commitPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": batch.ID, "revision": batch.Revision})
	result, err = dispatcher.Dispatch(context.Background(), "accounts.submit_batch_commit", commitPayload)
	if err != nil || result.(accountsend.Submission).State != accountsend.StateQueued || submissions.commitRevision != batch.Revision || schedule.retries != 1 {
		t.Fatalf("commit = %#v, %v revision=%d retries=%d", result, err, submissions.commitRevision, schedule.retries)
	}
}

func TestAccountDispatcherStagesBatchReplacementBeforeScheduling(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderIMAP,
		MailboxAddress: "remote@remote.example", Revision: 7, State: accounts.StateIdle, Capabilities: accounts.Capabilities{Send: true}}
	submissions := &recordingSubmissions{}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil,
		AccountExtensions{Schedule: schedule, Submissions: submissions, SubmissionReader: submissions})
	beginPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": "sub_batchabcdefghijklmnop",
		"revision": 4, "idempotencyKey": "tracked-group-1", "envelopeFrom": account.MailboxAddress,
		"recipients": []string{"new@example.net"}, "notBefore": nil})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.submit_batch_update_begin", beginPayload)
	if err != nil || result.(accountsend.Submission).State != accountsend.StateEditing || !submissions.replaceBatch || schedule.retries != 0 {
		t.Fatalf("begin replacement = %#v, %v replacement=%t retries=%d", result, err, submissions.replaceBatch, schedule.retries)
	}
	editing := result.(accountsend.Submission)
	appendPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": editing.ID,
		"revision": editing.Revision, "index": 0, "recipient": "new@example.net", "rawRFC822Base64": base64.StdEncoding.EncodeToString([]byte("new"))})
	result, err = dispatcher.Dispatch(context.Background(), "accounts.submit_batch_update_append", appendPayload)
	if err != nil || string(submissions.appendRequests[0].RawRFC822) != "new" || schedule.retries != 0 {
		t.Fatalf("append replacement = %#v, %v requests=%#v retries=%d", result, err, submissions.appendRequests, schedule.retries)
	}
	editing = result.(accountsend.Submission)
	commitPayload, _ := json.Marshal(map[string]any{"token": "valid", "id": account.ID, "submissionId": editing.ID, "revision": editing.Revision})
	result, err = dispatcher.Dispatch(context.Background(), "accounts.submit_batch_update_commit", commitPayload)
	if err != nil || result.(accountsend.Submission).State != accountsend.StateQueued || schedule.retries != 1 {
		t.Fatalf("commit replacement = %#v, %v retries=%d", result, err, schedule.retries)
	}
}

func TestAccountDispatcherSchedulesDeletingAccountImmediately(t *testing.T) {
	service := &recordingService{}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, service, nil, AccountExtensions{Schedule: schedule})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.delete", json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":7}`))
	if err != nil || !result.(map[string]bool)["deleting"] || schedule.retries != 1 {
		t.Fatalf("delete = %#v, %v retries=%d", result, err, schedule.retries)
	}
}

func TestAccountDispatcherReplaysAcceptedMutationBeforeRevisionConflict(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Revision: 9}
	existing := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{
		ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "mail-action-1", Kind: accountsync.MutationSetRead,
		MessageStableIDs: []string{"INBOX:7:9"}, Value: true,
	}, Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, State: accountsync.MutationApplied}
	mutations := &recordingMutations{existing: existing}
	schedule := &recordingSchedule{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Schedule: schedule, Mutations: mutations})

	payload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":7,"idempotencyKey":"mail-action-1","kind":"set_read","messageStableIds":["INBOX:7:9"],"value":true}`)
	result, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", payload)
	if err != nil || result.(accountsync.Mutation).ID != existing.ID || mutations.enqueues != 0 || schedule.retries != 0 {
		t.Fatalf("replay = %#v, %v enqueues=%d retries=%d", result, err, mutations.enqueues, schedule.retries)
	}

	changed := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":7,"idempotencyKey":"mail-action-1","kind":"set_read","messageStableIds":["INBOX:7:9"],"value":false}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", changed); stableCode(err) != "ECONFLICT" {
		t.Fatalf("changed replay error = %v", err)
	}
	unknown := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":7,"idempotencyKey":"mail-action-2","kind":"set_read","messageStableIds":["INBOX:7:9"],"value":true}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", unknown); stableCode(err) != "ECONFLICT" {
		t.Fatalf("unknown stale mutation error = %v", err)
	}
	statusPayload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","mutationId":"mut_abcdefghijklmnopqrst"}`)
	status, err := dispatcher.Dispatch(context.Background(), "accounts.mutation", statusPayload)
	if err != nil || status.(accountsync.Mutation).ID != existing.ID {
		t.Fatalf("mutation status = %#v, %v", status, err)
	}
	foreignStatus := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","mutationId":"mut_zzzzzzzzzzzzzzzzzzzz"}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutation", foreignStatus); stableCode(err) != "ENOTFOUND" {
		t.Fatalf("foreign mutation status error = %v", err)
	}
}

func TestAccountDispatcherResolvesOwnerScopedFolderLifecyclePaths(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Revision: 9, Capabilities: accounts.Capabilities{MailWrite: true}}
	index := &recordingIndex{folders: []accountsync.Folder{
		{StableID: "INBOX", Name: "INBOX", Role: "inbox", Delimiter: "/"},
		{StableID: "Projects", Name: "Projects", Delimiter: "/"},
		{StableID: "Projects/Old", Name: "Projects/Old", Delimiter: "/"},
	}}
	mutations := &recordingMutations{}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{Index: index, Schedule: &recordingSchedule{}, Mutations: mutations})

	createPayload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":9,"idempotencyKey":"folder-create-1","kind":"folder_create","messageStableIds":[],"parentStableId":"Projects","folderName":"日本語"}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", createPayload); err != nil {
		t.Fatal(err)
	}
	if mutations.last.NewFolderPath != "Projects/&ZeVnLIqe-" || mutations.last.ParentStableID != "Projects" {
		t.Fatalf("create mutation = %#v", mutations.last)
	}

	renamePayload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":9,"idempotencyKey":"folder-rename-1","kind":"folder_rename","messageStableIds":[],"folderStableId":"Projects/Old","folderName":"New"}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", renamePayload); err != nil {
		t.Fatal(err)
	}
	if mutations.last.FolderPath != "Projects/Old" || mutations.last.NewFolderPath != "Projects/New" {
		t.Fatalf("rename mutation = %#v", mutations.last)
	}

	protectedPayload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","revision":9,"idempotencyKey":"folder-delete-1","kind":"folder_delete","messageStableIds":[],"folderStableId":"INBOX"}`)
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.mutate", protectedPayload); stableCode(err) != "EBADREQUEST" {
		t.Fatalf("protected delete error = %v", err)
	}
}

func TestAccountDispatcherExposesOwnerScopedUnifiedMessagesAndRawSource(t *testing.T) {
	owner := accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, MailboxAddress: "remote@example.net"}
	index := &recordingUnifiedIndex{messages: []accountsync.UnifiedMessage{{Message: accountsync.Message{StableID: "INBOX:7:9", Subject: "Provider"}, AccountID: account.ID, MailboxAddress: account.MailboxAddress}}, raw: []byte("Subject: Provider\r\n\r\nbody")}
	dispatcher := NewAccountsDispatcher(fakeAuthenticator{}, &recordingService{got: account}, nil, AccountExtensions{UnifiedIndex: index})
	result, err := dispatcher.Dispatch(context.Background(), "accounts.unified_messages", json.RawMessage(`{"token":"valid","folderRole":"inbox","folderStableId":"INBOX","query":"provider","receivedAtFrom":"2026-08-22T12:00:00Z","oldestFirst":true,"limit":25,"offset":0}`))
	if err != nil || len(result.(map[string]any)["messages"].([]accountsync.UnifiedMessage)) != 1 || index.owner != owner || index.filter.FolderRole != "inbox" || index.filter.FolderStableID != "INBOX" || index.filter.ReceivedAtFrom == nil || !index.filter.OldestFirst {
		t.Fatalf("unified = %#v, %v owner=%#v filter=%#v", result, err, index.owner, index.filter)
	}
	if _, err := dispatcher.Dispatch(context.Background(), "accounts.unified_messages", json.RawMessage(`{"token":"valid","receivedAtFrom":"not-a-date","limit":25}`)); stableCode(err) != "EBADREQUEST" {
		t.Fatalf("invalid unified timestamp error = %v", err)
	}
	sourcePayload := json.RawMessage(`{"token":"valid","id":"ext_AAAAAAAAAAAAAAAAAAAA","stableId":"INBOX:7:9"}`)
	result, err = dispatcher.Dispatch(context.Background(), "accounts.message_source", sourcePayload)
	if err != nil || result.(map[string]any)["rawRFC822Base64"] != base64.StdEncoding.EncodeToString(index.raw) || index.scope.Owner != owner || index.stableID != "INBOX:7:9" {
		t.Fatalf("source = %#v, %v scope=%#v stable=%s", result, err, index.scope, index.stableID)
	}
	metadata, err := dispatcher.Dispatch(context.Background(), "accounts.message", sourcePayload)
	if err != nil || metadata.(map[string]any)["message"].(accountsync.Message).StableID != "INBOX:7:9" || index.scope.Owner != owner {
		t.Fatalf("message metadata = %#v, %v scope=%#v", metadata, err, index.scope)
	}
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) Authenticate(_ context.Context, token string) (auth.Principal, error) {
	if token != "valid" {
		return auth.Principal{}, errors.New("expired token detail")
	}
	return auth.Principal{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}, nil
}

type recordingService struct {
	owner   accounts.Owner
	listed  []accounts.ExternalAccount
	created accounts.ExternalAccount
	create  accounts.CreateRequest
	update  accounts.PresentationUpdate
	got     accounts.ExternalAccount
}

func (service *recordingService) Create(_ context.Context, owner accounts.Owner, request accounts.CreateRequest) (accounts.ExternalAccount, error) {
	service.owner, service.create = owner, request
	return service.created, nil
}

func (service *recordingService) Validate(_ context.Context, owner accounts.Owner, request accounts.CreateRequest) (accounts.Capabilities, error) {
	service.owner, service.create = owner, request
	return accounts.Capabilities{MailRead: true}, nil
}

func (service *recordingService) Get(_ context.Context, owner accounts.Owner, _ accounts.AccountID) (accounts.ExternalAccount, error) {
	service.owner = owner
	return service.got, nil
}

type recordingSubmissions struct {
	request         accountsend.Request
	replaceRequest  accountsend.Request
	replaceRevision uint64
	cancelID        string
	readID          string
	batchRequest    accountsend.BatchRequest
	appendRequests  []accountsend.AppendRequest
	commitRevision  uint64
	replaceBatch    bool
}

func (submissions *recordingSubmissions) BeginBatch(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.BatchRequest) (accountsend.Submission, error) {
	submissions.batchRequest = request
	return accountsend.Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Revision: 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients,
		State: accountsend.StateBuilding, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) AppendBatch(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.AppendRequest) (accountsend.Submission, error) {
	submissions.appendRequests = append(submissions.appendRequests, request)
	return accountsend.Submission{ID: request.ID, IdempotencyKey: submissions.batchRequest.IdempotencyKey, Revision: request.Revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: account.MailboxAddress, Recipients: submissions.batchRequest.Recipients,
		State: accountsend.StateBuilding, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) CommitBatch(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (accountsend.Submission, error) {
	submissions.commitRevision = revision
	return accountsend.Submission{ID: id, IdempotencyKey: submissions.batchRequest.IdempotencyKey, Revision: revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: account.MailboxAddress, Recipients: submissions.batchRequest.Recipients,
		State: accountsend.StateQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) BeginBatchReplace(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.BatchRequest, revision uint64) (accountsend.Submission, error) {
	submissions.batchRequest, submissions.commitRevision, submissions.replaceBatch = request, revision, true
	return accountsend.Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Revision: revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients,
		State: accountsend.StateEditing, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) AppendBatchReplace(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.AppendRequest) (accountsend.Submission, error) {
	submissions.appendRequests = append(submissions.appendRequests, request)
	return accountsend.Submission{ID: request.ID, IdempotencyKey: submissions.batchRequest.IdempotencyKey, Revision: request.Revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: account.MailboxAddress, Recipients: submissions.batchRequest.Recipients,
		State: accountsend.StateEditing, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) CommitBatchReplace(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (accountsend.Submission, error) {
	submissions.commitRevision = revision
	return accountsend.Submission{ID: id, IdempotencyKey: submissions.batchRequest.IdempotencyKey, Revision: revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: account.MailboxAddress, Recipients: submissions.batchRequest.Recipients,
		DeliveryCount: len(submissions.batchRequest.Recipients), State: accountsend.StateQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) AbortBatchReplace(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string, revision uint64) (accountsend.Submission, error) {
	return accountsend.Submission{ID: id, Revision: revision + 1, Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, State: accountsend.StateQueued}, nil
}

func (submissions *recordingSubmissions) GetSubmission(_ context.Context, scope accountsync.Scope, id string) (accountsend.Submission, error) {
	submissions.readID = id
	return accountsend.Submission{ID: id, Scope: scope, State: accountsend.StateCancelled}, nil
}

func (submissions *recordingSubmissions) Enqueue(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.Request) (accountsend.Submission, error) {
	submissions.request = request
	return accountsend.Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Scope: accountsync.Scope{Owner: owner, AccountID: account.ID},
		Revision: 1, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, State: accountsend.StateQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) Replace(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, request accountsend.Request, revision uint64) (accountsend.Submission, error) {
	submissions.replaceRequest, submissions.replaceRevision = request, revision
	return accountsend.Submission{ID: request.ID, IdempotencyKey: request.IdempotencyKey, Revision: revision + 1,
		Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients,
		State: accountsend.StateQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (submissions *recordingSubmissions) Cancel(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string) (accountsend.Submission, error) {
	submissions.cancelID = id
	return accountsend.Submission{ID: id, Scope: accountsync.Scope{Owner: owner, AccountID: account.ID}, State: accountsend.StateCancelled}, nil
}

type recordingSchedule struct{ retries int }

type recordingNotificationQueue struct {
	claim     accountsync.NewMailNotificationClaim
	holder    string
	limit     int
	token     string
	ids       []string
	delivered bool
	code      string
}

func (queue *recordingNotificationQueue) ClaimNewMailNotifications(_ context.Context, holder string, _ time.Time, _ time.Duration, limit int) (accountsync.NewMailNotificationClaim, error) {
	queue.holder, queue.limit = holder, limit
	return queue.claim, nil
}

func (queue *recordingNotificationQueue) CompleteNewMailNotifications(_ context.Context, token string, ids []string, delivered bool, code string, _ time.Time) error {
	queue.token, queue.ids, queue.delivered, queue.code = token, ids, delivered, code
	return nil
}

func (schedule *recordingSchedule) Retry(context.Context, accountsync.Scope) error {
	schedule.retries++
	return nil
}
func (*recordingSchedule) Diagnostics(context.Context, accountsync.Scope) (accountsync.Diagnostics, error) {
	return accountsync.Diagnostics{}, nil
}

type recordingMutations struct {
	existing accountsync.Mutation
	enqueues int
	last     accountsync.MutationRequest
}

func (mutations *recordingMutations) EnqueueMutation(_ context.Context, _ accounts.Owner, _ accounts.ExternalAccount, request accountsync.MutationRequest) (accountsync.Mutation, error) {
	mutations.enqueues++
	mutations.last = request
	return accountsync.Mutation{MutationRequest: request}, nil
}

type recordingIndex struct{ folders []accountsync.Folder }

func (index *recordingIndex) ListFolders(context.Context, accountsync.Scope) ([]accountsync.Folder, error) {
	return index.folders, nil
}

func (*recordingIndex) ListMessages(context.Context, accountsync.Scope, string, int) ([]accountsync.Message, error) {
	return nil, nil
}

func (mutations *recordingMutations) MutationByIdempotencyKey(_ context.Context, _ accounts.Owner, _ accounts.ExternalAccount, key string) (accountsync.Mutation, bool, error) {
	return mutations.existing, mutations.existing.IdempotencyKey == key, nil
}

func (mutations *recordingMutations) Mutation(_ context.Context, owner accounts.Owner, account accounts.ExternalAccount, id string) (accountsync.Mutation, error) {
	if mutations.existing.ID != id || mutations.existing.Scope.Owner != owner || mutations.existing.Scope.AccountID != account.ID {
		return accountsync.Mutation{}, accounts.ErrNotFound
	}
	return mutations.existing, nil
}

type recordingUnifiedIndex struct {
	owner    accounts.Owner
	filter   accountsync.UnifiedMessageFilter
	messages []accountsync.UnifiedMessage
	scope    accountsync.Scope
	stableID string
	raw      []byte
}

func (index *recordingUnifiedIndex) GetMessage(_ context.Context, scope accountsync.Scope, stableID string) (accountsync.Message, error) {
	index.scope, index.stableID = scope, stableID
	for _, candidate := range index.messages {
		if candidate.StableID == stableID && candidate.AccountID == scope.AccountID {
			return candidate.Message, nil
		}
	}
	return accountsync.Message{}, accounts.ErrNotFound
}

type recordingContactReader struct {
	owner    accounts.Owner
	contacts []contactsync.Contact
	books    []contactsync.OwnedAddressBook
}

type recordingCalendarReader struct {
	owner     accounts.Owner
	resources []calendarsync.OwnedResource
	calendars []calendarsync.OwnedCalendar
}

type recordingCalendarAccess struct {
	resource calendarsync.Resource
	scope    accountsync.Scope
	mutation calendarsync.Mutation
}

type recordingContactAccess struct {
	contact  contactsync.Contact
	scope    accountsync.Scope
	mutation contactsync.Mutation
}

func (access *recordingContactAccess) ProviderContact(_ context.Context, scope accountsync.Scope, _ string) (contactsync.Contact, error) {
	access.scope = scope
	return access.contact, nil
}
func (access *recordingContactAccess) EnqueueContactMutation(_ context.Context, scope accountsync.Scope, mutation contactsync.Mutation) (contactsync.Mutation, error) {
	access.scope, access.mutation = scope, mutation
	return mutation, nil
}
func (access *recordingContactAccess) ContactMutation(_ context.Context, scope accountsync.Scope, _ string) (contactsync.Mutation, error) {
	access.scope = scope
	return access.mutation, nil
}

func (reader *recordingContactReader) ListProviderContacts(_ context.Context, owner accounts.Owner) ([]contactsync.Contact, error) {
	reader.owner = owner
	return reader.contacts, nil
}

func (reader *recordingContactReader) ListProviderAddressBooks(_ context.Context, owner accounts.Owner) ([]contactsync.OwnedAddressBook, error) {
	reader.owner = owner
	return reader.books, nil
}

func (reader *recordingCalendarReader) ListProviderCalendarResources(_ context.Context, owner accounts.Owner) ([]calendarsync.OwnedResource, error) {
	reader.owner = owner
	return reader.resources, nil
}

func (reader *recordingCalendarReader) ListProviderCalendars(_ context.Context, owner accounts.Owner) ([]calendarsync.OwnedCalendar, error) {
	reader.owner = owner
	return reader.calendars, nil
}

func (access *recordingCalendarAccess) ProviderCalendarResource(_ context.Context, scope accountsync.Scope, _ string) (calendarsync.Resource, error) {
	access.scope = scope
	return access.resource, nil
}
func (access *recordingCalendarAccess) EnqueueCalendarMutation(_ context.Context, scope accountsync.Scope, mutation calendarsync.Mutation) (calendarsync.Mutation, error) {
	access.scope, access.mutation = scope, mutation
	return mutation, nil
}
func (access *recordingCalendarAccess) CalendarMutation(_ context.Context, scope accountsync.Scope, _ string) (calendarsync.Mutation, error) {
	access.scope = scope
	return access.mutation, nil
}

func (index *recordingUnifiedIndex) ListUnifiedMessages(_ context.Context, owner accounts.Owner, filter accountsync.UnifiedMessageFilter) ([]accountsync.UnifiedMessage, error) {
	index.owner, index.filter = owner, filter
	return index.messages, nil
}

func (index *recordingUnifiedIndex) GetMessageSource(_ context.Context, scope accountsync.Scope, stableID string) (accountsync.Message, []byte, error) {
	index.scope, index.stableID = scope, stableID
	return accountsync.Message{StableID: stableID, Subject: "Provider"}, index.raw, nil
}

func (service *recordingService) List(_ context.Context, owner accounts.Owner) ([]accounts.ExternalAccount, error) {
	service.owner = owner
	return service.listed, nil
}

func (service *recordingService) Pause(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error) {
	return accounts.ExternalAccount{}, nil
}

func (service *recordingService) Resume(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error) {
	return accounts.ExternalAccount{}, nil
}

func (service *recordingService) UpdatePresentation(_ context.Context, owner accounts.Owner, _ accounts.AccountID, _ uint64, update accounts.PresentationUpdate) (accounts.ExternalAccount, error) {
	service.owner, service.update = owner, update
	return service.created, nil
}

func (service *recordingService) Reconnect(_ context.Context, owner accounts.Owner, _ accounts.AccountID, _ uint64, credentials accounts.Credentials) (accounts.ExternalAccount, error) {
	service.owner = owner
	service.create.Credentials = credentials
	return service.created, nil
}

func (service *recordingService) Delete(context.Context, accounts.Owner, accounts.AccountID, uint64) error {
	return nil
}

type fakeDiscovery struct {
	address string
	result  accounts.DiscoveryResult
}

type recordingOAuthConnection struct {
	owner         accounts.Owner
	request       accounts.OAuthConnectionRequest
	authorization accounts.OAuthAuthorization
	account       accounts.ExternalAccount
	state         string
	code          string
}

func (connection *recordingOAuthConnection) Begin(_ context.Context, owner accounts.Owner, request accounts.OAuthConnectionRequest) (accounts.OAuthAuthorization, error) {
	connection.owner, connection.request = owner, request
	return connection.authorization, nil
}

func (connection *recordingOAuthConnection) Complete(_ context.Context, owner accounts.Owner, state, code string) (accounts.ExternalAccount, error) {
	connection.owner, connection.state, connection.code = owner, state, code
	return connection.account, nil
}

func (discovery *fakeDiscovery) Discover(_ context.Context, address string) (accounts.DiscoveryResult, error) {
	discovery.address = address
	return discovery.result, nil
}

func stableCode(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for index, character := range message {
		if character == ':' {
			return message[:index]
		}
	}
	return message
}

func containsText(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
