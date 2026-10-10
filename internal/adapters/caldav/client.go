// Package caldav implements bounded CalDAV discovery and synchronization.
package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/adapters/carddav"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

const (
	statusMultiStatus       = 207
	defaultMaxResponseBytes = int64(32 << 20)
	maximumCalendars        = 100
	maximumResources        = 10_000
)

var (
	ErrAuthentication   = errors.New("CalDAV authentication failed")
	ErrProtocol         = errors.New("CalDAV protocol failed")
	ErrResponseTooLarge = errors.New("CalDAV response exceeds limit")
	ErrUnsafeEndpoint   = errors.New("unsafe CalDAV endpoint")
	ErrSyncTokenInvalid = errors.New("CalDAV sync token is invalid")
	ErrPrecondition     = errors.New("CalDAV resource changed remotely")
)

type Options struct {
	HTTPClient       *http.Client
	MaxResponseBytes int64
	Resolver         carddav.IPResolver
	PrivateCIDRs     []string
	Timeout          time.Duration
}

type Client struct {
	httpClient *http.Client
	maxBytes   int64
}

type Auth struct {
	Username         string
	Password         string
	AllowInsecureTLS bool
}

type Discovery struct {
	PrincipalURL string
	HomeURL      string
	Calendars    []Calendar
}

type Calendar struct {
	URL       string
	Name      string
	Color     string
	SyncToken string
	CTag      string
}

type RemoteResource struct {
	Href      string
	ETag      string
	ICalendar string
}

type Delta struct {
	SyncToken string
	Changed   []RemoteResource
	Deleted   []string
}

func (client *Client) Fetch(ctx context.Context, target string, auth Auth) (RemoteResource, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return RemoteResource{}, ErrUnsafeEndpoint
	}
	request.Header.Set("Accept", "text/calendar")
	response, err := client.doResourceRequest(request, auth)
	if err != nil {
		return RemoteResource{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RemoteResource{}, ErrProtocol
	}
	limit := int64(calendarsync.MaxICalendarBytes)
	if client.maxBytes < limit {
		limit = client.maxBytes
	}
	if response.ContentLength > limit {
		return RemoteResource{}, ErrResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return RemoteResource{}, ErrProtocol
	}
	if int64(len(payload)) > limit || len(payload) == 0 {
		return RemoteResource{}, ErrResponseTooLarge
	}
	if response.Request == nil || response.Request.URL == nil {
		return RemoteResource{}, ErrProtocol
	}
	return RemoteResource{Href: response.Request.URL.String(), ETag: strings.TrimSpace(response.Header.Get("ETag")), ICalendar: string(payload)}, nil
}

func (client *Client) Put(ctx context.Context, target, calendar, expectedETag string, create bool, auth Auth) (string, error) {
	if len(calendar) == 0 || len(calendar) > calendarsync.MaxICalendarBytes {
		return "", ErrProtocol
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target, strings.NewReader(calendar))
	if err != nil {
		return "", ErrUnsafeEndpoint
	}
	request.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	if create {
		request.Header.Set("If-None-Match", "*")
	} else {
		if strings.TrimSpace(expectedETag) == "" {
			return "", ErrPrecondition
		}
		request.Header.Set("If-Match", expectedETag)
	}
	response, err := client.doResourceRequest(request, auth)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusPreconditionFailed || response.StatusCode == http.StatusConflict {
		return "", ErrPrecondition
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusOK {
		return "", ErrProtocol
	}
	return strings.TrimSpace(response.Header.Get("ETag")), nil
}

func (client *Client) Delete(ctx context.Context, target, expectedETag string, auth Auth) error {
	if strings.TrimSpace(expectedETag) == "" {
		return ErrPrecondition
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return ErrUnsafeEndpoint
	}
	request.Header.Set("If-Match", expectedETag)
	response, err := client.doResourceRequest(request, auth)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusPreconditionFailed || response.StatusCode == http.StatusConflict {
		return ErrPrecondition
	}
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNotFound {
		return ErrProtocol
	}
	return nil
}

func (client *Client) doResourceRequest(request *http.Request, auth Auth) (*http.Response, error) {
	request.Header.Set("User-Agent", "CADUCEUS-CalDAV/1")
	request.SetBasicAuth(auth.Username, auth.Password)
	request = request.WithContext(carddav.WithInsecureTLS(request.Context(), auth.AllowInsecureTLS))
	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, carddav.ErrUnsafeEndpoint) {
			return nil, ErrUnsafeEndpoint
		}
		return nil, ErrProtocol
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		response.Body.Close()
		return nil, ErrAuthentication
	}
	return response, nil
}

func NewClient(options Options) (*Client, error) {
	httpClient := options.HTTPClient
	if httpClient == nil {
		var err error
		httpClient, err = carddav.NewSafeHTTPClient(options.Resolver, options.PrivateCIDRs, options.Timeout)
		if err != nil {
			return nil, err
		}
	}
	limit := options.MaxResponseBytes
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	return &Client{httpClient: httpClient, maxBytes: limit}, nil
}

func (client *Client) Discover(ctx context.Context, endpoint *accounts.CalDAVEndpoint, credentials accounts.Credentials) (Discovery, error) {
	if client == nil || endpoint == nil || client.httpClient == nil {
		return Discovery{}, ErrUnsafeEndpoint
	}
	bootstrap, err := parseEndpoint(endpoint.URL)
	if err != nil {
		return Discovery{}, err
	}
	if bootstrap.Path == "" || bootstrap.Path == "/" {
		bootstrap.Path = "/.well-known/caldav"
	}
	auth := endpointAuth(endpoint, credentials)
	if auth.Username == "" || auth.Password == "" {
		return Discovery{}, ErrAuthentication
	}
	bootstrapResponses, effective, err := client.request(ctx, "PROPFIND", bootstrap.String(), "0", currentPrincipalRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	principalURL, err := resolveReference(effective, firstSuccessfulProperty(bootstrapResponses, func(property davProperty) string { return property.CurrentUserPrincipal.Href }))
	if err != nil {
		return Discovery{}, ErrProtocol
	}
	principalResponses, principalEffective, err := client.request(ctx, "PROPFIND", principalURL, "0", calendarHomeRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	homeURL, err := resolveReference(principalEffective, firstSuccessfulProperty(principalResponses, func(property davProperty) string { return property.CalendarHomeSet.Href }))
	if err != nil {
		return Discovery{}, ErrProtocol
	}
	homeResponses, homeEffective, err := client.request(ctx, "PROPFIND", homeURL, "1", calendarListRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	calendars := make([]Calendar, 0)
	for _, response := range homeResponses {
		for _, property := range successfulProperties(response) {
			if !property.ResourceType.Calendar {
				continue
			}
			calendarURL, resolveErr := resolveReference(homeEffective, response.Href)
			if resolveErr != nil {
				return Discovery{}, ErrProtocol
			}
			name := strings.TrimSpace(property.DisplayName)
			if name == "" {
				name = "Calendar"
			}
			calendars = append(calendars, Calendar{URL: calendarURL, Name: name, Color: strings.TrimSpace(property.CalendarColor), SyncToken: strings.TrimSpace(property.SyncToken), CTag: strings.TrimSpace(property.CTag)})
			if len(calendars) > maximumCalendars {
				return Discovery{}, ErrProtocol
			}
		}
	}
	if len(calendars) == 0 {
		return Discovery{}, ErrProtocol
	}
	return Discovery{PrincipalURL: principalURL, HomeURL: homeURL, Calendars: calendars}, nil
}

func (client *Client) InitialSync(ctx context.Context, calendar Calendar, auth Auth) ([]RemoteResource, error) {
	responses, effective, err := client.request(ctx, "REPORT", calendar.URL, "1", calendarQueryRequest, auth)
	if err != nil {
		return nil, err
	}
	resources := make([]RemoteResource, 0)
	for _, response := range responses {
		for _, property := range successfulProperties(response) {
			if strings.TrimSpace(property.CalendarData) == "" {
				continue
			}
			href, resolveErr := resolveReference(effective, response.Href)
			if resolveErr != nil {
				return nil, ErrProtocol
			}
			resources = append(resources, RemoteResource{Href: href, ETag: strings.TrimSpace(property.GetETag), ICalendar: property.CalendarData})
			if len(resources) > maximumResources {
				return nil, ErrProtocol
			}
		}
	}
	return resources, nil
}

func (client *Client) IncrementalSync(ctx context.Context, calendar Calendar, previousToken string, auth Auth) (Delta, error) {
	if strings.TrimSpace(previousToken) == "" {
		return Delta{}, ErrSyncTokenInvalid
	}
	document, effective, err := client.requestDocument(ctx, "REPORT", calendar.URL, "1", syncCollectionRequest(strings.TrimSpace(previousToken)), auth)
	if err != nil {
		var status davStatusError
		if errors.As(err, &status) && (status.Code == http.StatusForbidden || status.Code == http.StatusConflict) {
			return Delta{}, ErrSyncTokenInvalid
		}
		return Delta{}, err
	}
	delta := Delta{SyncToken: strings.TrimSpace(document.SyncToken)}
	if delta.SyncToken == "" {
		return Delta{}, ErrProtocol
	}
	type changedReference struct{ raw, canonical string }
	changed := make([]changedReference, 0)
	seen := map[string]bool{}
	for _, response := range document.Responses {
		rawHref := strings.TrimSpace(response.Href)
		if rawHref == "" || seen[rawHref] {
			continue
		}
		canonical, resolveErr := resolveReference(effective, rawHref)
		if resolveErr != nil {
			return Delta{}, ErrProtocol
		}
		if strings.Contains(response.Status, " 404 ") {
			delta.Deleted = append(delta.Deleted, canonical)
			seen[rawHref] = true
			continue
		}
		for _, property := range successfulProperties(response) {
			if strings.TrimSpace(property.GetETag) != "" {
				changed = append(changed, changedReference{raw: rawHref, canonical: canonical})
				seen[rawHref] = true
				break
			}
		}
		if len(changed)+len(delta.Deleted) > maximumResources {
			return Delta{}, ErrProtocol
		}
	}
	for start := 0; start < len(changed); start += 100 {
		end := start + 100
		if end > len(changed) {
			end = len(changed)
		}
		hrefs := make([]string, 0, end-start)
		for _, reference := range changed[start:end] {
			hrefs = append(hrefs, reference.raw)
		}
		responses, multigetEffective, requestErr := client.request(ctx, "REPORT", effective, "1", calendarMultigetRequest(hrefs), auth)
		if requestErr != nil {
			return Delta{}, requestErr
		}
		for _, response := range responses {
			for _, property := range successfulProperties(response) {
				if strings.TrimSpace(property.CalendarData) == "" {
					continue
				}
				href, resolveErr := resolveReference(multigetEffective, response.Href)
				if resolveErr != nil {
					return Delta{}, ErrProtocol
				}
				delta.Changed = append(delta.Changed, RemoteResource{Href: href, ETag: strings.TrimSpace(property.GetETag), ICalendar: property.CalendarData})
			}
		}
	}
	if len(delta.Changed) != len(changed) {
		return Delta{}, ErrProtocol
	}
	return delta, nil
}

func endpointAuth(endpoint *accounts.CalDAVEndpoint, credentials accounts.Credentials) Auth {
	auth := Auth{Username: strings.TrimSpace(endpoint.Username), Password: credentials.Password, AllowInsecureTLS: endpoint.AllowInsecureTLS}
	if auth.Username == "" {
		auth.Username = strings.TrimSpace(credentials.Username)
	}
	return auth
}

func (client *Client) request(ctx context.Context, method, target, depth, body string, auth Auth) ([]davResponse, string, error) {
	document, effective, err := client.requestDocument(ctx, method, target, depth, body, auth)
	if err != nil {
		var status davStatusError
		if errors.As(err, &status) && (status.Code == http.StatusUnauthorized || status.Code == http.StatusForbidden) {
			return nil, "", ErrAuthentication
		}
		return nil, "", err
	}
	return document.Responses, effective, nil
}

func (client *Client) requestDocument(ctx context.Context, method, target, depth, body string, auth Auth) (davMultistatus, string, error) {
	request, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return davMultistatus{}, "", ErrUnsafeEndpoint
	}
	request.Header.Set("Accept", "application/xml,text/xml;q=0.9")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	request.Header.Set("Depth", depth)
	request.Header.Set("User-Agent", "CADUCEUS-CalDAV/1")
	request.SetBasicAuth(auth.Username, auth.Password)
	request = request.WithContext(carddav.WithInsecureTLS(request.Context(), auth.AllowInsecureTLS))
	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, carddav.ErrUnsafeEndpoint) {
			return davMultistatus{}, "", ErrUnsafeEndpoint
		}
		return davMultistatus{}, "", fmt.Errorf("%w: request", ErrProtocol)
	}
	defer response.Body.Close()
	if response.StatusCode != statusMultiStatus {
		return davMultistatus{}, "", davStatusError{Code: response.StatusCode}
	}
	if response.ContentLength > client.maxBytes {
		return davMultistatus{}, "", ErrResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, client.maxBytes+1))
	if err != nil || int64(len(payload)) > client.maxBytes {
		return davMultistatus{}, "", ErrResponseTooLarge
	}
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.Strict = true
	var multistatus davMultistatus
	if err := decoder.Decode(&multistatus); err != nil {
		return davMultistatus{}, "", ErrProtocol
	}
	if response.Request == nil || response.Request.URL == nil {
		return davMultistatus{}, "", ErrProtocol
	}
	return multistatus, response.Request.URL.String(), nil
}

func parseEndpoint(value string) (*url.URL, error) {
	if len(value) > 2048 {
		return nil, ErrUnsafeEndpoint
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || parsed.Fragment != "" || parsed.Scheme != "https" {
		return nil, ErrUnsafeEndpoint
	}
	return parsed, nil
}

func resolveReference(base, reference string) (string, error) {
	if strings.TrimSpace(reference) == "" {
		return "", ErrProtocol
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(reference))
	if err != nil {
		return "", err
	}
	resolved := baseURL.ResolveReference(ref)
	if resolved.User != nil || resolved.Hostname() == "" || (resolved.Scheme != "https" && resolved.Scheme != "http") {
		return "", ErrProtocol
	}
	return resolved.String(), nil
}

type davMultistatus struct {
	SyncToken string        `xml:"sync-token"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	Status    string        `xml:"status"`
	Propstats []davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Property davProperty `xml:"prop"`
	Status   string      `xml:"status"`
}

type davProperty struct {
	CurrentUserPrincipal hrefProperty `xml:"current-user-principal"`
	CalendarHomeSet      hrefProperty `xml:"calendar-home-set"`
	DisplayName          string       `xml:"displayname"`
	ResourceType         resourceType `xml:"resourcetype"`
	SyncToken            string       `xml:"sync-token"`
	CTag                 string       `xml:"getctag"`
	CalendarColor        string       `xml:"calendar-color"`
	GetETag              string       `xml:"getetag"`
	CalendarData         string       `xml:"calendar-data"`
}

type hrefProperty struct {
	Href string `xml:"href"`
}
type resourceType struct{ Calendar bool }

func (value *resourceType) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			if current.Name.Local == "calendar" {
				value.Calendar = true
			}
			if err := decoder.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

type davStatusError struct{ Code int }

func (failure davStatusError) Error() string {
	return fmt.Sprintf("CalDAV HTTP status %d", failure.Code)
}

func successfulProperties(response davResponse) []davProperty {
	properties := make([]davProperty, 0, len(response.Propstats))
	for _, propstat := range response.Propstats {
		if strings.Contains(propstat.Status, " 200 ") {
			properties = append(properties, propstat.Property)
		}
	}
	return properties
}

func firstSuccessfulProperty(responses []davResponse, selectValue func(davProperty) string) string {
	for _, response := range responses {
		for _, property := range successfulProperties(response) {
			if value := strings.TrimSpace(selectValue(property)); value != "" {
				return value
			}
		}
	}
	return ""
}

const currentPrincipalRequest = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
const calendarHomeRequest = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`
const calendarListRequest = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/" xmlns:ical="http://apple.com/ns/ical/"><d:prop><d:displayname/><d:resourcetype/><d:sync-token/><cs:getctag/><ical:calendar-color/></d:prop></d:propfind>`
const calendarQueryRequest = `<?xml version="1.0" encoding="utf-8"?><c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop><c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"/></c:comp-filter></c:filter></c:calendar-query>`

func syncCollectionRequest(token string) string {
	return `<?xml version="1.0" encoding="utf-8"?><d:sync-collection xmlns:d="DAV:"><d:sync-token>` + xmlText(token) + `</d:sync-token><d:sync-level>1</d:sync-level><d:prop><d:getetag/></d:prop></d:sync-collection>`
}

func calendarMultigetRequest(hrefs []string) string {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="utf-8"?><c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop>`)
	for _, href := range hrefs {
		body.WriteString("<d:href>")
		body.WriteString(xmlText(href))
		body.WriteString("</d:href>")
	}
	body.WriteString(`</c:calendar-multiget>`)
	return body.String()
}

func xmlText(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}
