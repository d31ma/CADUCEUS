package carddav

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/contactsync"
)

type Connector struct {
	client ProtocolClient
	clock  func() time.Time
}

type ProtocolClient interface {
	Discover(context.Context, *accounts.CardDAVEndpoint, accounts.Credentials) (Discovery, error)
	InitialSync(context.Context, AddressBook, Auth) ([]RemoteContact, error)
	IncrementalSync(context.Context, AddressBook, string, Auth) (Delta, error)
	Put(context.Context, string, string, string, bool, Auth) (string, error)
	Delete(context.Context, string, string, Auth) error
	Fetch(context.Context, string, Auth) (RemoteContact, error)
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation contactsync.Mutation, current contactsync.Contact) (contactsync.MutationOutcome, error) {
	if current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || mutation.ContactID != current.ID || account.Connection.CardDAV == nil {
		return contactsync.OutcomeConflict, nil
	}
	if mutation.Kind == contactsync.MutationCreate {
		if mutation.ExpectedETag != "" || current.AddressBookID == "" || current.Href == "" || current.UID == "" {
			return contactsync.OutcomeConflict, nil
		}
	} else if mutation.ExpectedETag != current.ETag {
		return contactsync.OutcomeConflict, nil
	}
	auth := Auth{Username: strings.TrimSpace(account.Connection.CardDAV.Username), Password: credentials.Password, AllowInsecureTLS: account.Connection.CardDAV.AllowInsecureTLS}
	if auth.Username == "" {
		auth.Username = strings.TrimSpace(credentials.Username)
	}
	var err error
	switch mutation.Kind {
	case contactsync.MutationCreate:
		vcard, mergeErr := mergeVCard(newVCardSkeleton(current.UID), mutation.Desired)
		if mergeErr != nil {
			return "", accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "carddav_vcard_invalid", Message: "The contact could not be encoded for CardDAV."}
		}
		_, err = connector.client.Put(ctx, current.Href, vcard, "", true, auth)
	case contactsync.MutationUpdate:
		vcard, mergeErr := mergeVCard(current.RawVCard, mutation.Desired)
		if mergeErr != nil {
			return "", accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "carddav_vcard_invalid", Message: "The contact could not be encoded for CardDAV."}
		}
		_, err = connector.client.Put(ctx, current.Href, vcard, current.ETag, false, auth)
	case contactsync.MutationDelete:
		err = connector.client.Delete(ctx, current.Href, current.ETag, auth)
	default:
		return "", contactsync.ErrInvalidMutation
	}
	if err == nil {
		return contactsync.OutcomeApplied, nil
	}
	if err == ErrPrecondition {
		if mutation.Kind == contactsync.MutationCreate || mutation.Kind == contactsync.MutationUpdate {
			remote, fetchErr := connector.client.Fetch(ctx, current.Href, auth)
			if fetchErr == nil && mutationMatchesRemote(mutation.Desired, current, remote.VCard) {
				return contactsync.OutcomeApplied, nil
			}
			if fetchErr != nil && fetchErr != ErrProtocol {
				return "", classifyConnectorError(fetchErr)
			}
		}
		return contactsync.OutcomeConflict, nil
	}
	return "", classifyConnectorError(err)
}

func NewConnector(client ProtocolClient, clock ...func() time.Time) *Connector {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &Connector{client: client, clock: now}
}

func (connector *Connector) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, state contactsync.State) (contactsync.Snapshot, error) {
	if connector == nil || connector.client == nil || account.Connection.CardDAV == nil {
		return contactsync.Snapshot{}, accounts.ErrUnsupported
	}
	discovery, err := connector.client.Discover(ctx, account.Connection.CardDAV, credentials)
	if err != nil {
		return contactsync.Snapshot{}, classifyConnectorError(err)
	}
	auth := Auth{Username: strings.TrimSpace(account.Connection.CardDAV.Username), Password: credentials.Password, AllowInsecureTLS: account.Connection.CardDAV.AllowInsecureTLS}
	if auth.Username == "" {
		auth.Username = strings.TrimSpace(credentials.Username)
	}
	now := connector.clock().UTC()
	snapshot := contactsync.Snapshot{PrincipalURL: discovery.PrincipalURL, HomeURL: discovery.HomeURL, SyncedAt: now}
	previousBooks := make(map[string]contactsync.AddressBook, len(state.AddressBooks))
	for _, book := range state.AddressBooks {
		previousBooks[book.URL] = book
	}
	previousContacts := make(map[string][]contactsync.Contact)
	for _, contact := range state.Contacts {
		previousContacts[contact.AddressBookID] = append(previousContacts[contact.AddressBookID], contact)
	}
	for _, remoteBook := range discovery.AddressBooks {
		bookID := stableID("book", remoteBook.URL)
		book := contactsync.AddressBook{
			StableID: bookID, URL: remoteBook.URL, Name: remoteBook.Name, Writable: true, SyncToken: remoteBook.SyncToken, CTag: remoteBook.CTag,
		}
		previous, canIncrement := previousBooks[remoteBook.URL]
		if canIncrement && previous.SyncToken != "" {
			delta, syncErr := connector.client.IncrementalSync(ctx, remoteBook, previous.SyncToken, auth)
			if syncErr == nil {
				book.SyncToken = delta.SyncToken
				contacts := make(map[string]contactsync.Contact, len(previousContacts[previous.StableID]))
				for _, contact := range previousContacts[previous.StableID] {
					contact.AddressBookID = bookID
					contacts[contact.Href] = contact
				}
				for _, href := range delta.Deleted {
					delete(contacts, href)
				}
				for _, remote := range delta.Changed {
					contact, parseErr := projectRemoteContact(account.ID, bookID, remote, now)
					if parseErr != nil {
						return contactsync.Snapshot{}, parseErr
					}
					contacts[contact.Href] = contact
				}
				for _, contact := range contacts {
					snapshot.Contacts = append(snapshot.Contacts, contact)
				}
				snapshot.AddressBooks = append(snapshot.AddressBooks, book)
				continue
			}
			if syncErr != ErrSyncTokenInvalid {
				return contactsync.Snapshot{}, classifyConnectorError(syncErr)
			}
		}
		remoteContacts, syncErr := connector.client.InitialSync(ctx, remoteBook, auth)
		if syncErr != nil {
			return contactsync.Snapshot{}, classifyConnectorError(syncErr)
		}
		for _, remote := range remoteContacts {
			contact, parseErr := projectRemoteContact(account.ID, bookID, remote, now)
			if parseErr != nil {
				return contactsync.Snapshot{}, parseErr
			}
			snapshot.Contacts = append(snapshot.Contacts, contact)
		}
		snapshot.AddressBooks = append(snapshot.AddressBooks, book)
	}
	sort.Slice(snapshot.AddressBooks, func(left, right int) bool {
		return snapshot.AddressBooks[left].StableID < snapshot.AddressBooks[right].StableID
	})
	sort.Slice(snapshot.Contacts, func(left, right int) bool { return snapshot.Contacts[left].ID < snapshot.Contacts[right].ID })
	return snapshot, nil
}

func projectRemoteContact(accountID accounts.AccountID, bookID string, remote RemoteContact, now time.Time) (contactsync.Contact, error) {
	contact, err := parseVCard(remote.VCard)
	if err != nil {
		return contactsync.Contact{}, fmt.Errorf("%w: malformed vCard", ErrProtocol)
	}
	contact.ID = contactsync.RemoteContactID(accountID, remote.Href)
	contact.AccountID = accountID
	contact.AddressBookID = bookID
	contact.Href = remote.Href
	contact.ETag = remote.ETag
	contact.RawVCard = remote.VCard
	contact.RemoteUpdatedAt = now
	return contact, nil
}

func newVCardSkeleton(uid string) string {
	return "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + escapeVCard(uid) + "\r\nEND:VCARD\r\n"
}

func mutationMatchesRemote(desired, current contactsync.Contact, raw string) bool {
	if desired.UID == "" {
		desired.UID = current.UID
	}
	expectedRaw, err := mergeVCard(newVCardSkeleton(desired.UID), desired)
	if err != nil {
		return false
	}
	expected, err := parseVCard(expectedRaw)
	if err != nil {
		return false
	}
	actual, err := parseVCard(raw)
	if err != nil || expected.UID != actual.UID {
		return false
	}
	return expected.Name == actual.Name && expected.GivenName == actual.GivenName && expected.FamilyName == actual.FamilyName &&
		expected.Company == actual.Company && expected.Title == actual.Title && equalValues(expected.Emails, actual.Emails) &&
		equalValues(expected.Phones, actual.Phones) && equalAddresses(expected.Addresses, actual.Addresses) && equalValues(expected.URLs, actual.URLs) &&
		equalStrings(expected.Nicknames, actual.Nicknames) && expected.Birthday == actual.Birthday && expected.Notes == actual.Notes
}

func equalValues(left, right []contactsync.Value) bool {
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

func equalAddresses(left, right []contactsync.Address) bool {
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

func equalStrings(left, right []string) bool {
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

func stableID(prefix, value string) string {
	digest := sha256.Sum256([]byte(value))
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func classifyConnectorError(err error) error {
	if err == ErrAuthentication {
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "carddav_authentication_failed", Message: "CardDAV authentication failed; reconnect the account."}
	}
	if err == ErrUnsafeEndpoint {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "carddav_endpoint_unsafe", Message: "The CardDAV endpoint is not permitted by network policy."}
	}
	return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "carddav_sync_failed", Message: "CardDAV contact synchronization failed."}
}
