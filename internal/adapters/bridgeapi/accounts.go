// Package bridgeapi exposes typed Go application operations to the legacy
// Tachyon HTTP adapter over the authenticated loopback bridge.
package bridgeapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/auth"
	"github.com/d31ma/caduceus/internal/calendarsync"
	"github.com/d31ma/caduceus/internal/contactsync"
	"github.com/d31ma/caduceus/internal/platform/imapwire"
)

const AccountsPrefix = "accounts."

type Authenticator interface {
	Authenticate(context.Context, string) (auth.Principal, error)
}

type AccountService interface {
	Create(context.Context, accounts.Owner, accounts.CreateRequest) (accounts.ExternalAccount, error)
	Validate(context.Context, accounts.Owner, accounts.CreateRequest) (accounts.Capabilities, error)
	Get(context.Context, accounts.Owner, accounts.AccountID) (accounts.ExternalAccount, error)
	List(context.Context, accounts.Owner) ([]accounts.ExternalAccount, error)
	UpdatePresentation(context.Context, accounts.Owner, accounts.AccountID, uint64, accounts.PresentationUpdate) (accounts.ExternalAccount, error)
	Pause(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error)
	Resume(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error)
	Reconnect(context.Context, accounts.Owner, accounts.AccountID, uint64, accounts.Credentials) (accounts.ExternalAccount, error)
	Delete(context.Context, accounts.Owner, accounts.AccountID, uint64) error
}

type AccountDiscovery interface {
	Discover(context.Context, string) (accounts.DiscoveryResult, error)
}

type OAuthConnection interface {
	Begin(context.Context, accounts.Owner, accounts.OAuthConnectionRequest) (accounts.OAuthAuthorization, error)
	Complete(context.Context, accounts.Owner, string, string) (accounts.ExternalAccount, error)
}

type AccountsDispatcher struct {
	authenticator     Authenticator
	service           AccountService
	discovery         AccountDiscovery
	schedule          accountsync.ScheduleControl
	index             accountsync.IndexReader
	unifiedIndex      accountsync.UnifiedIndexReader
	mutations         accountsync.MutationEnqueuer
	submissions       accountsend.Enqueuer
	submissionReader  accountsend.Reader
	contacts          contactsync.Reader
	contactMutations  contactsync.MutationAccess
	calendars         calendarsync.Reader
	calendarMutations calendarsync.MutationAccess
	notifications     accountsync.NotificationQueue
	microsoftOAuth    OAuthConnection
	googleOAuth       OAuthConnection
}

type AccountExtensions struct {
	Schedule          accountsync.ScheduleControl
	Index             accountsync.IndexReader
	UnifiedIndex      accountsync.UnifiedIndexReader
	Mutations         accountsync.MutationEnqueuer
	Submissions       accountsend.Enqueuer
	SubmissionReader  accountsend.Reader
	Contacts          contactsync.Reader
	ContactMutations  contactsync.MutationAccess
	Calendars         calendarsync.Reader
	CalendarMutations calendarsync.MutationAccess
	Notifications     accountsync.NotificationQueue
	OAuth             OAuthConnection
	GoogleOAuth       OAuthConnection
}

func NewAccountsDispatcher(authenticator Authenticator, service AccountService, discovery AccountDiscovery, extensions ...AccountExtensions) *AccountsDispatcher {
	var extra AccountExtensions
	if len(extensions) > 0 {
		extra = extensions[0]
	}
	return &AccountsDispatcher{authenticator: authenticator, service: service, discovery: discovery, schedule: extra.Schedule, index: extra.Index, unifiedIndex: extra.UnifiedIndex, mutations: extra.Mutations, submissions: extra.Submissions, submissionReader: extra.SubmissionReader, contacts: extra.Contacts, contactMutations: extra.ContactMutations, calendars: extra.Calendars, calendarMutations: extra.CalendarMutations, notifications: extra.Notifications, microsoftOAuth: extra.OAuth, googleOAuth: extra.GoogleOAuth}
}

func AccountOperations() []string {
	operations := []string{
		"accounts.create", "accounts.delete", "accounts.discover", "accounts.get", "accounts.contacts", "accounts.contact_books", "accounts.contact_mutate", "accounts.contact_mutation", "accounts.calendar_resources", "accounts.calendar_mutate", "accounts.calendar_mutation",
		"accounts.folders", "accounts.list", "accounts.message", "accounts.messages", "accounts.mutate", "accounts.mutation", "accounts.pause", "accounts.providers", "accounts.reconnect",
		"accounts.resume", "accounts.retry", "accounts.diagnostics", "accounts.validate",
		"accounts.submit", "accounts.submit_batch_begin", "accounts.submit_batch_append", "accounts.submit_batch_commit",
		"accounts.submit_batch_update_begin", "accounts.submit_batch_update_append", "accounts.submit_batch_update_commit", "accounts.submit_batch_update_abort",
		"accounts.update_submission", "accounts.cancel_submission",
		"accounts.submission", "accounts.unified_messages", "accounts.message_source", "accounts.update_presentation",
		"accounts.notification_claim", "accounts.notification_complete",
		"accounts.microsoft_oauth_begin", "accounts.microsoft_oauth_complete", "accounts.google_oauth_begin", "accounts.google_oauth_complete",
	}
	sort.Strings(operations)
	return operations
}

func providerContactProjection(contact contactsync.Contact) map[string]any {
	return map[string]any{
		"id": contact.ID, "source": "carddav", "accountId": contact.AccountID, "addressBookId": contact.AddressBookID,
		"href": contact.Href, "etag": contact.ETag, "uid": contact.UID, "name": contact.Name,
		"givenName": contact.GivenName, "familyName": contact.FamilyName, "company": contact.Company, "title": contact.Title,
		"emails": contact.Emails, "phones": contact.Phones, "addresses": contact.Addresses, "urls": contact.URLs,
		"nicknames": contact.Nicknames, "birthday": contact.Birthday, "notes": contact.Notes,
		"groupIds": []string{}, "createdAt": contact.RemoteUpdatedAt, "updatedAt": contact.RemoteUpdatedAt, "readOnly": false,
	}
}

func (dispatcher *AccountsDispatcher) Dispatch(ctx context.Context, operation string, payload json.RawMessage) (any, error) {
	switch operation {
	case "accounts.notification_claim":
		request, err := decodeStrict[notificationClaimRequest](payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.notifications == nil {
			return nil, failure("EUNAVAILABLE", "provider notification queue is not configured")
		}
		result, err := dispatcher.notifications.ClaimNewMailNotifications(ctx, request.Holder, time.Now().UTC(), 2*time.Minute, request.Limit)
		return result, classify(err)
	case "accounts.notification_complete":
		request, err := decodeStrict[notificationCompleteRequest](payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.notifications == nil {
			return nil, failure("EUNAVAILABLE", "provider notification queue is not configured")
		}
		err = dispatcher.notifications.CompleteNewMailNotifications(ctx, request.Token, request.IDs, request.Delivered, request.Code, time.Now().UTC())
		return map[string]any{"completed": err == nil, "count": len(request.IDs)}, classify(err)
	case "accounts.providers":
		request, owner, err := decodeForOwner[tokenRequest](ctx, dispatcher, payload)
		_ = request
		_ = owner
		if err != nil {
			return nil, err
		}
		return accounts.ProviderCatalogWithOAuth(dispatcher.microsoftOAuth != nil, dispatcher.googleOAuth != nil), nil
	case "accounts.microsoft_oauth_begin":
		request, owner, err := decodeForOwner[oauthBeginRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.microsoftOAuth == nil {
			return nil, failure("EUNAVAILABLE", "Microsoft OAuth is not configured on this server")
		}
		result, err := dispatcher.microsoftOAuth.Begin(ctx, owner, request.domainRequest())
		return result, classify(err)
	case "accounts.microsoft_oauth_complete":
		request, owner, err := decodeForOwner[oauthCompleteRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.microsoftOAuth == nil {
			return nil, failure("EUNAVAILABLE", "Microsoft OAuth is not configured on this server")
		}
		result, err := dispatcher.microsoftOAuth.Complete(ctx, owner, request.State, request.Code)
		return result, classify(err)
	case "accounts.google_oauth_begin":
		request, owner, err := decodeForOwner[oauthBeginRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.googleOAuth == nil {
			return nil, failure("EUNAVAILABLE", "Google OAuth is not configured on this server")
		}
		result, err := dispatcher.googleOAuth.Begin(ctx, owner, request.domainRequest())
		return result, classify(err)
	case "accounts.google_oauth_complete":
		request, owner, err := decodeForOwner[oauthCompleteRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.googleOAuth == nil {
			return nil, failure("EUNAVAILABLE", "Google OAuth is not configured on this server")
		}
		result, err := dispatcher.googleOAuth.Complete(ctx, owner, request.State, request.Code)
		return result, classify(err)
	case "accounts.discover":
		request, _, err := decodeForOwner[discoverRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.discovery == nil {
			return nil, failure("EUNAVAILABLE", "account discovery is not configured")
		}
		result, err := dispatcher.discovery.Discover(ctx, request.Address)
		return result, classify(err)
	case "accounts.list":
		_, owner, err := decodeForOwner[tokenRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.List(ctx, owner)
		return result, classify(err)
	case "accounts.contacts":
		_, owner, err := decodeForOwner[tokenRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.contacts == nil {
			return nil, failure("EUNAVAILABLE", "provider contacts are not configured")
		}
		result, err := dispatcher.contacts.ListProviderContacts(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		ownedAccounts, err := dispatcher.service.List(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		enabledAccounts := make(map[accounts.AccountID]bool, len(ownedAccounts))
		for _, account := range ownedAccounts {
			enabledAccounts[account.ID] = contactsync.Enabled(account)
		}
		contacts := make([]map[string]any, 0, len(result))
		for _, contact := range result {
			if !enabledAccounts[contact.AccountID] {
				continue
			}
			contacts = append(contacts, providerContactProjection(contact))
		}
		return map[string]any{"contacts": contacts}, nil
	case "accounts.calendar_resources":
		_, owner, err := decodeForOwner[tokenRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.calendars == nil {
			return nil, failure("EUNAVAILABLE", "provider calendars are not configured")
		}
		resources, err := dispatcher.calendars.ListProviderCalendarResources(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		ownedAccounts, err := dispatcher.service.List(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		accountsByID := make(map[accounts.AccountID]accounts.ExternalAccount, len(ownedAccounts))
		for _, account := range ownedAccounts {
			accountsByID[account.ID] = account
		}
		ownedCalendars, err := dispatcher.calendars.ListProviderCalendars(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		projectedCalendars := make([]map[string]any, 0, len(ownedCalendars))
		for _, calendar := range ownedCalendars {
			account, exists := accountsByID[calendar.AccountID]
			if !exists || !calendarsync.Enabled(account) {
				continue
			}
			label := strings.TrimSpace(account.Label)
			if label == "" {
				label = account.MailboxAddress
			}
			projectedCalendars = append(projectedCalendars, map[string]any{
				"accountId": calendar.AccountID, "accountLabel": label, "accountColor": account.Color, "mailboxAddress": account.MailboxAddress,
				"calendarId": calendar.StableID, "calendarName": calendar.Name, "calendarColor": calendar.Color,
				"writable": calendar.Writable || account.Connection.CalDAV != nil,
			})
		}
		projected := make([]map[string]any, 0, len(resources))
		for _, resource := range resources {
			account, exists := accountsByID[resource.AccountID]
			if !exists || !calendarsync.Enabled(account) {
				continue
			}
			label := strings.TrimSpace(account.Label)
			if label == "" {
				label = account.MailboxAddress
			}
			source := string(account.Provider) + "-calendar"
			if account.Connection.CalDAV != nil {
				source = "caldav"
			}
			projected = append(projected, map[string]any{
				"id": resource.ID, "source": source, "accountId": resource.AccountID, "accountLabel": label, "accountColor": account.Color,
				"mailboxAddress": account.MailboxAddress, "calendarId": resource.CalendarID, "calendarName": resource.CalendarName, "calendarColor": resource.CalendarColor,
				"etag": resource.ETag, "rawICalendar": resource.RawICalendar, "updatedAt": resource.RemoteUpdatedAt,
			})
		}
		return map[string]any{"calendars": projectedCalendars, "resources": projected}, nil
	case "accounts.calendar_mutate":
		request, owner, err := decodeForOwner[calendarMutationRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.calendarMutations == nil || dispatcher.calendars == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "provider calendar mutation queue is not configured")
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if !calendarsync.Enabled(account) {
			return nil, failure("EUNSUPPORTED", "the external account does not support calendar writes")
		}
		scope := accountsync.Scope{Owner: owner, AccountID: account.ID}
		providerCalendars, err := dispatcher.calendars.ListProviderCalendars(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		var resourceID, expectedETag string
		desired := calendarsync.Resource{RawICalendar: request.RawICalendar}
		if request.Kind == calendarsync.MutationCreate {
			if strings.TrimSpace(request.CalendarID) == "" || request.ResourceID != "" || request.ExpectedETag != "" {
				return nil, failure("EBADREQUEST", "an active writable provider calendar is required")
			}
			calendar, found := ownedCalendar(providerCalendars, account.ID, request.CalendarID)
			if !found || !calendar.Writable && account.Connection.CalDAV == nil {
				return nil, failure("ENOTFOUND", "writable provider calendar not found")
			}
			href, identityErr := newProviderCalendarResource(calendar.URL)
			if identityErr != nil {
				return nil, failure("EUNAVAILABLE", "provider calendar identity is unavailable")
			}
			resourceID = calendarsync.RemoteResourceID(account.ID, href)
			desired.ID, desired.AccountID, desired.CalendarID, desired.Href = resourceID, account.ID, calendar.StableID, href
		} else {
			if (request.Kind != calendarsync.MutationUpdate && request.Kind != calendarsync.MutationDelete) || request.CalendarID != "" {
				return nil, failure("EBADREQUEST", "invalid provider calendar mutation")
			}
			current, loadErr := dispatcher.calendarMutations.ProviderCalendarResource(ctx, scope, request.ResourceID)
			if loadErr != nil {
				return nil, classify(loadErr)
			}
			calendar, found := ownedCalendar(providerCalendars, account.ID, current.CalendarID)
			if !found || !calendar.Writable && account.Connection.CalDAV == nil {
				return nil, failure("ENOTFOUND", "writable provider calendar not found")
			}
			if current.ETag != request.ExpectedETag {
				return nil, failure("ECONFLICT", "the provider calendar event changed; refresh and retry")
			}
			resourceID, expectedETag = current.ID, current.ETag
		}
		mutationID, err := newCalendarMutationID()
		if err != nil {
			return nil, failure("EUNAVAILABLE", "provider calendar mutation identity is unavailable")
		}
		candidate := calendarsync.MutationRequest{ID: mutationID, IdempotencyKey: request.IdempotencyKey, Kind: request.Kind, ResourceID: resourceID, ExpectedETag: expectedETag, Desired: desired}
		if err := calendarsync.ValidateMutation(candidate); err != nil {
			return nil, classify(err)
		}
		now := time.Now().UTC()
		result, err := dispatcher.calendarMutations.EnqueueCalendarMutation(ctx, scope, calendarsync.Mutation{MutationRequest: candidate, Scope: scope, State: calendarsync.MutationQueued, CreatedAt: now, UpdatedAt: now})
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, scope)
		}
		return result, classify(err)
	case "accounts.calendar_mutation":
		request, owner, err := decodeForOwner[calendarMutationStatusRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.calendarMutations == nil {
			return nil, failure("EUNAVAILABLE", "provider calendar mutation queue is not configured")
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		result, err := dispatcher.calendarMutations.CalendarMutation(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.MutationID)
		return result, classify(err)
	case "accounts.contact_books":
		_, owner, err := decodeForOwner[tokenRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.contacts == nil {
			return nil, failure("EUNAVAILABLE", "provider contact address books are not configured")
		}
		books, err := dispatcher.contacts.ListProviderAddressBooks(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		ownedAccounts, err := dispatcher.service.List(ctx, owner)
		if err != nil {
			return nil, classify(err)
		}
		accountsByID := make(map[accounts.AccountID]accounts.ExternalAccount, len(ownedAccounts))
		for _, account := range ownedAccounts {
			accountsByID[account.ID] = account
		}
		projected := make([]map[string]any, 0, len(books))
		for _, book := range books {
			account, exists := accountsByID[book.AccountID]
			if !exists || !contactsync.Enabled(account) {
				continue
			}
			label := strings.TrimSpace(account.Label)
			if label == "" {
				label = account.MailboxAddress
			}
			projected = append(projected, map[string]any{"accountId": book.AccountID, "addressBookId": book.StableID, "name": book.Name, "accountLabel": label, "mailboxAddress": account.MailboxAddress, "writable": book.Writable || account.Connection.CardDAV != nil})
		}
		return map[string]any{"addressBooks": projected}, nil
	case "accounts.contact_mutate":
		request, owner, err := decodeForOwner[contactMutationRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.contactMutations == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "provider contact mutation queue is not configured")
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if !contactsync.Enabled(account) {
			return nil, failure("EUNSUPPORTED", "the external account does not support provider contact writes")
		}
		scope := accountsync.Scope{Owner: owner, AccountID: account.ID}
		if dispatcher.contacts == nil {
			return nil, failure("EUNAVAILABLE", "provider contact address books are not configured")
		}
		books, listErr := dispatcher.contacts.ListProviderAddressBooks(ctx, owner)
		if listErr != nil {
			return nil, classify(listErr)
		}
		var contactID, expectedETag string
		desired := request.Contact.domainContact()
		if request.Kind == contactsync.MutationCreate {
			if strings.TrimSpace(request.AddressBookID) == "" || request.ContactID != "" || request.ExpectedETag != "" {
				return nil, failure("EBADREQUEST", "an active writable provider address book is required")
			}
			book, found := ownedAddressBook(books, account.ID, request.AddressBookID)
			if !found || !book.Writable && account.Connection.CardDAV == nil {
				return nil, failure("ENOTFOUND", "writable provider address book not found")
			}
			uid, href, identityErr := newProviderContactResource(book.URL)
			if identityErr != nil {
				return nil, failure("EUNAVAILABLE", "provider contact identity is unavailable")
			}
			contactID = contactsync.RemoteContactID(account.ID, href)
			desired.ID, desired.AccountID, desired.AddressBookID, desired.Href, desired.UID = contactID, account.ID, book.StableID, href, uid
		} else {
			if request.Kind != contactsync.MutationUpdate && request.Kind != contactsync.MutationDelete || request.AddressBookID != "" {
				return nil, failure("EBADREQUEST", "invalid provider contact mutation")
			}
			current, loadErr := dispatcher.contactMutations.ProviderContact(ctx, scope, request.ContactID)
			if loadErr != nil {
				return nil, classify(loadErr)
			}
			book, found := ownedAddressBook(books, account.ID, current.AddressBookID)
			if !found || !book.Writable && account.Connection.CardDAV == nil {
				return nil, failure("ENOTFOUND", "writable provider address book not found")
			}
			if current.ETag != request.ExpectedETag {
				return nil, failure("ECONFLICT", "the provider contact changed; refresh and retry")
			}
			contactID, expectedETag = current.ID, current.ETag
		}
		mutationID, err := newContactMutationID()
		if err != nil {
			return nil, failure("EUNAVAILABLE", "provider contact mutation identity is unavailable")
		}
		candidate := contactsync.MutationRequest{ID: mutationID, IdempotencyKey: request.IdempotencyKey, Kind: request.Kind, ContactID: contactID, ExpectedETag: expectedETag, Desired: desired}
		if err := contactsync.ValidateMutation(candidate); err != nil {
			return nil, classify(err)
		}
		now := time.Now().UTC()
		result, err := dispatcher.contactMutations.EnqueueContactMutation(ctx, scope, contactsync.Mutation{MutationRequest: candidate, Scope: scope, State: contactsync.MutationQueued, CreatedAt: now, UpdatedAt: now})
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, scope)
		}
		return result, classify(err)
	case "accounts.contact_mutation":
		request, owner, err := decodeForOwner[contactMutationStatusRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if dispatcher.contactMutations == nil {
			return nil, failure("EUNAVAILABLE", "provider contact mutation queue is not configured")
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		result, err := dispatcher.contactMutations.ContactMutation(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.MutationID)
		return result, classify(err)
	case "accounts.get":
		request, owner, err := decodeForOwner[idRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		return result, classify(err)
	case "accounts.create":
		request, owner, err := decodeForOwner[createRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Create(ctx, owner, request.domainRequest())
		return result, classify(err)
	case "accounts.validate":
		request, owner, err := decodeForOwner[createRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Validate(ctx, owner, request.domainRequest())
		return map[string]any{"capabilities": result}, classify(err)
	case "accounts.pause":
		request, owner, err := decodeForOwner[revisionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Pause(ctx, owner, accounts.AccountID(request.ID), request.Revision)
		return result, classify(err)
	case "accounts.resume":
		request, owner, err := decodeForOwner[revisionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Resume(ctx, owner, accounts.AccountID(request.ID), request.Revision)
		return result, classify(err)
	case "accounts.update_presentation":
		request, owner, err := decodeForOwner[presentationRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.UpdatePresentation(ctx, owner, accounts.AccountID(request.ID), request.Revision, accounts.PresentationUpdate{
			Label: request.Label, Color: request.Color, Order: request.Order,
		})
		return result, classify(err)
	case "accounts.reconnect":
		request, owner, err := decodeForOwner[reconnectRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		result, err := dispatcher.service.Reconnect(
			ctx, owner, accounts.AccountID(request.ID), request.Revision, request.Credentials.domainCredentials(),
		)
		return result, classify(err)
	case "accounts.retry":
		request, owner, err := decodeForOwner[revisionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if account.Revision != request.Revision {
			return nil, classify(accounts.ErrConflict)
		}
		if dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account scheduler is not configured")
		}
		err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		return map[string]bool{"scheduled": err == nil}, classify(err)
	case "accounts.diagnostics":
		request, owner, err := decodeForOwner[idRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account scheduler is not configured")
		}
		result, err := dispatcher.schedule.Diagnostics(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		return map[string]any{"state": account.State, "failure": account.Failure, "progress": account.Progress, "sync": result}, classify(err)
	case "accounts.folders":
		request, owner, err := decodeForOwner[idRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.index == nil {
			return nil, failure("EUNAVAILABLE", "account index is not configured")
		}
		result, err := dispatcher.index.ListFolders(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		return map[string]any{"folders": result}, classify(err)
	case "accounts.messages":
		request, owner, err := decodeForOwner[messagesRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.index == nil {
			return nil, failure("EUNAVAILABLE", "account index is not configured")
		}
		result, err := dispatcher.index.ListMessages(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.Folder, request.Limit)
		return map[string]any{"messages": result}, classify(err)
	case "accounts.unified_messages":
		request, owner, err := decodeForOwner[unifiedMessagesRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		var receivedAtFrom *time.Time
		if request.ReceivedAtFrom != "" {
			parsed, parseErr := time.Parse(time.RFC3339Nano, request.ReceivedAtFrom)
			if parseErr != nil {
				return nil, failure("EBADREQUEST", "receivedAtFrom must be an RFC 3339 timestamp")
			}
			parsed = parsed.UTC()
			receivedAtFrom = &parsed
		}
		if dispatcher.unifiedIndex == nil {
			return nil, failure("EUNAVAILABLE", "unified account index is not configured")
		}
		result, err := dispatcher.unifiedIndex.ListUnifiedMessages(ctx, owner, accountsync.UnifiedMessageFilter{AccountID: accounts.AccountID(request.AccountID),
			FolderRole: request.FolderRole, FolderStableID: request.FolderStableID, Query: request.Query, Read: request.Read, Starred: request.Starred,
			ReceivedAtFrom: receivedAtFrom, OldestFirst: request.OldestFirst, Limit: request.Limit, Offset: request.Offset})
		return map[string]any{"messages": result}, classify(err)
	case "accounts.message_source":
		request, owner, err := decodeForOwner[messageSourceRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.unifiedIndex == nil {
			return nil, failure("EUNAVAILABLE", "unified account index is not configured")
		}
		message, raw, err := dispatcher.unifiedIndex.GetMessageSource(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.StableID)
		if err != nil {
			return nil, classify(err)
		}
		return map[string]any{"message": message, "rawRFC822Base64": base64.StdEncoding.EncodeToString(raw)}, nil
	case "accounts.message":
		request, owner, err := decodeForOwner[messageSourceRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.unifiedIndex == nil {
			return nil, failure("EUNAVAILABLE", "unified account index is not configured")
		}
		message, err := dispatcher.unifiedIndex.GetMessage(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.StableID)
		return map[string]any{"message": message}, classify(err)
	case "accounts.mutate":
		request, owner, err := decodeForOwner[mutationRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.mutations == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account mutation queue is not configured")
		}
		mutationID, err := newMutationID()
		if err != nil {
			return nil, failure("EUNAVAILABLE", "account mutation identity is unavailable")
		}
		candidate := accountsync.MutationRequest{ID: mutationID, IdempotencyKey: request.IdempotencyKey,
			Kind: request.Kind, MessageStableIDs: request.MessageStableIDs, TargetFolderID: request.TargetFolderID,
			FolderStableID: request.FolderStableID, ParentStableID: request.ParentStableID, FolderName: request.FolderName, Value: request.Value}
		candidate, err = dispatcher.prepareFolderMutation(ctx, owner, account, candidate)
		if err != nil {
			return nil, classify(err)
		}
		if err := accountsync.ValidateMutation(candidate); err != nil {
			return nil, classify(err)
		}
		if account.Revision != request.Revision {
			existing, found, lookupErr := dispatcher.mutations.MutationByIdempotencyKey(ctx, owner, account, request.IdempotencyKey)
			if lookupErr != nil {
				return nil, classify(lookupErr)
			}
			if !found || !equivalentMutation(existing.MutationRequest, candidate) {
				return nil, classify(accounts.ErrConflict)
			}
			return existing, nil
		}
		result, err := dispatcher.mutations.EnqueueMutation(ctx, owner, account, candidate)
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		}
		return result, classify(err)
	case "accounts.mutation":
		request, owner, err := decodeForOwner[providerMutationStatusRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.mutations == nil {
			return nil, failure("EUNAVAILABLE", "account mutation journal is not configured")
		}
		mutation, err := dispatcher.mutations.Mutation(ctx, owner, account, request.MutationID)
		return mutation, classify(err)
	case "accounts.submit":
		request, owner, err := decodeForOwner[submissionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		if len(request.RawRFC822Base64) > base64.StdEncoding.EncodedLen(accountsend.MaxRawBytes) {
			return nil, failure("EBADREQUEST", "external message exceeds the submission limit")
		}
		raw, decodeErr := base64.StdEncoding.DecodeString(request.RawRFC822Base64)
		if decodeErr != nil {
			return nil, failure("EBADREQUEST", "external message encoding is invalid")
		}
		submissionID, err := newSubmissionID()
		if err != nil {
			return nil, failure("EUNAVAILABLE", "account submission identity is unavailable")
		}
		result, err := dispatcher.submissions.Enqueue(ctx, owner, account, accountsend.Request{ID: submissionID, IdempotencyKey: request.IdempotencyKey,
			EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, RawRFC822: raw, NotBefore: request.NotBefore})
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		}
		return result, classify(err)
	case "accounts.submit_batch_begin":
		request, owner, err := decodeForOwner[submissionBatchBeginRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		submissionID, err := newSubmissionID()
		if err != nil {
			return nil, failure("EUNAVAILABLE", "account submission identity is unavailable")
		}
		result, err := dispatcher.submissions.BeginBatch(ctx, owner, account, accountsend.BatchRequest{ID: submissionID,
			IdempotencyKey: request.IdempotencyKey, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, NotBefore: request.NotBefore})
		return result, classify(err)
	case "accounts.submit_batch_append":
		request, owner, err := decodeForOwner[submissionBatchAppendRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		if len(request.RawRFC822Base64) > base64.StdEncoding.EncodedLen(accountsend.MaxRawBytes) {
			return nil, failure("EBADREQUEST", "external message exceeds the submission limit")
		}
		raw, decodeErr := base64.StdEncoding.DecodeString(request.RawRFC822Base64)
		if decodeErr != nil {
			return nil, failure("EBADREQUEST", "external message encoding is invalid")
		}
		result, err := dispatcher.submissions.AppendBatch(ctx, owner, account, accountsend.AppendRequest{ID: request.SubmissionID,
			Revision: request.Revision, Index: request.Index, Recipient: request.Recipient, RawRFC822: raw})
		return result, classify(err)
	case "accounts.submit_batch_commit":
		request, owner, err := decodeForOwner[submissionBatchCommitRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		result, err := dispatcher.submissions.CommitBatch(ctx, owner, account, request.SubmissionID, request.Revision)
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		}
		return result, classify(err)
	case "accounts.submit_batch_update_begin":
		request, owner, err := decodeForOwner[submissionBatchUpdateBeginRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		result, err := dispatcher.submissions.BeginBatchReplace(ctx, owner, account, accountsend.BatchRequest{ID: request.SubmissionID,
			IdempotencyKey: request.IdempotencyKey, EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, NotBefore: request.NotBefore}, request.Revision)
		return result, classify(err)
	case "accounts.submit_batch_update_append":
		request, owner, err := decodeForOwner[submissionBatchAppendRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		if len(request.RawRFC822Base64) > base64.StdEncoding.EncodedLen(accountsend.MaxRawBytes) {
			return nil, failure("EBADREQUEST", "external message exceeds the submission limit")
		}
		raw, decodeErr := base64.StdEncoding.DecodeString(request.RawRFC822Base64)
		if decodeErr != nil {
			return nil, failure("EBADREQUEST", "external message encoding is invalid")
		}
		result, err := dispatcher.submissions.AppendBatchReplace(ctx, owner, account, accountsend.AppendRequest{ID: request.SubmissionID,
			Revision: request.Revision, Index: request.Index, Recipient: request.Recipient, RawRFC822: raw})
		return result, classify(err)
	case "accounts.submit_batch_update_commit":
		request, owner, err := decodeForOwner[submissionBatchCommitRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		result, err := dispatcher.submissions.CommitBatchReplace(ctx, owner, account, request.SubmissionID, request.Revision)
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		}
		return result, classify(err)
	case "accounts.submit_batch_update_abort":
		request, owner, err := decodeForOwner[submissionBatchCommitRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		result, err := dispatcher.submissions.AbortBatchReplace(ctx, owner, account, request.SubmissionID, request.Revision)
		return result, classify(err)
	case "accounts.update_submission":
		request, owner, err := decodeForOwner[updateSubmissionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil || dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		if len(request.RawRFC822Base64) > base64.StdEncoding.EncodedLen(accountsend.MaxRawBytes) {
			return nil, failure("EBADREQUEST", "external message exceeds the submission limit")
		}
		raw, decodeErr := base64.StdEncoding.DecodeString(request.RawRFC822Base64)
		if decodeErr != nil {
			return nil, failure("EBADREQUEST", "external message encoding is invalid")
		}
		result, err := dispatcher.submissions.Replace(ctx, owner, account, accountsend.Request{ID: request.SubmissionID, IdempotencyKey: request.IdempotencyKey,
			EnvelopeFrom: request.EnvelopeFrom, Recipients: request.Recipients, RawRFC822: raw, NotBefore: request.NotBefore}, request.Revision)
		if err == nil {
			err = dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
		}
		return result, classify(err)
	case "accounts.cancel_submission":
		request, owner, err := decodeForOwner[cancelSubmissionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissions == nil {
			return nil, failure("EUNAVAILABLE", "account submission queue is not configured")
		}
		result, err := dispatcher.submissions.Cancel(ctx, owner, account, request.SubmissionID)
		return result, classify(err)
	case "accounts.submission":
		request, owner, err := decodeForOwner[submissionStatusRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		account, err := dispatcher.service.Get(ctx, owner, accounts.AccountID(request.ID))
		if err != nil {
			return nil, classify(err)
		}
		if dispatcher.submissionReader == nil {
			return nil, failure("EUNAVAILABLE", "account submission journal is not configured")
		}
		result, err := dispatcher.submissionReader.GetSubmission(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID}, request.SubmissionID)
		return result, classify(err)
	case "accounts.delete":
		request, owner, err := decodeForOwner[revisionRequest](ctx, dispatcher, payload)
		if err != nil {
			return nil, err
		}
		if err := dispatcher.service.Delete(ctx, owner, accounts.AccountID(request.ID), request.Revision); err != nil {
			return nil, classify(err)
		}
		if dispatcher.schedule == nil {
			return nil, failure("EUNAVAILABLE", "account scheduler is not configured")
		}
		if err := dispatcher.schedule.Retry(ctx, accountsync.Scope{Owner: owner, AccountID: accounts.AccountID(request.ID)}); err != nil {
			return nil, classify(err)
		}
		return map[string]bool{"deleting": true}, nil
	default:
		return nil, failure("EBADREQUEST", "unknown account operation")
	}
}

func equivalentMutation(left, right accountsync.MutationRequest) bool {
	return left.IdempotencyKey == right.IdempotencyKey && left.Kind == right.Kind && left.TargetFolderID == right.TargetFolderID &&
		left.FolderStableID == right.FolderStableID && left.ParentStableID == right.ParentStableID && left.FolderName == right.FolderName &&
		left.FolderPath == right.FolderPath && left.NewFolderPath == right.NewFolderPath &&
		left.Value == right.Value && slices.Equal(left.MessageStableIDs, right.MessageStableIDs)
}

func (dispatcher *AccountsDispatcher) prepareFolderMutation(ctx context.Context, owner accounts.Owner, account accounts.ExternalAccount, candidate accountsync.MutationRequest) (accountsync.MutationRequest, error) {
	if !accountsync.IsFolderMutation(candidate.Kind) {
		return candidate, nil
	}
	if !account.Capabilities.MailWrite {
		return accountsync.MutationRequest{}, accounts.ErrInvalid
	}
	if dispatcher.index == nil {
		return accountsync.MutationRequest{}, failure("EUNAVAILABLE", "account folder index is not configured")
	}
	folders, err := dispatcher.index.ListFolders(ctx, accountsync.Scope{Owner: owner, AccountID: account.ID})
	if err != nil {
		return accountsync.MutationRequest{}, err
	}
	byID := make(map[string]accountsync.Folder, len(folders))
	for _, folder := range folders {
		byID[folder.StableID] = folder
	}
	name := candidate.FolderName
	if candidate.Kind != accountsync.MutationFolderDelete && (name == "" || name != strings.TrimSpace(name) || len(name) > 128 || strings.ContainsAny(name, "\x00\r\n")) {
		return accountsync.MutationRequest{}, accounts.ErrInvalid
	}

	displayPath := name
	wireName := imapwire.EncodeMailbox(name)
	switch candidate.Kind {
	case accountsync.MutationFolderCreate:
		candidate.NewFolderPath = wireName
		if candidate.ParentStableID != "" {
			parent, ok := byID[candidate.ParentStableID]
			if !ok || parent.Delimiter == "" || folderHasAttribute(parent, "\\noinferiors") || strings.Contains(name, parent.Delimiter) {
				return accountsync.MutationRequest{}, accounts.ErrInvalid
			}
			displayPath = parent.Name + parent.Delimiter + name
			candidate.NewFolderPath = parent.StableID + parent.Delimiter + wireName
		} else if delimiter := providerFolderDelimiter(folders); delimiter != "" && strings.Contains(name, delimiter) {
			return accountsync.MutationRequest{}, accounts.ErrInvalid
		}
	case accountsync.MutationFolderRename:
		folder, ok := byID[candidate.FolderStableID]
		if !ok || protectedProviderFolder(folder) || folder.Delimiter != "" && strings.Contains(name, folder.Delimiter) {
			return accountsync.MutationRequest{}, accounts.ErrInvalid
		}
		if boundary := strings.LastIndex(folder.Name, folder.Delimiter); folder.Delimiter != "" && boundary >= 0 {
			displayPath = folder.Name[:boundary+len(folder.Delimiter)] + name
		}
		candidate.FolderPath = folder.StableID
		candidate.NewFolderPath = wireName
		if boundary := strings.LastIndex(folder.StableID, folder.Delimiter); folder.Delimiter != "" && boundary >= 0 {
			candidate.NewFolderPath = folder.StableID[:boundary+len(folder.Delimiter)] + wireName
		}
		if displayPath == folder.Name {
			return accountsync.MutationRequest{}, accounts.ErrInvalid
		}
	case accountsync.MutationFolderDelete:
		folder, ok := byID[candidate.FolderStableID]
		if !ok || protectedProviderFolder(folder) {
			return accountsync.MutationRequest{}, accounts.ErrInvalid
		}
		candidate.FolderPath = folder.StableID
		return candidate, nil
	}
	for _, folder := range folders {
		if folder.StableID != candidate.FolderStableID && strings.EqualFold(folder.Name, displayPath) {
			return accountsync.MutationRequest{}, accounts.ErrConflict
		}
	}
	return candidate, nil
}

func protectedProviderFolder(folder accountsync.Folder) bool {
	if folder.Role != "" || strings.EqualFold(folder.Name, "INBOX") {
		return true
	}
	for _, attribute := range folder.Attributes {
		switch strings.ToLower(attribute) {
		case "\\all", "\\archive", "\\drafts", "\\flagged", "\\important", "\\inbox", "\\junk", "\\sent", "\\trash":
			return true
		}
	}
	return false
}

func providerFolderDelimiter(folders []accountsync.Folder) string {
	for _, folder := range folders {
		if folder.Delimiter != "" {
			return folder.Delimiter
		}
	}
	return ""
}

func folderHasAttribute(folder accountsync.Folder, wanted string) bool {
	for _, attribute := range folder.Attributes {
		if strings.EqualFold(attribute, wanted) {
			return true
		}
	}
	return false
}

type tokenRequest struct {
	Token string `json:"token"`
}

type discoverRequest struct {
	Token   string `json:"token"`
	Address string `json:"address"`
}

type idRequest struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

type revisionRequest struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
}

type presentationRequest struct {
	Token    string  `json:"token"`
	ID       string  `json:"id"`
	Revision uint64  `json:"revision"`
	Label    *string `json:"label"`
	Color    *string `json:"color"`
	Order    *uint16 `json:"order"`
}

type credentialInput struct {
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (credentials credentialInput) domainCredentials() accounts.Credentials {
	return accounts.Credentials{
		Username: credentials.Username, Password: credentials.Password,
		AccessToken: credentials.AccessToken, RefreshToken: credentials.RefreshToken,
		ExpiresAt: credentials.ExpiresAt,
	}
}

type createRequest struct {
	Token                string                    `json:"token"`
	Provider             accounts.Provider         `json:"provider"`
	MailboxAddress       string                    `json:"mailboxAddress"`
	AuthIdentity         string                    `json:"authIdentity"`
	SharedMailboxAddress string                    `json:"sharedMailboxAddress"`
	Label                string                    `json:"label"`
	Color                string                    `json:"color"`
	Order                uint16                    `json:"order"`
	ContainerFolder      string                    `json:"containerFolder"`
	Credentials          credentialInput           `json:"credentials"`
	IMAP                 *accounts.Endpoint        `json:"imap"`
	SMTP                 *accounts.Endpoint        `json:"smtp"`
	CardDAV              *accounts.CardDAVEndpoint `json:"carddav"`
	CalDAV               *accounts.CalDAVEndpoint  `json:"caldav"`
}

type oauthBeginRequest struct {
	Token                string             `json:"token"`
	AccountID            accounts.AccountID `json:"accountId"`
	Revision             uint64             `json:"revision"`
	Provider             accounts.Provider  `json:"provider"`
	MailboxAddress       string             `json:"mailboxAddress"`
	AuthIdentity         string             `json:"authIdentity"`
	SharedMailboxAddress string             `json:"sharedMailboxAddress"`
	Label                string             `json:"label"`
	Color                string             `json:"color"`
	Order                uint16             `json:"order"`
}

func (request oauthBeginRequest) domainRequest() accounts.OAuthConnectionRequest {
	return accounts.OAuthConnectionRequest{AccountID: request.AccountID, Revision: request.Revision,
		Provider: request.Provider, MailboxAddress: request.MailboxAddress, AuthIdentity: request.AuthIdentity,
		SharedMailboxAddress: request.SharedMailboxAddress, Label: request.Label, Color: request.Color, Order: request.Order}
}

type oauthCompleteRequest struct {
	Token string `json:"token"`
	State string `json:"state"`
	Code  string `json:"code"`
}

func (request createRequest) domainRequest() accounts.CreateRequest {
	return accounts.CreateRequest{
		Provider: request.Provider, MailboxAddress: request.MailboxAddress,
		AuthIdentity: request.AuthIdentity, SharedMailboxAddress: request.SharedMailboxAddress,
		Label: request.Label, Color: request.Color, Order: request.Order, ContainerFolder: request.ContainerFolder,
		Credentials: request.Credentials.domainCredentials(),
		IMAP:        request.IMAP, SMTP: request.SMTP, CardDAV: request.CardDAV, CalDAV: request.CalDAV,
	}
}

type reconnectRequest struct {
	Token       string          `json:"token"`
	ID          string          `json:"id"`
	Revision    uint64          `json:"revision"`
	Credentials credentialInput `json:"credentials"`
}

type messagesRequest struct {
	Token  string `json:"token"`
	ID     string `json:"id"`
	Folder string `json:"folder"`
	Limit  int    `json:"limit"`
}

type unifiedMessagesRequest struct {
	Token          string `json:"token"`
	AccountID      string `json:"accountId"`
	FolderRole     string `json:"folderRole"`
	FolderStableID string `json:"folderStableId"`
	Query          string `json:"query"`
	Read           *bool  `json:"read"`
	Starred        *bool  `json:"starred"`
	ReceivedAtFrom string `json:"receivedAtFrom"`
	OldestFirst    bool   `json:"oldestFirst"`
	Limit          int    `json:"limit"`
	Offset         int    `json:"offset"`
}

type messageSourceRequest struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	StableID string `json:"stableId"`
}

type mutationRequest struct {
	Token            string                   `json:"token"`
	ID               string                   `json:"id"`
	Revision         uint64                   `json:"revision"`
	IdempotencyKey   string                   `json:"idempotencyKey"`
	Kind             accountsync.MutationKind `json:"kind"`
	MessageStableIDs []string                 `json:"messageStableIds"`
	TargetFolderID   string                   `json:"targetFolderId"`
	FolderStableID   string                   `json:"folderStableId"`
	ParentStableID   string                   `json:"parentStableId"`
	FolderName       string                   `json:"folderName"`
	Value            bool                     `json:"value"`
}

type submissionRequest struct {
	Token           string     `json:"token"`
	ID              string     `json:"id"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	EnvelopeFrom    string     `json:"envelopeFrom"`
	Recipients      []string   `json:"recipients"`
	RawRFC822Base64 string     `json:"rawRFC822Base64"`
	NotBefore       *time.Time `json:"notBefore"`
}

type updateSubmissionRequest struct {
	Token           string     `json:"token"`
	ID              string     `json:"id"`
	SubmissionID    string     `json:"submissionId"`
	Revision        uint64     `json:"revision"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	EnvelopeFrom    string     `json:"envelopeFrom"`
	Recipients      []string   `json:"recipients"`
	RawRFC822Base64 string     `json:"rawRFC822Base64"`
	NotBefore       *time.Time `json:"notBefore"`
}

type submissionBatchBeginRequest struct {
	Token          string     `json:"token"`
	ID             string     `json:"id"`
	IdempotencyKey string     `json:"idempotencyKey"`
	EnvelopeFrom   string     `json:"envelopeFrom"`
	Recipients     []string   `json:"recipients"`
	NotBefore      *time.Time `json:"notBefore"`
}

type submissionBatchUpdateBeginRequest struct {
	Token          string     `json:"token"`
	ID             string     `json:"id"`
	SubmissionID   string     `json:"submissionId"`
	Revision       uint64     `json:"revision"`
	IdempotencyKey string     `json:"idempotencyKey"`
	EnvelopeFrom   string     `json:"envelopeFrom"`
	Recipients     []string   `json:"recipients"`
	NotBefore      *time.Time `json:"notBefore"`
}

type submissionBatchAppendRequest struct {
	Token           string `json:"token"`
	ID              string `json:"id"`
	SubmissionID    string `json:"submissionId"`
	Revision        uint64 `json:"revision"`
	Index           int    `json:"index"`
	Recipient       string `json:"recipient"`
	RawRFC822Base64 string `json:"rawRFC822Base64"`
}

type submissionBatchCommitRequest struct {
	Token        string `json:"token"`
	ID           string `json:"id"`
	SubmissionID string `json:"submissionId"`
	Revision     uint64 `json:"revision"`
}

type cancelSubmissionRequest struct {
	Token        string `json:"token"`
	ID           string `json:"id"`
	SubmissionID string `json:"submissionId"`
}

type contactInput struct {
	Name       string                `json:"name"`
	GivenName  string                `json:"givenName"`
	FamilyName string                `json:"familyName"`
	Company    string                `json:"company"`
	Title      string                `json:"title"`
	Emails     []contactsync.Value   `json:"emails"`
	Phones     []contactsync.Value   `json:"phones"`
	Addresses  []contactsync.Address `json:"addresses"`
	URLs       []contactsync.Value   `json:"urls"`
	Nicknames  []string              `json:"nicknames"`
	Birthday   string                `json:"birthday"`
	Notes      string                `json:"notes"`
	GroupIDs   []string              `json:"groupIds"`
}

func (input contactInput) domainContact() contactsync.Contact {
	return contactsync.Contact{Name: strings.TrimSpace(input.Name), GivenName: strings.TrimSpace(input.GivenName), FamilyName: strings.TrimSpace(input.FamilyName), Company: strings.TrimSpace(input.Company), Title: strings.TrimSpace(input.Title),
		Emails: input.Emails, Phones: input.Phones, Addresses: input.Addresses, URLs: input.URLs, Nicknames: input.Nicknames, Birthday: strings.TrimSpace(input.Birthday), Notes: input.Notes}
}

type contactMutationRequest struct {
	Token          string                   `json:"token"`
	ID             string                   `json:"id"`
	IdempotencyKey string                   `json:"idempotencyKey"`
	Kind           contactsync.MutationKind `json:"kind"`
	AddressBookID  string                   `json:"addressBookId"`
	ContactID      string                   `json:"contactId"`
	ExpectedETag   string                   `json:"expectedEtag"`
	Contact        contactInput             `json:"contact"`
}

func ownedAddressBook(books []contactsync.OwnedAddressBook, accountID accounts.AccountID, bookID string) (contactsync.AddressBook, bool) {
	for _, book := range books {
		if book.AccountID == accountID && book.StableID == bookID {
			return book.AddressBook, true
		}
	}
	return contactsync.AddressBook{}, false
}

func newProviderContactResource(bookURL string) (string, string, error) {
	base, err := url.Parse(bookURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return "", "", errors.New("invalid provider address book URL")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	random[6] = random[6]&0x0f | 0x40
	random[8] = random[8]&0x3f | 0x80
	uid := fmt.Sprintf("%x-%x-%x-%x-%x", random[0:4], random[4:6], random[6:8], random[8:10], random[10:16])
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	filename := "caduceus-" + hex.EncodeToString(random) + ".vcf"
	href := base.ResolveReference(&url.URL{Path: filename}).String()
	return uid, href, nil
}

type contactMutationStatusRequest struct {
	Token      string `json:"token"`
	ID         string `json:"id"`
	MutationID string `json:"mutationId"`
}

type calendarMutationRequest struct {
	Token          string                    `json:"token"`
	ID             string                    `json:"id"`
	IdempotencyKey string                    `json:"idempotencyKey"`
	Kind           calendarsync.MutationKind `json:"kind"`
	CalendarID     string                    `json:"calendarId"`
	ResourceID     string                    `json:"resourceId"`
	ExpectedETag   string                    `json:"expectedEtag"`
	RawICalendar   string                    `json:"rawICalendar"`
}

type calendarMutationStatusRequest struct {
	Token      string `json:"token"`
	ID         string `json:"id"`
	MutationID string `json:"mutationId"`
}

type providerMutationStatusRequest struct {
	Token      string `json:"token"`
	ID         string `json:"id"`
	MutationID string `json:"mutationId"`
}

type notificationClaimRequest struct {
	Holder string `json:"holder"`
	Limit  int    `json:"limit"`
}

type notificationCompleteRequest struct {
	Token     string   `json:"token"`
	IDs       []string `json:"ids"`
	Delivered bool     `json:"delivered"`
	Code      string   `json:"code"`
}

func ownedCalendar(calendars []calendarsync.OwnedCalendar, accountID accounts.AccountID, calendarID string) (calendarsync.Calendar, bool) {
	for _, calendar := range calendars {
		if calendar.AccountID == accountID && calendar.StableID == calendarID {
			return calendar.Calendar, true
		}
	}
	return calendarsync.Calendar{}, false
}

func newProviderCalendarResource(calendarURL string) (string, error) {
	base, err := url.Parse(calendarURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return "", errors.New("invalid provider calendar URL")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	filename := "caduceus-" + hex.EncodeToString(random) + ".ics"
	return base.ResolveReference(&url.URL{Path: filename}).String(), nil
}

type submissionStatusRequest struct {
	Token        string `json:"token"`
	ID           string `json:"id"`
	SubmissionID string `json:"submissionId"`
}

type tokenCarrier interface {
	tokenRequest | discoverRequest | idRequest | revisionRequest | presentationRequest | createRequest | oauthBeginRequest | oauthCompleteRequest | reconnectRequest | messagesRequest | unifiedMessagesRequest | messageSourceRequest | mutationRequest | providerMutationStatusRequest | submissionRequest | submissionBatchBeginRequest | submissionBatchUpdateBeginRequest | submissionBatchAppendRequest | submissionBatchCommitRequest | updateSubmissionRequest | cancelSubmissionRequest | submissionStatusRequest | contactMutationRequest | contactMutationStatusRequest | calendarMutationRequest | calendarMutationStatusRequest
}

func newMutationID() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "mut_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func newSubmissionID() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "sub_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func newContactMutationID() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "cmut_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func newCalendarMutationID() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "calmut_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func decodeForOwner[T tokenCarrier](ctx context.Context, dispatcher *AccountsDispatcher, payload json.RawMessage) (T, accounts.Owner, error) {
	request, err := decodeStrict[T](payload)
	if err != nil {
		return request, accounts.Owner{}, err
	}
	token := tokenFrom(request)
	if strings.TrimSpace(token) == "" {
		return request, accounts.Owner{}, failure("EBADREQUEST", "token is required")
	}
	if dispatcher == nil || dispatcher.authenticator == nil || dispatcher.service == nil {
		return request, accounts.Owner{}, failure("EUNAVAILABLE", "account boundary is not configured")
	}
	principal, err := dispatcher.authenticator.Authenticate(ctx, token)
	if err != nil {
		return request, accounts.Owner{}, failure("EUNAUTHORIZED", "authentication required")
	}
	return request, accounts.Owner{
		TenantID: principal.TenantID, PrincipalID: principal.PrincipalID, Address: principal.Address,
	}, nil
}

func tokenFrom[T tokenCarrier](request T) string {
	encoded, _ := json.Marshal(request)
	var carrier tokenRequest
	_ = json.Unmarshal(encoded, &carrier)
	return carrier.Token
}

func decodeStrict[T any](payload json.RawMessage) (T, error) {
	var result T
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, failure("EBADREQUEST", "invalid account request")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, failure("EBADREQUEST", "account request must contain one JSON object")
	}
	return result, nil
}

type operationFailure struct {
	code    string
	message string
}

func (err operationFailure) Error() string { return err.code + ": " + err.message }

func failure(code, message string) error {
	return operationFailure{code: code, message: message}
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	var validation accounts.ValidationFailure
	if errors.As(err, &validation) {
		return classifyValidation(validation)
	}
	switch {
	case errors.Is(err, accounts.ErrOAuthTransaction):
		return failure("EBADREQUEST", "provider authorization is invalid or expired")
	case errors.Is(err, accounts.ErrOAuthGrant):
		return failure("EAUTHREQUIRED", "provider authorization could not be completed")
	case errors.Is(err, accounts.ErrInvalid):
		return failure("EBADREQUEST", "invalid external account request")
	case errors.Is(err, accounts.ErrNotFound):
		return failure("ENOTFOUND", "external account not found")
	case errors.Is(err, accounts.ErrConflict), errors.Is(err, accounts.ErrInvalidTransition):
		return failure("ECONFLICT", "external account changed; refresh and retry")
	case errors.Is(err, accountsend.ErrInvalidSubmission):
		return failure("EBADREQUEST", "invalid external account submission")
	case errors.Is(err, accountsend.ErrSubmissionConflict):
		return failure("ECONFLICT", "external account submission has already started")
	case errors.Is(err, contactsync.ErrInvalidMutation):
		return failure("EBADREQUEST", "invalid provider contact mutation")
	case errors.Is(err, contactsync.ErrContactNotFound):
		return failure("ENOTFOUND", "provider contact not found")
	case errors.Is(err, contactsync.ErrRemoteConflict):
		return failure("ECONFLICT", "the CardDAV contact changed; refresh and retry")
	case errors.Is(err, calendarsync.ErrInvalidMutation):
		return failure("EBADREQUEST", "invalid provider calendar mutation")
	case errors.Is(err, calendarsync.ErrResourceNotFound):
		return failure("ENOTFOUND", "provider calendar resource not found")
	case errors.Is(err, calendarsync.ErrRemoteConflict):
		return failure("ECONFLICT", "the CalDAV event changed; refresh and retry")
	case errors.Is(err, accounts.ErrUnsupported):
		return failure("EUNSUPPORTED", "external account provider is not configured")
	default:
		return failure("EUNAVAILABLE", "external account operation failed")
	}
}

func classifyValidation(validation accounts.ValidationFailure) error {
	service := strings.ToUpper(string(validation.Service))
	if service != "IMAP" && service != "SMTP" {
		service = "provider"
	}
	switch validation.Code {
	case accounts.ValidationAuthentication:
		return failure("EAUTHREQUIRED", service+" credentials were rejected")
	case accounts.ValidationMailbox:
		return failure("EBADREQUEST", service+" INBOX access is required")
	case accounts.ValidationConnection:
		return failure("EUNAVAILABLE", service+" connection could not be established")
	default:
		return failure("EUNAVAILABLE", service+" protocol validation failed")
	}
}
