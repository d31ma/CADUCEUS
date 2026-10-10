// Package googlecalendar implements bounded Google Calendar API v3 sync and
// conditional mutations for OAuth Gmail / Workspace accounts.
package googlecalendar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/adapters/calendarapi"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

const defaultBaseURL = "https://www.googleapis.com/calendar/v3/"
const maxResponseBytes = 4 << 20

var (
	ErrAuthentication = errors.New("Google Calendar authentication failed")
	ErrConflict       = errors.New("Google Calendar event changed")
	ErrProtocol       = errors.New("Google Calendar protocol failed")
	ErrSyncToken      = errors.New("Google Calendar sync token expired")
)

type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	Clock      func() time.Time
}

type Connector struct {
	base  *url.URL
	http  *http.Client
	clock func() time.Time
}

type calendarListResponse struct {
	NextPageToken string         `json:"nextPageToken"`
	Items         []calendarItem `json:"items"`
}

type calendarItem struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	BackgroundColor string `json:"backgroundColor"`
	AccessRole      string `json:"accessRole"`
	Deleted         bool   `json:"deleted"`
}

type eventListResponse struct {
	NextPageToken string        `json:"nextPageToken"`
	NextSyncToken string        `json:"nextSyncToken"`
	Items         []googleEvent `json:"items"`
}

type eventDate struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type eventPerson struct {
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Response    string `json:"responseStatus,omitempty"`
}

type eventReminder struct {
	Method  string `json:"method"`
	Minutes int    `json:"minutes"`
}

type googleEvent struct {
	ID                string        `json:"id,omitempty"`
	ETag              string        `json:"etag,omitempty"`
	Status            string        `json:"status,omitempty"`
	ICalUID           string        `json:"iCalUID,omitempty"`
	Summary           string        `json:"summary,omitempty"`
	Description       string        `json:"description,omitempty"`
	Location          string        `json:"location,omitempty"`
	Start             eventDate     `json:"start"`
	End               eventDate     `json:"end"`
	OriginalStartTime *eventDate    `json:"originalStartTime,omitempty"`
	RecurringEventID  string        `json:"recurringEventId,omitempty"`
	Recurrence        []string      `json:"recurrence,omitempty"`
	Organizer         eventPerson   `json:"organizer,omitempty"`
	Attendees         []eventPerson `json:"attendees,omitempty"`
	Reminders         struct {
		UseDefault bool            `json:"useDefault"`
		Overrides  []eventReminder `json:"overrides,omitempty"`
	} `json:"reminders,omitempty"`
}

func New(options Options) (*Connector, error) {
	if strings.TrimSpace(options.BaseURL) == "" {
		options.BaseURL = defaultBaseURL
	}
	base, err := url.Parse(options.BaseURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || base.RawQuery != "" || base.Fragment != "" || !secureOrLoopback(base) {
		return nil, errors.New("Google Calendar API base URL is invalid")
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	return &Connector{base: base, http: options.HTTPClient, clock: options.Clock}, nil
}

func (connector *Connector) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, state calendarsync.State) (calendarsync.Snapshot, error) {
	if connector == nil || connector.http == nil || account.Provider != accounts.ProviderGmail || strings.TrimSpace(credentials.AccessToken) == "" {
		return calendarsync.Snapshot{}, accounts.ErrUnsupported
	}
	remoteCalendars, err := connector.calendars(ctx, credentials.AccessToken)
	if err != nil {
		return calendarsync.Snapshot{}, classify(err)
	}
	now := connector.clock().UTC()
	snapshot := calendarsync.Snapshot{PrincipalURL: connector.endpoint("users/me/calendarList"), HomeURL: connector.base.String(), SyncedAt: now}
	previousCalendars := map[string]calendarsync.Calendar{}
	previousResources := map[string][]calendarsync.Resource{}
	for _, calendar := range state.Calendars {
		previousCalendars[calendar.URL] = calendar
	}
	for _, resource := range state.Resources {
		previousResources[resource.CalendarID] = append(previousResources[resource.CalendarID], resource)
	}
	for _, remote := range remoteCalendars {
		calendarURL := connector.endpoint("calendars/" + url.PathEscape(remote.ID) + "/events")
		calendarID := stableID(calendarURL)
		calendar := calendarsync.Calendar{StableID: calendarID, URL: calendarURL, Name: fallback(remote.Summary, remote.ID), Color: normalizedColor(remote.BackgroundColor), Writable: remote.AccessRole == "writer" || remote.AccessRole == "owner"}
		previous, found := previousCalendars[calendarURL]
		resources := map[string]calendarsync.Resource{}
		if found && previous.SyncToken != "" {
			for _, resource := range previousResources[previous.StableID] {
				resource.CalendarID = calendarID
				resources[resource.Href] = resource
			}
			changes, token, syncErr := connector.events(ctx, calendarURL, previous.SyncToken, credentials.AccessToken)
			if syncErr == nil {
				calendar.SyncToken = token
				series := seriesEvents(calendarURL, resources, changes)
				for _, event := range changes {
					href := eventURL(calendarURL, event.ID)
					if event.Status == "cancelled" && event.RecurringEventID == "" {
						delete(resources, href)
						continue
					}
					resource, projectErr := project(account.ID, calendarID, calendarURL, event, now, series)
					if projectErr != nil {
						return calendarsync.Snapshot{}, classify(projectErr)
					}
					resources[href] = resource
				}
			} else if !errors.Is(syncErr, ErrSyncToken) {
				return calendarsync.Snapshot{}, classify(syncErr)
			} else {
				resources = map[string]calendarsync.Resource{}
			}
		}
		if calendar.SyncToken == "" {
			events, token, syncErr := connector.events(ctx, calendarURL, "", credentials.AccessToken)
			if syncErr != nil {
				return calendarsync.Snapshot{}, classify(syncErr)
			}
			calendar.SyncToken = token
			series := seriesEvents(calendarURL, nil, events)
			for _, event := range events {
				if event.Status == "cancelled" && event.RecurringEventID == "" {
					continue
				}
				resource, projectErr := project(account.ID, calendarID, calendarURL, event, now, series)
				if projectErr != nil {
					return calendarsync.Snapshot{}, classify(projectErr)
				}
				resources[resource.Href] = resource
			}
		}
		snapshot.Calendars = append(snapshot.Calendars, calendar)
		for _, resource := range resources {
			snapshot.Resources = append(snapshot.Resources, resource)
		}
		if len(snapshot.Resources) > calendarsync.MaxResources {
			return calendarsync.Snapshot{}, calendarsync.ErrInvalidSnapshot
		}
	}
	sort.Slice(snapshot.Calendars, func(i, j int) bool { return snapshot.Calendars[i].StableID < snapshot.Calendars[j].StableID })
	sort.Slice(snapshot.Resources, func(i, j int) bool { return snapshot.Resources[i].ID < snapshot.Resources[j].ID })
	return snapshot, nil
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation calendarsync.Mutation, current calendarsync.Resource) (calendarsync.MutationOutcome, error) {
	if account.Provider != accounts.ProviderGmail || current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || current.ID != mutation.ResourceID || strings.TrimSpace(credentials.AccessToken) == "" {
		return calendarsync.OutcomeConflict, nil
	}
	desired, err := calendarapi.Parse(mutation.Desired.RawICalendar)
	if mutation.Kind != calendarsync.MutationDelete && err != nil {
		return "", calendarsync.ErrInvalidMutation
	}
	if mutation.Kind != calendarsync.MutationCreate && mutation.ExpectedETag != current.ETag {
		return calendarsync.OutcomeConflict, nil
	}
	switch mutation.Kind {
	case calendarsync.MutationCreate:
		collection := parentURL(current.Href)
		payload := fromEvent(desired)
		payload.ID = eventID(mutation.IdempotencyKey)
		body, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return "", encodeErr
		}
		_, status, callErr := connector.call(ctx, http.MethodPost, collection, credentials.AccessToken, "", body)
		if callErr == nil && status == http.StatusCreated {
			return calendarsync.OutcomeApplied, nil
		}
		if status == http.StatusConflict {
			remote, fetchErr := connector.fetch(ctx, eventURL(collection, payload.ID), credentials.AccessToken)
			if fetchErr == nil && equivalent(desired, remote) {
				return calendarsync.OutcomeApplied, nil
			}
			return calendarsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	case calendarsync.MutationUpdate:
		body, encodeErr := json.Marshal(fromEvent(desired))
		if encodeErr != nil {
			return "", encodeErr
		}
		_, status, callErr := connector.call(ctx, http.MethodPut, current.Href, credentials.AccessToken, current.ETag, body)
		if callErr == nil && status == http.StatusOK {
			return calendarsync.OutcomeApplied, nil
		}
		if status == http.StatusPreconditionFailed {
			remote, fetchErr := connector.fetch(ctx, current.Href, credentials.AccessToken)
			if fetchErr == nil && equivalent(desired, remote) {
				return calendarsync.OutcomeApplied, nil
			}
			return calendarsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	case calendarsync.MutationDelete:
		_, status, callErr := connector.call(ctx, http.MethodDelete, current.Href, credentials.AccessToken, current.ETag, nil)
		if callErr == nil && status == http.StatusNoContent || status == http.StatusNotFound && mutation.State == calendarsync.MutationApplying {
			return calendarsync.OutcomeApplied, nil
		}
		if status == http.StatusPreconditionFailed {
			return calendarsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	default:
		return "", calendarsync.ErrInvalidMutation
	}
}

func (connector *Connector) calendars(ctx context.Context, token string) ([]calendarItem, error) {
	items := []calendarItem{}
	pageToken := ""
	for page := 0; page < 100; page++ {
		target, _ := url.Parse(connector.endpoint("users/me/calendarList"))
		query := target.Query()
		query.Set("maxResults", "250")
		query.Set("showDeleted", "false")
		query.Set("showHidden", "true")
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		target.RawQuery = query.Encode()
		body, _, err := connector.call(ctx, http.MethodGet, target.String(), token, "", nil)
		if err != nil {
			return nil, err
		}
		var response calendarListResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, ErrProtocol
		}
		for _, item := range response.Items {
			if item.Deleted || item.ID == "" {
				continue
			}
			if len(items) >= calendarsync.MaxCalendars {
				return nil, calendarsync.ErrInvalidSnapshot
			}
			items = append(items, item)
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			return items, nil
		}
	}
	return nil, ErrProtocol
}

func (connector *Connector) events(ctx context.Context, calendarURL, syncToken, token string) ([]googleEvent, string, error) {
	items := []googleEvent{}
	pageToken := ""
	for page := 0; page < 100; page++ {
		target, err := connector.safeURL(calendarURL)
		if err != nil {
			return nil, "", err
		}
		query := target.Query()
		query.Set("maxResults", "2500")
		query.Set("showDeleted", "true")
		query.Set("singleEvents", "false")
		if syncToken != "" {
			query.Set("syncToken", syncToken)
		}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		target.RawQuery = query.Encode()
		body, status, callErr := connector.call(ctx, http.MethodGet, target.String(), token, "", nil)
		if status == http.StatusGone {
			return nil, "", ErrSyncToken
		}
		if callErr != nil {
			return nil, "", callErr
		}
		var response eventListResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, "", ErrProtocol
		}
		items = append(items, response.Items...)
		if len(items) > calendarsync.MaxResources {
			return nil, "", calendarsync.ErrInvalidSnapshot
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			if strings.TrimSpace(response.NextSyncToken) == "" {
				return nil, "", ErrProtocol
			}
			return items, response.NextSyncToken, nil
		}
	}
	return nil, "", ErrProtocol
}

func (connector *Connector) fetch(ctx context.Context, target, token string) (googleEvent, error) {
	body, _, err := connector.call(ctx, http.MethodGet, target, token, "", nil)
	if err != nil {
		return googleEvent{}, err
	}
	var event googleEvent
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		return googleEvent{}, ErrProtocol
	}
	return event, nil
}

func (connector *Connector) call(ctx context.Context, method, target, token, etag string, payload []byte) ([]byte, int, error) {
	parsed, err := connector.safeURL(target)
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, 0, ErrProtocol
	}
	request, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, 0, ErrProtocol
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if len(payload) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if etag != "" {
		request.Header.Set("If-Match", etag)
	}
	response, err := connector.http.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if readErr != nil || len(body) > maxResponseBytes {
		return nil, response.StatusCode, ErrProtocol
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return body, response.StatusCode, ErrAuthentication
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return body, response.StatusCode, ErrProtocol
	}
	return body, response.StatusCode, nil
}

func (connector *Connector) safeURL(target string) (*url.URL, error) {
	parsed, err := url.Parse(target)
	if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != connector.base.Scheme || !strings.EqualFold(parsed.Host, connector.base.Host) || !strings.HasPrefix(parsed.EscapedPath(), connector.base.EscapedPath()) {
		return nil, ErrProtocol
	}
	return parsed, nil
}

func (connector *Connector) endpoint(relative string) string {
	return connector.base.ResolveReference(&url.URL{Path: strings.TrimPrefix(relative, "/")}).String()
}

func project(accountID accounts.AccountID, calendarID, calendarURL string, remote googleEvent, now time.Time, series map[string]calendarapi.Event) (calendarsync.Resource, error) {
	event, err := toEvent(remote)
	if err != nil && remote.Status == "cancelled" && remote.RecurringEventID != "" && remote.OriginalStartTime != nil {
		event, err = cancelledInstance(remote, series[remote.RecurringEventID])
	}
	if err != nil {
		return calendarsync.Resource{}, err
	}
	raw, err := calendarapi.Render(event, now)
	if err != nil {
		return calendarsync.Resource{}, err
	}
	href := eventURL(calendarURL, remote.ID)
	return calendarsync.Resource{ID: calendarsync.RemoteResourceID(accountID, href), AccountID: accountID, CalendarID: calendarID, Href: href, ETag: remote.ETag, RawICalendar: raw, RemoteUpdatedAt: now}, nil
}

func seriesEvents(calendarURL string, resources map[string]calendarsync.Resource, remote []googleEvent) map[string]calendarapi.Event {
	series := map[string]calendarapi.Event{}
	for href, resource := range resources {
		event, err := calendarapi.Parse(resource.RawICalendar)
		if err == nil && event.RecurrenceID == nil {
			series[remoteEventID(calendarURL, href)] = event
		}
	}
	for _, candidate := range remote {
		if candidate.RecurringEventID != "" || candidate.Status == "cancelled" {
			continue
		}
		if event, err := toEvent(candidate); err == nil {
			series[candidate.ID] = event
		}
	}
	return series
}

func remoteEventID(calendarURL, href string) string {
	prefix := strings.TrimSuffix(calendarURL, "/") + "/"
	if !strings.HasPrefix(href, prefix) {
		return ""
	}
	value, err := url.PathUnescape(strings.TrimPrefix(href, prefix))
	if err != nil || strings.Contains(value, "/") {
		return ""
	}
	return value
}

func cancelledInstance(remote googleEvent, master calendarapi.Event) (calendarapi.Event, error) {
	recurrenceID, err := toDate(*remote.OriginalStartTime)
	if err != nil {
		return calendarapi.Event{}, err
	}
	if master.UID == "" {
		master.UID = fallback(remote.ICalUID, remote.RecurringEventID)
		master.Start = recurrenceID
		if recurrenceID.AllDay {
			master.End, _ = calendarapi.NewDate(addCivilDays(recurrenceID.Date, 1))
		} else {
			master.End = calendarapi.DateTime{Instant: recurrenceID.Instant.Add(time.Hour), TimeZone: recurrenceID.TimeZone}
		}
	} else if recurrenceID.AllDay && master.Start.AllDay {
		start, _ := time.Parse("2006-01-02", master.Start.Date)
		end, _ := time.Parse("2006-01-02", master.End.Date)
		master.Start = recurrenceID
		master.End, _ = calendarapi.NewDate(addCivilDays(recurrenceID.Date, int(end.Sub(start)/(24*time.Hour))))
	} else if !recurrenceID.AllDay && !master.Start.AllDay {
		duration := master.End.Instant.Sub(master.Start.Instant)
		master.Start = recurrenceID
		master.End = calendarapi.DateTime{Instant: recurrenceID.Instant.Add(duration), TimeZone: recurrenceID.TimeZone}
	} else {
		return calendarapi.Event{}, calendarapi.ErrInvalidCalendar
	}
	master.Status = "cancelled"
	master.Recurrence = nil
	master.RecurrenceID = &recurrenceID
	return master, calendarapi.Validate(master)
}

func addCivilDays(value string, days int) string {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || days < 1 {
		days = 1
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func toEvent(remote googleEvent) (calendarapi.Event, error) {
	start, err := toDate(remote.Start)
	if err != nil {
		return calendarapi.Event{}, err
	}
	end, err := toDate(remote.End)
	if err != nil {
		return calendarapi.Event{}, err
	}
	event := calendarapi.Event{UID: fallback(remote.ICalUID, remote.ID), Summary: remote.Summary, Description: remote.Description, Location: remote.Location, Status: remote.Status, Organizer: strings.ToLower(remote.Organizer.Email), Start: start, End: end, Recurrence: append([]string(nil), remote.Recurrence...)}
	if remote.OriginalStartTime != nil && remote.RecurringEventID != "" {
		recurrenceID, dateErr := toDate(*remote.OriginalStartTime)
		if dateErr != nil {
			return calendarapi.Event{}, dateErr
		}
		event.RecurrenceID = &recurrenceID
	}
	for _, attendee := range remote.Attendees {
		if attendee.Email != "" {
			event.Attendees = append(event.Attendees, calendarapi.Attendee{Email: strings.ToLower(attendee.Email), Name: attendee.DisplayName, Status: attendee.Response})
		}
	}
	for _, reminder := range remote.Reminders.Overrides {
		if reminder.Method == "popup" && reminder.Minutes >= 0 && reminder.Minutes <= 40320 {
			minutes := reminder.Minutes
			event.ReminderMinutes = &minutes
			break
		}
	}
	return event, calendarapi.Validate(event)
}

func fromEvent(event calendarapi.Event) googleEvent {
	result := googleEvent{Summary: event.Summary, Description: event.Description, Location: event.Location, Status: event.Status, Recurrence: append([]string(nil), event.Recurrence...)}
	result.Start = fromDate(event.Start)
	result.End = fromDate(event.End)
	for _, attendee := range event.Attendees {
		result.Attendees = append(result.Attendees, eventPerson{Email: attendee.Email, DisplayName: attendee.Name, Response: googleStatus(attendee.Status)})
	}
	if event.ReminderMinutes != nil {
		result.Reminders.Overrides = []eventReminder{{Method: "popup", Minutes: *event.ReminderMinutes}}
	}
	return result
}

func toDate(value eventDate) (calendarapi.DateTime, error) {
	if value.Date != "" {
		return calendarapi.NewDate(value.Date)
	}
	return calendarapi.NewInstant(value.DateTime, value.TimeZone)
}

func fromDate(value calendarapi.DateTime) eventDate {
	if value.AllDay {
		return eventDate{Date: value.Date}
	}
	return eventDate{DateTime: value.Instant.Format(time.RFC3339), TimeZone: value.TimeZone}
}

func equivalent(desired calendarapi.Event, remote googleEvent) bool {
	actual, err := toEvent(remote)
	if err != nil {
		return false
	}
	return desired.Summary == actual.Summary && desired.Description == actual.Description && desired.Location == actual.Location &&
		calendarapi.DateString(desired.Start) == calendarapi.DateString(actual.Start) && calendarapi.DateString(desired.End) == calendarapi.DateString(actual.End) &&
		strings.Join(calendarapi.SortedRecurrence(desired.Recurrence), "\x00") == strings.Join(calendarapi.SortedRecurrence(actual.Recurrence), "\x00")
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAuthentication) {
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "google_calendar_authentication_failed", Message: "Google Calendar authorization expired; reconnect the account."}
	}
	if errors.Is(err, calendarapi.ErrInvalidCalendar) || errors.Is(err, calendarsync.ErrInvalidSnapshot) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "google_calendar_invalid_data", Message: "Google Calendar returned an invalid event snapshot."}
	}
	return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "google_calendar_sync_failed", Message: "Google Calendar synchronization failed."}
}

func stableID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "calendar_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func eventID(key string) string {
	digest := sha256.Sum256([]byte(key))
	return strings.ToLower(base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:20]))
}

func eventURL(calendarURL, id string) string {
	return strings.TrimSuffix(calendarURL, "/") + "/" + url.PathEscape(id)
}

func parentURL(target string) string {
	if parsed, err := url.Parse(target); err == nil {
		path := strings.TrimSuffix(parsed.Path, "/")
		if slash := strings.LastIndex(path, "/"); slash >= 0 {
			parsed.Path, parsed.RawPath, parsed.RawQuery = path[:slash], "", ""
			return parsed.String()
		}
	}
	return ""
}

func secureOrLoopback(parsed *url.URL) bool {
	if parsed.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return parsed.Scheme == "http" && ip != nil && ip.IsLoopback()
}

func normalizedColor(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 7 && value[0] == '#' {
		if _, err := strconv.ParseUint(value[1:], 16, 32); err == nil {
			return strings.ToUpper(value)
		}
	}
	return ""
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func googleStatus(value string) string {
	switch strings.ToLower(value) {
	case "accepted", "declined", "tentative":
		return strings.ToLower(value)
	default:
		return "needsAction"
	}
}

func (connector *Connector) String() string {
	return fmt.Sprintf("Google Calendar API (%s)", connector.base)
}
