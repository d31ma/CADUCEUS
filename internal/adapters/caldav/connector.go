package caldav

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"sort"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

type ProtocolClient interface {
	Discover(context.Context, *accounts.CalDAVEndpoint, accounts.Credentials) (Discovery, error)
	InitialSync(context.Context, Calendar, Auth) ([]RemoteResource, error)
	IncrementalSync(context.Context, Calendar, string, Auth) (Delta, error)
	Put(context.Context, string, string, string, bool, Auth) (string, error)
	Delete(context.Context, string, string, Auth) error
	Fetch(context.Context, string, Auth) (RemoteResource, error)
}

type Connector struct {
	client ProtocolClient
	clock  func() time.Time
}

func NewConnector(client ProtocolClient, clock ...func() time.Time) *Connector {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &Connector{client: client, clock: now}
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation calendarsync.Mutation, current calendarsync.Resource) (calendarsync.MutationOutcome, error) {
	if current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || mutation.ResourceID != current.ID || account.Connection.CalDAV == nil {
		return calendarsync.OutcomeConflict, nil
	}
	if mutation.Kind == calendarsync.MutationCreate {
		if mutation.ExpectedETag != "" || current.CalendarID == "" || current.Href == "" {
			return calendarsync.OutcomeConflict, nil
		}
	} else if mutation.ExpectedETag != current.ETag {
		return calendarsync.OutcomeConflict, nil
	}
	auth := endpointAuth(account.Connection.CalDAV, credentials)
	var err error
	switch mutation.Kind {
	case calendarsync.MutationCreate:
		_, err = connector.client.Put(ctx, current.Href, mutation.Desired.RawICalendar, "", true, auth)
	case calendarsync.MutationUpdate:
		_, err = connector.client.Put(ctx, current.Href, mutation.Desired.RawICalendar, current.ETag, false, auth)
	case calendarsync.MutationDelete:
		err = connector.client.Delete(ctx, current.Href, current.ETag, auth)
	default:
		return "", calendarsync.ErrInvalidMutation
	}
	if err == nil {
		return calendarsync.OutcomeApplied, nil
	}
	if err == ErrPrecondition {
		if mutation.Kind == calendarsync.MutationCreate || mutation.Kind == calendarsync.MutationUpdate {
			remote, fetchErr := connector.client.Fetch(ctx, current.Href, auth)
			if fetchErr == nil && calendarsEquivalent(mutation.Desired.RawICalendar, remote.ICalendar) {
				return calendarsync.OutcomeApplied, nil
			}
			if fetchErr != nil && fetchErr != ErrProtocol {
				return "", classifyConnectorError(fetchErr)
			}
		}
		return calendarsync.OutcomeConflict, nil
	}
	return "", classifyConnectorError(err)
}

func (connector *Connector) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, state calendarsync.State) (calendarsync.Snapshot, error) {
	if connector == nil || connector.client == nil || account.Connection.CalDAV == nil {
		return calendarsync.Snapshot{}, accounts.ErrUnsupported
	}
	discovery, err := connector.client.Discover(ctx, account.Connection.CalDAV, credentials)
	if err != nil {
		return calendarsync.Snapshot{}, classifyConnectorError(err)
	}
	auth := endpointAuth(account.Connection.CalDAV, credentials)
	now := connector.clock().UTC()
	snapshot := calendarsync.Snapshot{PrincipalURL: discovery.PrincipalURL, HomeURL: discovery.HomeURL, SyncedAt: now}
	previousCalendars := make(map[string]calendarsync.Calendar, len(state.Calendars))
	for _, calendar := range state.Calendars {
		previousCalendars[calendar.URL] = calendar
	}
	previousResources := make(map[string][]calendarsync.Resource)
	for _, resource := range state.Resources {
		previousResources[resource.CalendarID] = append(previousResources[resource.CalendarID], resource)
	}
	for _, remoteCalendar := range discovery.Calendars {
		calendarID := stableID("calendar", remoteCalendar.URL)
		calendar := calendarsync.Calendar{StableID: calendarID, URL: remoteCalendar.URL, Name: remoteCalendar.Name, Color: normalizedColor(remoteCalendar.Color), Writable: true, SyncToken: remoteCalendar.SyncToken, CTag: remoteCalendar.CTag}
		previous, canIncrement := previousCalendars[remoteCalendar.URL]
		if canIncrement && previous.SyncToken != "" {
			delta, syncErr := connector.client.IncrementalSync(ctx, remoteCalendar, previous.SyncToken, auth)
			if syncErr == nil {
				calendar.SyncToken = delta.SyncToken
				resources := make(map[string]calendarsync.Resource, len(previousResources[previous.StableID]))
				for _, resource := range previousResources[previous.StableID] {
					resource.CalendarID = calendarID
					resources[resource.Href] = resource
				}
				for _, href := range delta.Deleted {
					delete(resources, href)
				}
				for _, remote := range delta.Changed {
					resources[remote.Href] = projectResource(account.ID, calendarID, remote, now)
				}
				for _, resource := range resources {
					snapshot.Resources = append(snapshot.Resources, resource)
				}
				snapshot.Calendars = append(snapshot.Calendars, calendar)
				continue
			}
			if syncErr != ErrSyncTokenInvalid {
				return calendarsync.Snapshot{}, classifyConnectorError(syncErr)
			}
		}
		remoteResources, syncErr := connector.client.InitialSync(ctx, remoteCalendar, auth)
		if syncErr != nil {
			return calendarsync.Snapshot{}, classifyConnectorError(syncErr)
		}
		for _, remote := range remoteResources {
			snapshot.Resources = append(snapshot.Resources, projectResource(account.ID, calendarID, remote, now))
		}
		snapshot.Calendars = append(snapshot.Calendars, calendar)
	}
	sort.Slice(snapshot.Calendars, func(left, right int) bool {
		return snapshot.Calendars[left].StableID < snapshot.Calendars[right].StableID
	})
	sort.Slice(snapshot.Resources, func(left, right int) bool { return snapshot.Resources[left].ID < snapshot.Resources[right].ID })
	return snapshot, nil
}

func projectResource(accountID accounts.AccountID, calendarID string, remote RemoteResource, now time.Time) calendarsync.Resource {
	return calendarsync.Resource{ID: calendarsync.RemoteResourceID(accountID, remote.Href), AccountID: accountID, CalendarID: calendarID, Href: remote.Href, ETag: remote.ETag, RawICalendar: remote.ICalendar, RemoteUpdatedAt: now}
}

func stableID(prefix, value string) string {
	digest := sha256.Sum256([]byte(value))
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func classifyConnectorError(err error) error {
	switch err {
	case ErrAuthentication:
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "caldav_authentication_failed", Message: "CalDAV authentication failed; reconnect the account."}
	case ErrUnsafeEndpoint:
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "caldav_endpoint_unsafe", Message: "The CalDAV endpoint is not permitted by network policy."}
	default:
		return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "caldav_sync_failed", Message: "CalDAV calendar synchronization failed."}
	}
}

func normalizedColor(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 9 && value[0] == '#' {
		return value[:7]
	}
	if len(value) == 7 && value[0] == '#' {
		return value
	}
	return ""
}

func calendarsEquivalent(left, right string) bool {
	normalize := func(value string) string {
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n"))
	}
	return normalize(left) == normalize(right)
}
