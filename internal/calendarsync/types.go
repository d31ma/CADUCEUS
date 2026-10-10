// Package calendarsync owns provider-calendar synchronization contracts.
// Protocol adapters produce bounded snapshots; persistence adapters enforce
// exact tenant, principal, and external-account ownership.
package calendarsync

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
	MaxCalendars          = 100
	MaxResources          = 10_000
	MaxICalendarBytes     = 2 << 20
	MaxSnapshotTotalBytes = 32 << 20
)

var ErrInvalidSnapshot = errors.New("invalid provider calendar snapshot")

var (
	ErrInvalidMutation  = errors.New("invalid provider calendar mutation")
	ErrResourceNotFound = errors.New("provider calendar resource not found")
	ErrRemoteConflict   = errors.New("provider calendar resource changed remotely")
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

type Calendar struct {
	StableID  string `json:"stableId"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	Writable  bool   `json:"writable"`
	SyncToken string `json:"syncToken,omitempty"`
	CTag      string `json:"ctag,omitempty"`
}

// Resource is one remote iCalendar object. RawICalendar is retained for
// server-side RFC 5545 projection and never appears in account metadata.
type Resource struct {
	ID              string             `json:"id"`
	AccountID       accounts.AccountID `json:"accountId"`
	CalendarID      string             `json:"calendarId"`
	Href            string             `json:"href"`
	ETag            string             `json:"etag"`
	RawICalendar    string             `json:"rawICalendar"`
	RemoteUpdatedAt time.Time          `json:"remoteUpdatedAt"`
}

type Snapshot struct {
	PrincipalURL string
	HomeURL      string
	Calendars    []Calendar
	Resources    []Resource
	SyncedAt     time.Time
}

type State struct {
	PrincipalURL string
	HomeURL      string
	Calendars    []Calendar
	Resources    []Resource
	LastSyncedAt *time.Time
}

type OwnedResource struct {
	OwnerAddress  string `json:"ownerAddress"`
	CalendarName  string `json:"calendarName"`
	CalendarColor string `json:"calendarColor"`
	Resource
}

// OwnedCalendar binds a discovered collection to its exact account owner.
// URL and synchronization cursors remain behind the trusted Go boundary.
type OwnedCalendar struct {
	AccountID accounts.AccountID `json:"accountId"`
	Calendar
}

type MutationRequest struct {
	ID             string       `json:"id"`
	IdempotencyKey string       `json:"idempotencyKey"`
	Kind           MutationKind `json:"kind"`
	ResourceID     string       `json:"resourceId"`
	ExpectedETag   string       `json:"expectedEtag"`
	Desired        Resource     `json:"desired,omitempty"`
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
	ApplyMutation(context.Context, accounts.ExternalAccount, accounts.Credentials, Mutation, Resource) (MutationOutcome, error)
}

// Enabled reports whether an account has a provider calendar transport. IMAP
// accounts opt in with CalDAV; OAuth providers expose their native calendar
// APIs when the validated account capability is present.
func Enabled(account accounts.ExternalAccount) bool {
	if account.Connection.CalDAV != nil {
		return true
	}
	if !account.Capabilities.Calendar {
		return false
	}
	return account.Provider == accounts.ProviderGmail || account.Provider == accounts.ProviderMicrosoft
}

type Store interface {
	CalendarState(context.Context, accountsync.Scope) (State, error)
	CommitCalendarSnapshot(context.Context, accountsync.Lease, Snapshot) error
	ProviderCalendarResource(context.Context, accountsync.Scope, string) (Resource, error)
	EnqueueCalendarMutation(context.Context, accountsync.Scope, Mutation) (Mutation, error)
	PendingCalendarMutations(context.Context, accountsync.Lease, int) ([]Mutation, error)
	UpdateCalendarMutation(context.Context, accountsync.Lease, Mutation) error
	CalendarMutation(context.Context, accountsync.Scope, string) (Mutation, error)
}

type Reader interface {
	ListProviderCalendarResources(context.Context, accounts.Owner) ([]OwnedResource, error)
	ListProviderCalendars(context.Context, accounts.Owner) ([]OwnedCalendar, error)
}

type MutationAccess interface {
	ProviderCalendarResource(context.Context, accountsync.Scope, string) (Resource, error)
	EnqueueCalendarMutation(context.Context, accountsync.Scope, Mutation) (Mutation, error)
	CalendarMutation(context.Context, accountsync.Scope, string) (Mutation, error)
}

func ValidateSnapshot(scope accountsync.Scope, snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.PrincipalURL) == "" || strings.TrimSpace(snapshot.HomeURL) == "" || snapshot.SyncedAt.IsZero() ||
		len(snapshot.Calendars) > MaxCalendars || len(snapshot.Resources) > MaxResources {
		return ErrInvalidSnapshot
	}
	calendars := make(map[string]bool, len(snapshot.Calendars))
	for _, calendar := range snapshot.Calendars {
		if invalid(calendar.StableID, 256) || invalid(calendar.URL, 2048) || invalid(calendar.Name, 512) || len(calendar.Color) > 64 ||
			len(calendar.SyncToken) > 4096 || len(calendar.CTag) > 1024 || calendars[calendar.StableID] {
			return ErrInvalidSnapshot
		}
		calendars[calendar.StableID] = true
	}
	totalBytes := 0
	resources := make(map[string]bool, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		totalBytes += len(resource.RawICalendar)
		if resource.AccountID != scope.AccountID || invalid(resource.ID, 256) || invalid(resource.CalendarID, 256) || !calendars[resource.CalendarID] ||
			invalid(resource.Href, 2048) || len(resource.ETag) > 1024 || len(resource.RawICalendar) == 0 || len(resource.RawICalendar) > MaxICalendarBytes ||
			resource.RemoteUpdatedAt.IsZero() || resources[resource.ID] || !looksLikeCalendar(resource.RawICalendar) {
			return ErrInvalidSnapshot
		}
		resources[resource.ID] = true
	}
	if totalBytes > MaxSnapshotTotalBytes {
		return ErrInvalidSnapshot
	}
	return nil
}

func ValidateMutation(request MutationRequest) error {
	if !strings.HasPrefix(request.ID, "calmut_") || len(request.ID) < 26 || len(request.ID) > 90 || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 ||
		invalid(request.ResourceID, 256) || len(request.ExpectedETag) > 1024 || (request.Kind != MutationCreate && request.Kind != MutationUpdate && request.Kind != MutationDelete) {
		return ErrInvalidMutation
	}
	switch request.Kind {
	case MutationCreate:
		if request.ExpectedETag != "" || request.Desired.ID != request.ResourceID || invalid(string(request.Desired.AccountID), 256) || invalid(request.Desired.CalendarID, 256) ||
			invalid(request.Desired.Href, 2048) || RemoteResourceID(request.Desired.AccountID, request.Desired.Href) != request.ResourceID || !validRemoteHref(request.Desired.Href) ||
			!validDesiredCalendar(request.Desired.RawICalendar) {
			return ErrInvalidMutation
		}
	case MutationUpdate:
		if strings.TrimSpace(request.ExpectedETag) == "" || request.Desired.ID != "" || request.Desired.AccountID != "" || request.Desired.CalendarID != "" || request.Desired.Href != "" ||
			!validDesiredCalendar(request.Desired.RawICalendar) {
			return ErrInvalidMutation
		}
	case MutationDelete:
		if strings.TrimSpace(request.ExpectedETag) == "" || request.Desired.ID != "" || request.Desired.AccountID != "" || request.Desired.CalendarID != "" || request.Desired.Href != "" || request.Desired.RawICalendar != "" {
			return ErrInvalidMutation
		}
	}
	return nil
}

func validDesiredCalendar(value string) bool {
	return len(value) > 0 && len(value) <= MaxICalendarBytes && looksLikeCalendar(value)
}

func validRemoteHref(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.User == nil && parsed.Hostname() != "" && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Fragment == ""
}

func RemoteResourceID(accountID accounts.AccountID, href string) string {
	digest := sha256.Sum256([]byte(string(accountID) + "\x00" + href))
	return "caldav_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func invalid(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}

func looksLikeCalendar(value string) bool {
	upper := strings.ToUpper(value)
	return strings.Contains(upper, "BEGIN:VCALENDAR") && strings.Contains(upper, "END:VCALENDAR") && strings.Contains(upper, "BEGIN:VEVENT")
}
