// Package microsoftcalendar implements bounded Microsoft Graph v1.0 calendar
// synchronization and conditional event mutations for delegated OAuth users
// and shared mailboxes.
package microsoftcalendar

import (
	"bytes"
	"context"
	"crypto/sha256"
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

const defaultBaseURL = "https://graph.microsoft.com/v1.0/"
const maxResponseBytes = 4 << 20

var (
	ErrAuthentication = errors.New("Microsoft Graph calendar authentication failed")
	ErrProtocol       = errors.New("Microsoft Graph calendar protocol failed")
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
	Next  string          `json:"@odata.nextLink"`
	Value []graphCalendar `json:"value"`
}

type graphCalendar struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	CanEdit bool   `json:"canEdit"`
}

type eventListResponse struct {
	Next  string       `json:"@odata.nextLink"`
	Value []graphEvent `json:"value"`
}

type graphDate struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type graphEmail struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type graphPerson struct {
	EmailAddress graphEmail `json:"emailAddress"`
}

type graphAttendee struct {
	EmailAddress graphEmail `json:"emailAddress"`
	Type         string     `json:"type,omitempty"`
	Status       struct {
		Response string `json:"response"`
	} `json:"status,omitempty"`
}

type recurrencePattern struct {
	Type           string   `json:"type"`
	Interval       int      `json:"interval"`
	Month          int      `json:"month,omitempty"`
	DayOfMonth     int      `json:"dayOfMonth,omitempty"`
	DaysOfWeek     []string `json:"daysOfWeek,omitempty"`
	FirstDayOfWeek string   `json:"firstDayOfWeek,omitempty"`
	Index          string   `json:"index,omitempty"`
}

type recurrenceRange struct {
	Type                string `json:"type"`
	StartDate           string `json:"startDate"`
	EndDate             string `json:"endDate,omitempty"`
	NumberOfOccurrences int    `json:"numberOfOccurrences,omitempty"`
	RecurrenceTimeZone  string `json:"recurrenceTimeZone,omitempty"`
}

type graphRecurrence struct {
	Pattern recurrencePattern `json:"pattern"`
	Range   recurrenceRange   `json:"range"`
}

type graphEvent struct {
	ID              string           `json:"id,omitempty"`
	ETag            string           `json:"@odata.etag,omitempty"`
	ICalUID         string           `json:"iCalUId,omitempty"`
	Subject         string           `json:"subject,omitempty"`
	Body            graphBody        `json:"body,omitempty"`
	BodyPreview     string           `json:"bodyPreview,omitempty"`
	Location        graphLocation    `json:"location,omitempty"`
	Start           graphDate        `json:"start"`
	End             graphDate        `json:"end"`
	IsAllDay        bool             `json:"isAllDay,omitempty"`
	IsCancelled     bool             `json:"isCancelled,omitempty"`
	SeriesMasterID  string           `json:"seriesMasterId,omitempty"`
	OriginalStart   string           `json:"originalStart,omitempty"`
	Type            string           `json:"type,omitempty"`
	Organizer       graphPerson      `json:"organizer,omitempty"`
	Attendees       []graphAttendee  `json:"attendees,omitempty"`
	Recurrence      *graphRecurrence `json:"recurrence,omitempty"`
	IsReminderOn    bool             `json:"isReminderOn,omitempty"`
	ReminderMinutes int              `json:"reminderMinutesBeforeStart,omitempty"`
	TransactionID   string           `json:"transactionId,omitempty"`
}

type graphBody struct {
	ContentType string `json:"contentType,omitempty"`
	Content     string `json:"content,omitempty"`
}

type graphLocation struct {
	DisplayName string `json:"displayName,omitempty"`
}

func New(options Options) (*Connector, error) {
	if strings.TrimSpace(options.BaseURL) == "" {
		options.BaseURL = defaultBaseURL
	}
	base, err := url.Parse(options.BaseURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || base.RawQuery != "" || base.Fragment != "" || !secureOrLoopback(base) {
		return nil, errors.New("Microsoft Graph API base URL is invalid")
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

func (connector *Connector) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, _ calendarsync.State) (calendarsync.Snapshot, error) {
	if connector == nil || connector.http == nil || account.Provider != accounts.ProviderMicrosoft || strings.TrimSpace(credentials.AccessToken) == "" {
		return calendarsync.Snapshot{}, accounts.ErrUnsupported
	}
	root := connector.mailboxRoot(account.MailboxAddress)
	calendars, err := connector.listCalendars(ctx, root+"/calendars", credentials.AccessToken)
	if err != nil {
		return calendarsync.Snapshot{}, classify(err)
	}
	now := connector.clock().UTC()
	snapshot := calendarsync.Snapshot{PrincipalURL: root, HomeURL: root + "/calendars", SyncedAt: now}
	for _, remote := range calendars {
		calendarURL := root + "/calendars/" + url.PathEscape(remote.ID) + "/events"
		calendarID := stableID(calendarURL)
		calendar := calendarsync.Calendar{StableID: calendarID, URL: calendarURL, Name: fallback(remote.Name, "Calendar"), Color: graphColor(remote.Color), Writable: remote.CanEdit, CTag: "full-v1"}
		events, listErr := connector.listEvents(ctx, calendarURL, credentials.AccessToken)
		if listErr != nil {
			return calendarsync.Snapshot{}, classify(listErr)
		}
		snapshot.Calendars = append(snapshot.Calendars, calendar)
		for _, event := range events {
			if event.IsCancelled && event.SeriesMasterID == "" {
				continue
			}
			resource, projectErr := project(account.ID, calendarID, calendarURL, event, now)
			if projectErr != nil {
				return calendarsync.Snapshot{}, classify(projectErr)
			}
			snapshot.Resources = append(snapshot.Resources, resource)
			if len(snapshot.Resources) > calendarsync.MaxResources {
				return calendarsync.Snapshot{}, calendarsync.ErrInvalidSnapshot
			}
		}
	}
	sort.Slice(snapshot.Calendars, func(i, j int) bool { return snapshot.Calendars[i].StableID < snapshot.Calendars[j].StableID })
	sort.Slice(snapshot.Resources, func(i, j int) bool { return snapshot.Resources[i].ID < snapshot.Resources[j].ID })
	return snapshot, nil
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation calendarsync.Mutation, current calendarsync.Resource) (calendarsync.MutationOutcome, error) {
	if account.Provider != accounts.ProviderMicrosoft || current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || current.ID != mutation.ResourceID || strings.TrimSpace(credentials.AccessToken) == "" {
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
		payload, convertErr := fromEvent(desired)
		if convertErr != nil {
			return "", convertErr
		}
		payload.TransactionID = transactionID(mutation.IdempotencyKey)
		body, _ := json.Marshal(payload)
		_, status, callErr := connector.call(ctx, http.MethodPost, parentURL(current.Href), credentials.AccessToken, "", body)
		if callErr == nil && status == http.StatusCreated {
			return calendarsync.OutcomeApplied, nil
		}
		if status == http.StatusConflict {
			return calendarsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	case calendarsync.MutationUpdate:
		payload, convertErr := fromEvent(desired)
		if convertErr != nil {
			return "", convertErr
		}
		body, _ := json.Marshal(payload)
		_, status, callErr := connector.call(ctx, http.MethodPatch, current.Href, credentials.AccessToken, current.ETag, body)
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

func (connector *Connector) listCalendars(ctx context.Context, target, token string) ([]graphCalendar, error) {
	items := []graphCalendar{}
	next := target + "?$top=100"
	for page := 0; page < 100 && next != ""; page++ {
		body, _, err := connector.call(ctx, http.MethodGet, next, token, "", nil)
		if err != nil {
			return nil, err
		}
		var response calendarListResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, ErrProtocol
		}
		for _, item := range response.Value {
			if strings.TrimSpace(item.ID) != "" {
				items = append(items, item)
			}
		}
		if len(items) > calendarsync.MaxCalendars {
			return nil, calendarsync.ErrInvalidSnapshot
		}
		next = response.Next
	}
	if next != "" {
		return nil, ErrProtocol
	}
	return items, nil
}

func (connector *Connector) listEvents(ctx context.Context, target, token string) ([]graphEvent, error) {
	items := []graphEvent{}
	next := target + "?$top=100"
	for page := 0; page < 200 && next != ""; page++ {
		body, _, err := connector.call(ctx, http.MethodGet, next, token, "", nil)
		if err != nil {
			return nil, err
		}
		var response eventListResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, ErrProtocol
		}
		items = append(items, response.Value...)
		if len(items) > calendarsync.MaxResources {
			return nil, calendarsync.ErrInvalidSnapshot
		}
		next = response.Next
	}
	if next != "" {
		return nil, ErrProtocol
	}
	return items, nil
}

func (connector *Connector) fetch(ctx context.Context, target, token string) (graphEvent, error) {
	body, _, err := connector.call(ctx, http.MethodGet, target, token, "", nil)
	if err != nil {
		return graphEvent{}, err
	}
	var event graphEvent
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		return graphEvent{}, ErrProtocol
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
	request.Header.Set("Prefer", `outlook.timezone="UTC"`)
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

func (connector *Connector) mailboxRoot(mailbox string) string {
	return strings.TrimSuffix(connector.base.String(), "/") + "/users/" + url.PathEscape(strings.ToLower(strings.TrimSpace(mailbox)))
}

func project(accountID accounts.AccountID, calendarID, calendarURL string, remote graphEvent, now time.Time) (calendarsync.Resource, error) {
	event, err := toEvent(remote)
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

func toEvent(remote graphEvent) (calendarapi.Event, error) {
	start, err := graphDateValue(remote.Start, remote.IsAllDay)
	if err != nil {
		return calendarapi.Event{}, err
	}
	end, err := graphDateValue(remote.End, remote.IsAllDay)
	if err != nil {
		return calendarapi.Event{}, err
	}
	description := remote.Body.Content
	if strings.EqualFold(remote.Body.ContentType, "html") || description == "" {
		description = remote.BodyPreview
	}
	event := calendarapi.Event{UID: fallback(remote.ICalUID, remote.ID), Summary: remote.Subject, Description: description, Location: remote.Location.DisplayName, Organizer: strings.ToLower(remote.Organizer.EmailAddress.Address), Start: start, End: end}
	if remote.IsCancelled {
		event.Status = "cancelled"
	} else {
		event.Status = "confirmed"
	}
	if remote.SeriesMasterID != "" && remote.OriginalStart != "" {
		var recurrenceID calendarapi.DateTime
		var parseErr error
		if remote.IsAllDay && len(remote.OriginalStart) >= 10 {
			recurrenceID, parseErr = calendarapi.NewDate(remote.OriginalStart[:10])
		} else {
			recurrenceID, parseErr = calendarapi.NewInstant(normalizeGraphInstant(remote.OriginalStart), "UTC")
		}
		if parseErr == nil {
			event.RecurrenceID = &recurrenceID
		}
	}
	if remote.Recurrence != nil {
		event.Recurrence = graphRecurrenceToRFC(*remote.Recurrence)
	}
	for _, attendee := range remote.Attendees {
		if attendee.EmailAddress.Address != "" {
			event.Attendees = append(event.Attendees, calendarapi.Attendee{Email: strings.ToLower(attendee.EmailAddress.Address), Name: attendee.EmailAddress.Name, Status: graphStatus(attendee.Status.Response)})
		}
	}
	if remote.IsReminderOn && remote.ReminderMinutes >= 0 && remote.ReminderMinutes <= 40320 {
		minutes := remote.ReminderMinutes
		event.ReminderMinutes = &minutes
	}
	return event, calendarapi.Validate(event)
}

func fromEvent(event calendarapi.Event) (graphEvent, error) {
	recurrence, err := rfcRecurrenceToGraph(event)
	if err != nil {
		return graphEvent{}, err
	}
	result := graphEvent{Subject: event.Summary, Body: graphBody{ContentType: "text", Content: event.Description}, Location: graphLocation{DisplayName: event.Location}, Start: toGraphDate(event.Start), End: toGraphDate(event.End), IsAllDay: event.Start.AllDay, Recurrence: recurrence}
	for _, attendee := range event.Attendees {
		result.Attendees = append(result.Attendees, graphAttendee{EmailAddress: graphEmail{Address: attendee.Email, Name: attendee.Name}, Type: "required"})
	}
	if event.ReminderMinutes != nil {
		result.IsReminderOn, result.ReminderMinutes = true, *event.ReminderMinutes
	}
	return result, nil
}

func graphDateValue(value graphDate, allDay bool) (calendarapi.DateTime, error) {
	if allDay {
		if len(value.DateTime) < 10 {
			return calendarapi.DateTime{}, calendarapi.ErrInvalidCalendar
		}
		return calendarapi.NewDate(value.DateTime[:10])
	}
	return calendarapi.NewInstant(normalizeGraphInstant(value.DateTime), "UTC")
}

func normalizeGraphInstant(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "Z") || strings.ContainsAny(strings.TrimPrefix(value[10:], "T"), "+-") {
		return value
	}
	return value + "Z"
}

func toGraphDate(value calendarapi.DateTime) graphDate {
	if value.AllDay {
		return graphDate{DateTime: value.Date + "T00:00:00.0000000", TimeZone: "UTC"}
	}
	return graphDate{DateTime: value.Instant.UTC().Format("2006-01-02T15:04:05.0000000"), TimeZone: "UTC"}
}

func graphRecurrenceToRFC(recurrence graphRecurrence) []string {
	pattern := recurrence.Pattern
	parts := []string{}
	switch pattern.Type {
	case "daily":
		parts = append(parts, "FREQ=DAILY")
	case "weekly":
		parts = append(parts, "FREQ=WEEKLY")
	case "absoluteMonthly", "relativeMonthly":
		parts = append(parts, "FREQ=MONTHLY")
	case "absoluteYearly", "relativeYearly":
		parts = append(parts, "FREQ=YEARLY")
	default:
		return nil
	}
	if pattern.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(pattern.Interval))
	}
	if len(pattern.DaysOfWeek) > 0 {
		days := []string{}
		for _, day := range pattern.DaysOfWeek {
			if mapped := graphDayToRFC(day); mapped != "" {
				days = append(days, mapped)
			}
		}
		if len(days) > 0 {
			parts = append(parts, "BYDAY="+strings.Join(days, ","))
		}
	}
	if pattern.DayOfMonth > 0 {
		parts = append(parts, "BYMONTHDAY="+strconv.Itoa(pattern.DayOfMonth))
	}
	if pattern.Month > 0 {
		parts = append(parts, "BYMONTH="+strconv.Itoa(pattern.Month))
	}
	if index := graphIndexToRFC(pattern.Index); index != 0 && strings.Contains(pattern.Type, "relative") {
		parts = append(parts, "BYSETPOS="+strconv.Itoa(index))
	}
	switch recurrence.Range.Type {
	case "numbered":
		if recurrence.Range.NumberOfOccurrences > 0 {
			parts = append(parts, "COUNT="+strconv.Itoa(recurrence.Range.NumberOfOccurrences))
		}
	case "endDate":
		if parsed, err := time.Parse("2006-01-02", recurrence.Range.EndDate); err == nil {
			parts = append(parts, "UNTIL="+parsed.Format("20060102")+"T235959Z")
		}
	}
	return []string{"RRULE:" + strings.Join(parts, ";")}
}

func rfcRecurrenceToGraph(event calendarapi.Event) (*graphRecurrence, error) {
	parts := calendarapi.RecurrenceParts(event.Recurrence)
	if len(parts) == 0 {
		return nil, nil
	}
	interval, _ := strconv.Atoi(parts["INTERVAL"])
	if interval <= 0 {
		interval = 1
	}
	pattern := recurrencePattern{Interval: interval, FirstDayOfWeek: "monday"}
	freq := parts["FREQ"]
	switch freq {
	case "DAILY":
		pattern.Type = "daily"
	case "WEEKLY":
		pattern.Type = "weekly"
	case "MONTHLY":
		if parts["BYSETPOS"] != "" {
			pattern.Type, pattern.Index = "relativeMonthly", rfcIndexToGraph(parts["BYSETPOS"])
		} else {
			pattern.Type = "absoluteMonthly"
		}
	case "YEARLY":
		if parts["BYSETPOS"] != "" {
			pattern.Type, pattern.Index = "relativeYearly", rfcIndexToGraph(parts["BYSETPOS"])
		} else {
			pattern.Type = "absoluteYearly"
		}
	default:
		return nil, calendarapi.ErrInvalidCalendar
	}
	if value, err := strconv.Atoi(parts["BYMONTHDAY"]); err == nil {
		pattern.DayOfMonth = value
	}
	if value, err := strconv.Atoi(parts["BYMONTH"]); err == nil {
		pattern.Month = value
	}
	for _, day := range strings.Split(parts["BYDAY"], ",") {
		if mapped := rfcDayToGraph(day); mapped != "" {
			pattern.DaysOfWeek = append(pattern.DaysOfWeek, mapped)
		}
	}
	startDate := event.Start.Date
	if !event.Start.AllDay {
		startDate = event.Start.Instant.UTC().Format("2006-01-02")
	}
	rangeValue := recurrenceRange{Type: "noEnd", StartDate: startDate, RecurrenceTimeZone: "UTC"}
	if count, err := strconv.Atoi(parts["COUNT"]); err == nil && count > 0 {
		rangeValue.Type, rangeValue.NumberOfOccurrences = "numbered", count
	} else if until := parts["UNTIL"]; len(until) >= 8 {
		if parsed, err := time.Parse("20060102", until[:8]); err == nil {
			rangeValue.Type, rangeValue.EndDate = "endDate", parsed.Format("2006-01-02")
		}
	}
	return &graphRecurrence{Pattern: pattern, Range: rangeValue}, nil
}

func equivalent(desired calendarapi.Event, remote graphEvent) bool {
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
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "microsoft_calendar_authentication_failed", Message: "Microsoft Calendar authorization expired; reconnect the account."}
	}
	if errors.Is(err, calendarapi.ErrInvalidCalendar) || errors.Is(err, calendarsync.ErrInvalidSnapshot) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "microsoft_calendar_invalid_data", Message: "Microsoft Calendar returned an invalid event snapshot."}
	}
	return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "microsoft_calendar_sync_failed", Message: "Microsoft Calendar synchronization failed."}
}

func stableID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "calendar_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}

func transactionID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
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

func graphColor(value string) string {
	colors := map[string]string{"lightBlue": "#4FC3F7", "lightGreen": "#81C784", "lightOrange": "#FFB74D", "lightGray": "#B0BEC5", "lightYellow": "#FFF176", "lightTeal": "#4DB6AC", "lightPink": "#F48FB1", "lightBrown": "#A1887F", "lightRed": "#E57373", "maxColor": "#7986CB"}
	return colors[value]
}

func graphStatus(value string) string {
	switch strings.ToLower(value) {
	case "accepted", "declined", "tentativelyaccepted":
		if strings.EqualFold(value, "tentativelyAccepted") {
			return "tentative"
		}
		return strings.ToLower(value)
	default:
		return "needsAction"
	}
}

func graphDayToRFC(value string) string {
	return map[string]string{"sunday": "SU", "monday": "MO", "tuesday": "TU", "wednesday": "WE", "thursday": "TH", "friday": "FR", "saturday": "SA"}[strings.ToLower(value)]
}

func rfcDayToGraph(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) > 2 {
		value = value[len(value)-2:]
	}
	return map[string]string{"SU": "sunday", "MO": "monday", "TU": "tuesday", "WE": "wednesday", "TH": "thursday", "FR": "friday", "SA": "saturday"}[value]
}

func graphIndexToRFC(value string) int {
	return map[string]int{"first": 1, "second": 2, "third": 3, "fourth": 4, "last": -1}[value]
}

func rfcIndexToGraph(value string) string {
	return map[string]string{"1": "first", "2": "second", "3": "third", "4": "fourth", "-1": "last"}[value]
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func (connector *Connector) String() string {
	return fmt.Sprintf("Microsoft Graph Calendar API (%s)", connector.base)
}
