// Package contactsync owns the provider-contact synchronization contract.
// Protocol adapters produce bounded snapshots; FYLO adapters enforce exact
// tenant, principal, and account ownership while committing them.
package contactsync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

const (
	MaxAddressBooks = 100
	MaxContacts     = 5000
	MaxVCardBytes   = 1 << 20
)

var ErrInvalidSnapshot = errors.New("invalid provider contact snapshot")

var (
	ErrInvalidMutation = errors.New("invalid provider contact mutation")
	ErrContactNotFound = errors.New("provider contact not found")
	ErrRemoteConflict  = errors.New("provider contact changed remotely")
)

type MutationKind string
type MutationState string

const (
	MutationCreate MutationKind = "create"
	MutationUpdate MutationKind = "update"
	MutationDelete MutationKind = "delete"

	MutationQueued   MutationState = "queued"
	MutationApplying MutationState = "applying"
	MutationApplied  MutationState = "applied"
	MutationConflict MutationState = "conflict"
	MutationFailed   MutationState = "failed"
)

type Value struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type Address struct {
	StreetAddress   string `json:"streetAddress,omitempty"`
	ExtendedAddress string `json:"extendedAddress,omitempty"`
	City            string `json:"city,omitempty"`
	Region          string `json:"region,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	Country         string `json:"country,omitempty"`
	Type            string `json:"type,omitempty"`
}

type AddressBook struct {
	StableID  string `json:"stableId"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	Writable  bool   `json:"writable"`
	SyncToken string `json:"syncToken,omitempty"`
	CTag      string `json:"ctag,omitempty"`
}

// OwnedAddressBook binds one discovered CardDAV collection to the exact
// external account that owns it. URL and sync cursors remain server-side.
type OwnedAddressBook struct {
	AccountID accounts.AccountID `json:"accountId"`
	AddressBook
}

// Contact is the redacted, normal-contact-compatible projection. Raw vCard
// data is retained for conditional remote updates but never returned by List.
type Contact struct {
	ID              string             `json:"id"`
	AccountID       accounts.AccountID `json:"accountId"`
	AddressBookID   string             `json:"addressBookId"`
	Href            string             `json:"href"`
	ETag            string             `json:"etag"`
	UID             string             `json:"uid,omitempty"`
	Name            string             `json:"name"`
	GivenName       string             `json:"givenName,omitempty"`
	FamilyName      string             `json:"familyName,omitempty"`
	Company         string             `json:"company,omitempty"`
	Title           string             `json:"title,omitempty"`
	Emails          []Value            `json:"emails"`
	Phones          []Value            `json:"phones"`
	Addresses       []Address          `json:"addresses"`
	URLs            []Value            `json:"urls"`
	Nicknames       []string           `json:"nicknames"`
	Birthday        string             `json:"birthday,omitempty"`
	Notes           string             `json:"notes,omitempty"`
	RawVCard        string             `json:"-"`
	RemoteUpdatedAt time.Time          `json:"remoteUpdatedAt"`
}

type Snapshot struct {
	PrincipalURL string
	HomeURL      string
	AddressBooks []AddressBook
	Contacts     []Contact
	SyncedAt     time.Time
}

type State struct {
	PrincipalURL string
	HomeURL      string
	AddressBooks []AddressBook
	// Contacts is the active, unredacted provider snapshot supplied only to
	// the sync connector so RFC 6578 deltas can be applied without a full
	// address-book download. It never crosses the bridge reader boundary.
	Contacts     []Contact
	LastSyncedAt *time.Time
	LastCode     string
}

type MutationRequest struct {
	ID             string       `json:"id"`
	IdempotencyKey string       `json:"idempotencyKey"`
	Kind           MutationKind `json:"kind"`
	ContactID      string       `json:"contactId"`
	ExpectedETag   string       `json:"expectedEtag"`
	Desired        Contact      `json:"desired,omitempty"`
}

type Mutation struct {
	MutationRequest
	Scope     accountsync.Scope `json:"-"`
	State     MutationState     `json:"state"`
	Attempts  uint16            `json:"attempts"`
	LastCode  string            `json:"lastCode,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type MutationOutcome string

const (
	OutcomeApplied  MutationOutcome = "applied"
	OutcomeConflict MutationOutcome = "conflict"
)

type Connector interface {
	Sync(context.Context, accounts.ExternalAccount, accounts.Credentials, State) (Snapshot, error)
	ApplyMutation(context.Context, accounts.ExternalAccount, accounts.Credentials, Mutation, Contact) (MutationOutcome, error)
}

// Enabled reports whether an account has a provider contact transport.
// Password accounts opt in with CardDAV; OAuth providers use their native
// contact APIs only after validation records the Contacts capability.
func Enabled(account accounts.ExternalAccount) bool {
	if account.Connection.CardDAV != nil {
		return true
	}
	if !account.Capabilities.Contacts {
		return false
	}
	return account.Provider == accounts.ProviderGmail || account.Provider == accounts.ProviderMicrosoft
}

type Store interface {
	ContactState(context.Context, accountsync.Scope) (State, error)
	CommitContactSnapshot(context.Context, accountsync.Lease, Snapshot) error
	ListProviderContacts(context.Context, accounts.Owner) ([]Contact, error)
	ProviderContact(context.Context, accountsync.Scope, string) (Contact, error)
	EnqueueContactMutation(context.Context, accountsync.Scope, Mutation) (Mutation, error)
	PendingContactMutations(context.Context, accountsync.Lease, int) ([]Mutation, error)
	UpdateContactMutation(context.Context, accountsync.Lease, Mutation) error
	ContactMutation(context.Context, accountsync.Scope, string) (Mutation, error)
}

type Reader interface {
	ListProviderContacts(context.Context, accounts.Owner) ([]Contact, error)
	ListProviderAddressBooks(context.Context, accounts.Owner) ([]OwnedAddressBook, error)
}

type MutationAccess interface {
	ProviderContact(context.Context, accountsync.Scope, string) (Contact, error)
	EnqueueContactMutation(context.Context, accountsync.Scope, Mutation) (Mutation, error)
	ContactMutation(context.Context, accountsync.Scope, string) (Mutation, error)
}

func ValidateSnapshot(scope accountsync.Scope, snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.PrincipalURL) == "" || strings.TrimSpace(snapshot.HomeURL) == "" || snapshot.SyncedAt.IsZero() ||
		len(snapshot.AddressBooks) > MaxAddressBooks || len(snapshot.Contacts) > MaxContacts {
		return ErrInvalidSnapshot
	}
	books := make(map[string]bool, len(snapshot.AddressBooks))
	for _, book := range snapshot.AddressBooks {
		if invalid(book.StableID, 256) || invalid(book.URL, 2048) || invalid(book.Name, 512) || len(book.SyncToken) > 4096 || len(book.CTag) > 1024 || books[book.StableID] {
			return ErrInvalidSnapshot
		}
		books[book.StableID] = true
	}
	seen := make(map[string]bool, len(snapshot.Contacts))
	for _, contact := range snapshot.Contacts {
		if contact.AccountID != scope.AccountID || invalid(contact.ID, 256) || invalid(contact.AddressBookID, 256) || !books[contact.AddressBookID] ||
			invalid(contact.Href, 2048) || len(contact.ETag) > 1024 || len(contact.RawVCard) == 0 || len(contact.RawVCard) > MaxVCardBytes ||
			contact.RemoteUpdatedAt.IsZero() || seen[contact.ID] || invalidContactDetails(contact) {
			return ErrInvalidSnapshot
		}
		seen[contact.ID] = true
	}
	return nil
}

func ValidateMutation(request MutationRequest) error {
	if !strings.HasPrefix(request.ID, "cmut_") || len(request.ID) < 25 || len(request.ID) > 90 || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 ||
		invalid(request.ContactID, 256) || len(request.ExpectedETag) > 1024 || (request.Kind != MutationCreate && request.Kind != MutationUpdate && request.Kind != MutationDelete) {
		return ErrInvalidMutation
	}
	switch request.Kind {
	case MutationCreate:
		if request.ExpectedETag != "" || invalid(request.Desired.ID, 256) || request.Desired.ID != request.ContactID ||
			invalid(string(request.Desired.AccountID), 256) || invalid(request.Desired.AddressBookID, 256) || invalid(request.Desired.Href, 2048) || invalid(request.Desired.UID, 512) ||
			RemoteContactID(request.Desired.AccountID, request.Desired.Href) != request.ContactID || !validRemoteHref(request.Desired.Href) || invalidDesiredContact(request.Desired) {
			return ErrInvalidMutation
		}
	case MutationUpdate:
		if strings.TrimSpace(request.ExpectedETag) == "" || request.Desired.ID != "" || request.Desired.AccountID != "" || request.Desired.AddressBookID != "" || request.Desired.Href != "" || invalidDesiredContact(request.Desired) {
			return ErrInvalidMutation
		}
	case MutationDelete:
		if strings.TrimSpace(request.ExpectedETag) == "" || request.Desired.ID != "" || request.Desired.Name != "" || request.Desired.AccountID != "" || request.Desired.AddressBookID != "" || request.Desired.Href != "" {
			return ErrInvalidMutation
		}
	}
	return nil
}

func invalidDesiredContact(contact Contact) bool {
	return (strings.TrimSpace(contact.Name) == "" && len(contact.Emails) == 0 && len(contact.Phones) == 0) ||
		invalidContactDetails(contact)
}

func invalidContactDetails(contact Contact) bool {
	if len(contact.Name) > 512 || len(contact.GivenName) > 256 || len(contact.FamilyName) > 256 || len(contact.Company) > 512 || len(contact.Title) > 512 ||
		len(contact.Emails) > 100 || len(contact.Phones) > 100 || len(contact.Addresses) > 100 || len(contact.URLs) > 100 || len(contact.Nicknames) > 100 ||
		len(contact.Birthday) > 64 || len(contact.Notes) > 1<<16 || containsControl(contact.Name, false) || containsControl(contact.GivenName, false) ||
		containsControl(contact.FamilyName, false) || containsControl(contact.Company, false) || containsControl(contact.Title, false) || containsControl(contact.Birthday, false) || containsControl(contact.Notes, true) {
		return true
	}
	for _, values := range [][]Value{contact.Emails, contact.Phones, contact.URLs} {
		for _, value := range values {
			if strings.TrimSpace(value.Value) == "" || len(value.Value) > 2048 || len(value.Type) > 40 || containsControl(value.Value, false) || containsControl(value.Type, false) {
				return true
			}
		}
	}
	for _, address := range contact.Addresses {
		if len(address.StreetAddress) > 512 || len(address.ExtendedAddress) > 512 || len(address.City) > 256 || len(address.Region) > 256 ||
			len(address.PostalCode) > 128 || len(address.Country) > 256 || len(address.Type) > 40 || containsControl(address.StreetAddress, false) ||
			containsControl(address.ExtendedAddress, false) || containsControl(address.City, false) || containsControl(address.Region, false) ||
			containsControl(address.PostalCode, false) || containsControl(address.Country, false) || containsControl(address.Type, false) {
			return true
		}
	}
	for _, nickname := range contact.Nicknames {
		if len(nickname) > 256 || containsControl(nickname, false) {
			return true
		}
	}
	return false
}

func containsControl(value string, allowNewlines bool) bool {
	if strings.ContainsRune(value, '\x00') {
		return true
	}
	return !allowNewlines && strings.ContainsAny(value, "\r\n")
}

func validRemoteHref(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.User == nil && parsed.Hostname() != "" && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Fragment == ""
}

// RemoteContactID is stable across refreshes because CardDAV href is the
// resource identity. Account ID prevents equal hrefs in two accounts from
// colliding in the unified contact projection.
func RemoteContactID(accountID accounts.AccountID, href string) string {
	digest := sha256.Sum256([]byte(string(accountID) + "\x00" + href))
	return "carddav_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func invalid(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}
