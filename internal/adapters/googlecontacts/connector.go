// Package googlecontacts implements bounded Google People API contact sync
// and source-ETag mutations for Gmail and Google Workspace accounts.
package googlecontacts

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
	"github.com/d31ma/caduceus/internal/adapters/contactapi"
	"github.com/d31ma/caduceus/internal/contactsync"
)

const defaultBaseURL = "https://people.googleapis.com/v1/"
const maxResponseBytes = 4 << 20
const personFields = "addresses,biographies,birthdays,clientData,emailAddresses,metadata,names,nicknames,organizations,phoneNumbers,urls"
const updateFields = "addresses,biographies,birthdays,emailAddresses,names,nicknames,organizations,phoneNumbers,urls"
const markerKey = "caduceus-idempotency"

var (
	ErrAuthentication = errors.New("Google People authentication failed")
	ErrProtocol       = errors.New("Google People protocol failed")
	ErrSyncToken      = errors.New("Google People sync token expired")
	ErrPrecondition   = errors.New("Google People contact changed")
	ErrAmbiguous      = errors.New("Google People create outcome is ambiguous")
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

type listResponse struct {
	Connections   []person `json:"connections"`
	NextPageToken string   `json:"nextPageToken"`
	NextSyncToken string   `json:"nextSyncToken"`
}

type person struct {
	ResourceName string `json:"resourceName,omitempty"`
	ETag         string `json:"etag,omitempty"`
	Metadata     struct {
		Deleted bool     `json:"deleted,omitempty"`
		Sources []source `json:"sources,omitempty"`
	} `json:"metadata,omitempty"`
	Names          []name         `json:"names,omitempty"`
	EmailAddresses []typedValue   `json:"emailAddresses,omitempty"`
	PhoneNumbers   []typedValue   `json:"phoneNumbers,omitempty"`
	Addresses      []address      `json:"addresses,omitempty"`
	URLs           []typedValue   `json:"urls,omitempty"`
	Nicknames      []textValue    `json:"nicknames,omitempty"`
	Birthdays      []birthday     `json:"birthdays,omitempty"`
	Biographies    []textValue    `json:"biographies,omitempty"`
	Organizations  []organization `json:"organizations,omitempty"`
	ClientData     []clientDatum  `json:"clientData,omitempty"`
}

type source struct {
	Type string `json:"type"`
	ETag string `json:"etag,omitempty"`
}
type name struct {
	DisplayName string `json:"displayName,omitempty"`
	GivenName   string `json:"givenName,omitempty"`
	FamilyName  string `json:"familyName,omitempty"`
}
type typedValue struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}
type textValue struct {
	Value string `json:"value"`
}
type address struct {
	StreetAddress   string `json:"streetAddress,omitempty"`
	ExtendedAddress string `json:"extendedAddress,omitempty"`
	City            string `json:"city,omitempty"`
	Region          string `json:"region,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	Country         string `json:"country,omitempty"`
	Type            string `json:"type,omitempty"`
}
type birthday struct {
	Date struct {
		Year  int `json:"year,omitempty"`
		Month int `json:"month,omitempty"`
		Day   int `json:"day,omitempty"`
	} `json:"date"`
}
type organization struct {
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Current bool   `json:"current,omitempty"`
}
type clientDatum struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func New(options Options) (*Connector, error) {
	if strings.TrimSpace(options.BaseURL) == "" {
		options.BaseURL = defaultBaseURL
	}
	base, err := url.Parse(options.BaseURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || base.RawQuery != "" || base.Fragment != "" || !secureOrLoopback(base) {
		return nil, errors.New("Google People API base URL is invalid")
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

func (connector *Connector) Sync(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, state contactsync.State) (contactsync.Snapshot, error) {
	if connector == nil || connector.http == nil || account.Provider != accounts.ProviderGmail || strings.TrimSpace(credentials.AccessToken) == "" {
		return contactsync.Snapshot{}, accounts.ErrUnsupported
	}
	bookURL := connector.endpoint("people/me/connections")
	bookID := stableID(bookURL)
	previousToken := ""
	contacts := map[string]contactsync.Contact{}
	for _, book := range state.AddressBooks {
		if book.URL == bookURL {
			previousToken = book.SyncToken
		}
	}
	if previousToken != "" {
		for _, contact := range state.Contacts {
			if contact.AddressBookID == bookID {
				contacts[contact.Href] = contact
			}
		}
	}
	people, token, err := connector.list(ctx, credentials.AccessToken, previousToken)
	if errors.Is(err, ErrSyncToken) {
		contacts = map[string]contactsync.Contact{}
		people, token, err = connector.list(ctx, credentials.AccessToken, "")
	}
	if err != nil {
		return contactsync.Snapshot{}, classify(err)
	}
	now := connector.clock().UTC()
	for _, remote := range people {
		if !strings.HasPrefix(remote.ResourceName, "people/") || len(remote.ResourceName) <= len("people/") || strings.ContainsAny(remote.ResourceName, "?#") {
			return contactsync.Snapshot{}, classify(ErrProtocol)
		}
		href := connector.endpoint(strings.TrimPrefix(remote.ResourceName, "/"))
		if remote.Metadata.Deleted {
			delete(contacts, href)
			continue
		}
		contact, projectErr := project(account.ID, bookID, href, remote, now)
		if projectErr != nil {
			return contactsync.Snapshot{}, classify(projectErr)
		}
		contacts[href] = contact
	}
	snapshot := contactsync.Snapshot{PrincipalURL: connector.endpoint("people/me"), HomeURL: bookURL, SyncedAt: now,
		AddressBooks: []contactsync.AddressBook{{StableID: bookID, URL: bookURL, Name: "Google Contacts", Writable: true, SyncToken: token}}}
	for _, contact := range contacts {
		snapshot.Contacts = append(snapshot.Contacts, contact)
	}
	if len(snapshot.Contacts) > contactsync.MaxContacts {
		return contactsync.Snapshot{}, contactsync.ErrInvalidSnapshot
	}
	sort.Slice(snapshot.Contacts, func(i, j int) bool { return snapshot.Contacts[i].ID < snapshot.Contacts[j].ID })
	return snapshot, nil
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation contactsync.Mutation, current contactsync.Contact) (contactsync.MutationOutcome, error) {
	if account.Provider != accounts.ProviderGmail || current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || current.ID != mutation.ContactID || strings.TrimSpace(credentials.AccessToken) == "" {
		return contactsync.OutcomeConflict, nil
	}
	if mutation.Kind != contactsync.MutationCreate && mutation.ExpectedETag != current.ETag {
		return contactsync.OutcomeConflict, nil
	}
	switch mutation.Kind {
	case contactsync.MutationCreate:
		marker := markerValue(mutation.IdempotencyKey)
		found, findErr := connector.findMarker(ctx, credentials.AccessToken, marker)
		if findErr != nil {
			return "", classify(findErr)
		}
		if found {
			return contactsync.OutcomeApplied, nil
		}
		if mutation.Attempts > 1 {
			return "", classify(ErrAmbiguous)
		}
		payload := fromContact(mutation.Desired)
		payload.ClientData = []clientDatum{{Key: markerKey, Value: marker}}
		body, _ := json.Marshal(payload)
		_, status, callErr := connector.call(ctx, http.MethodPost, connector.endpoint("people:createContact"), credentials.AccessToken, "", body)
		if callErr == nil && status == http.StatusOK {
			return contactsync.OutcomeApplied, nil
		}
		return "", classify(callErr)
	case contactsync.MutationUpdate:
		payload := fromContact(mutation.Desired)
		payload.ResourceName = connector.remoteName(current.Href)
		payload.Metadata.Sources = []source{{Type: "CONTACT", ETag: current.ETag}}
		target := current.Href + ":updateContact?updatePersonFields=" + url.QueryEscape(updateFields) + "&personFields=" + url.QueryEscape(personFields)
		body, _ := json.Marshal(payload)
		_, status, callErr := connector.call(ctx, http.MethodPatch, target, credentials.AccessToken, "", body)
		if callErr == nil && status == http.StatusOK {
			return contactsync.OutcomeApplied, nil
		}
		if errors.Is(callErr, ErrPrecondition) {
			return contactsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	case contactsync.MutationDelete:
		remote, fetchErr := connector.fetch(ctx, current.Href, credentials.AccessToken)
		if fetchErr != nil {
			if mutation.State == contactsync.MutationApplying && errors.Is(fetchErr, contactsync.ErrContactNotFound) {
				return contactsync.OutcomeApplied, nil
			}
			return "", classify(fetchErr)
		}
		if sourceETag(remote) != current.ETag {
			return contactsync.OutcomeConflict, nil
		}
		_, status, callErr := connector.call(ctx, http.MethodDelete, current.Href+":deleteContact", credentials.AccessToken, "", nil)
		if callErr == nil && status == http.StatusNoContent || status == http.StatusNotFound && mutation.State == contactsync.MutationApplying {
			return contactsync.OutcomeApplied, nil
		}
		return "", classify(callErr)
	default:
		return "", contactsync.ErrInvalidMutation
	}
}

func (connector *Connector) list(ctx context.Context, token, syncToken string) ([]person, string, error) {
	people := []person{}
	pageToken := ""
	for page := 0; page < 100; page++ {
		target, _ := url.Parse(connector.endpoint("people/me/connections"))
		query := target.Query()
		query.Set("pageSize", "1000")
		query.Set("personFields", personFields)
		query.Set("requestSyncToken", "true")
		if syncToken != "" {
			query.Set("syncToken", syncToken)
		}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		target.RawQuery = query.Encode()
		body, status, callErr := connector.call(ctx, http.MethodGet, target.String(), token, "", nil)
		if status == http.StatusBadRequest && strings.Contains(string(body), "EXPIRED_SYNC_TOKEN") {
			return nil, "", ErrSyncToken
		}
		if callErr != nil {
			return nil, "", callErr
		}
		var response listResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, "", ErrProtocol
		}
		people = append(people, response.Connections...)
		if len(people) > contactsync.MaxContacts {
			return nil, "", contactsync.ErrInvalidSnapshot
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			if response.NextSyncToken == "" {
				return nil, "", ErrProtocol
			}
			return people, response.NextSyncToken, nil
		}
	}
	return nil, "", ErrProtocol
}

func (connector *Connector) findMarker(ctx context.Context, token, marker string) (bool, error) {
	people, _, err := connector.list(ctx, token, "")
	if err != nil {
		return false, err
	}
	for _, person := range people {
		for _, datum := range person.ClientData {
			if datum.Key == markerKey && datum.Value == marker {
				return true, nil
			}
		}
	}
	return false, nil
}

func (connector *Connector) fetch(ctx context.Context, target, token string) (person, error) {
	target += "?personFields=" + url.QueryEscape(personFields)
	body, status, err := connector.call(ctx, http.MethodGet, target, token, "", nil)
	if status == http.StatusNotFound {
		return person{}, contactsync.ErrContactNotFound
	}
	if err != nil {
		return person{}, err
	}
	var result person
	if json.Unmarshal(body, &result) != nil || result.ResourceName == "" {
		return person{}, ErrProtocol
	}
	return result, nil
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
	if response.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "failedPrecondition") {
		return body, response.StatusCode, ErrPrecondition
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return body, response.StatusCode, ErrProtocol
	}
	return body, response.StatusCode, nil
}

func project(accountID accounts.AccountID, bookID, href string, remote person, now time.Time) (contactsync.Contact, error) {
	contact := toContact(remote)
	contact.ID, contact.AccountID, contact.AddressBookID, contact.Href, contact.ETag, contact.UID, contact.RemoteUpdatedAt = contactsync.RemoteContactID(accountID, href), accountID, bookID, href, sourceETag(remote), remote.ResourceName, now
	raw, err := contactapi.RenderVCard(contact)
	if err != nil {
		return contactsync.Contact{}, err
	}
	contact.RawVCard = raw
	return contact, nil
}

func toContact(remote person) contactsync.Contact {
	contact := contactsync.Contact{}
	if len(remote.Names) > 0 {
		contact.Name, contact.GivenName, contact.FamilyName = remote.Names[0].DisplayName, remote.Names[0].GivenName, remote.Names[0].FamilyName
	}
	for _, value := range remote.EmailAddresses {
		contact.Emails = append(contact.Emails, contactsync.Value{Value: value.Value, Type: value.Type})
	}
	for _, value := range remote.PhoneNumbers {
		contact.Phones = append(contact.Phones, contactsync.Value{Value: value.Value, Type: value.Type})
	}
	for _, value := range remote.URLs {
		contact.URLs = append(contact.URLs, contactsync.Value{Value: value.Value, Type: value.Type})
	}
	for _, value := range remote.Addresses {
		contact.Addresses = append(contact.Addresses, contactsync.Address{StreetAddress: value.StreetAddress, ExtendedAddress: value.ExtendedAddress, City: value.City, Region: value.Region, PostalCode: value.PostalCode, Country: value.Country, Type: value.Type})
	}
	for _, value := range remote.Nicknames {
		contact.Nicknames = append(contact.Nicknames, value.Value)
	}
	if len(remote.Biographies) > 0 {
		contact.Notes = remote.Biographies[0].Value
	}
	if len(remote.Birthdays) > 0 && remote.Birthdays[0].Date.Month > 0 && remote.Birthdays[0].Date.Day > 0 {
		date := remote.Birthdays[0].Date
		if date.Year > 0 {
			contact.Birthday = fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)
		} else {
			contact.Birthday = fmt.Sprintf("--%02d-%02d", date.Month, date.Day)
		}
	}
	for _, value := range remote.Organizations {
		if contact.Company == "" || value.Current {
			contact.Company, contact.Title = value.Name, value.Title
		}
		if value.Current {
			break
		}
	}
	return contact
}

func fromContact(contact contactsync.Contact) person {
	result := person{}
	displayName := name{DisplayName: contact.Name, GivenName: contact.GivenName, FamilyName: contact.FamilyName}
	if displayName.DisplayName != "" || displayName.GivenName != "" || displayName.FamilyName != "" {
		result.Names = []name{displayName}
	}
	for _, value := range contact.Emails {
		result.EmailAddresses = append(result.EmailAddresses, typedValue{Value: value.Value, Type: value.Type})
	}
	for _, value := range contact.Phones {
		result.PhoneNumbers = append(result.PhoneNumbers, typedValue{Value: value.Value, Type: value.Type})
	}
	for _, value := range contact.URLs {
		result.URLs = append(result.URLs, typedValue{Value: value.Value, Type: value.Type})
	}
	for _, value := range contact.Addresses {
		result.Addresses = append(result.Addresses, address{StreetAddress: value.StreetAddress, ExtendedAddress: value.ExtendedAddress, City: value.City, Region: value.Region, PostalCode: value.PostalCode, Country: value.Country, Type: value.Type})
	}
	for _, value := range contact.Nicknames {
		result.Nicknames = append(result.Nicknames, textValue{Value: value})
	}
	if contact.Notes != "" {
		result.Biographies = []textValue{{Value: contact.Notes}}
	}
	if parsed := parseBirthday(contact.Birthday); parsed != nil {
		result.Birthdays = []birthday{*parsed}
	}
	if contact.Company != "" || contact.Title != "" {
		result.Organizations = []organization{{Name: contact.Company, Title: contact.Title, Current: true}}
	}
	return result
}

func parseBirthday(value string) *birthday {
	parts := strings.Split(strings.TrimPrefix(value, "--"), "-")
	if len(parts) == 3 {
		result := birthday{}
		result.Date.Year, _ = strconv.Atoi(parts[0])
		result.Date.Month, _ = strconv.Atoi(parts[1])
		result.Date.Day, _ = strconv.Atoi(parts[2])
		return &result
	}
	if len(parts) == 2 {
		result := birthday{}
		result.Date.Month, _ = strconv.Atoi(parts[0])
		result.Date.Day, _ = strconv.Atoi(parts[1])
		return &result
	}
	return nil
}

func sourceETag(value person) string {
	for _, source := range value.Metadata.Sources {
		if source.Type == "CONTACT" && source.ETag != "" {
			return source.ETag
		}
	}
	return value.ETag
}

func markerValue(key string) string {
	digest := sha256.Sum256([]byte(key))
	return base64.RawURLEncoding.EncodeToString(digest[:18])
}
func stableID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "book_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}
func (connector *Connector) remoteName(href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	path := strings.TrimPrefix(parsed.EscapedPath(), connector.base.EscapedPath())
	value, _ := url.PathUnescape(path)
	return strings.TrimPrefix(value, "/")
}
func (connector *Connector) endpoint(relative string) string {
	return connector.base.ResolveReference(&url.URL{Path: strings.TrimPrefix(relative, "/")}).String()
}
func (connector *Connector) safeURL(target string) (*url.URL, error) {
	parsed, err := url.Parse(target)
	if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != connector.base.Scheme || !strings.EqualFold(parsed.Host, connector.base.Host) || !strings.HasPrefix(parsed.EscapedPath(), connector.base.EscapedPath()) {
		return nil, ErrProtocol
	}
	return parsed, nil
}
func secureOrLoopback(parsed *url.URL) bool {
	if parsed.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return parsed.Scheme == "http" && ip != nil && ip.IsLoopback()
}
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAuthentication) {
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "google_contacts_authentication_failed", Message: "Google Contacts authorization expired; reconnect the account."}
	}
	if errors.Is(err, contactapi.ErrInvalidContact) || errors.Is(err, contactsync.ErrInvalidSnapshot) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "google_contacts_invalid_data", Message: "Google Contacts returned invalid contact data."}
	}
	if errors.Is(err, ErrAmbiguous) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "google_contacts_create_ambiguous", Message: "Google Contacts has not yet confirmed the contact creation; retry after provider synchronization completes."}
	}
	return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "google_contacts_sync_failed", Message: "Google Contacts synchronization failed."}
}
