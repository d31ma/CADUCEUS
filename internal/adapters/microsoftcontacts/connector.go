// Package microsoftcontacts implements bounded Microsoft Graph v1 contact
// synchronization and conditional mutations for delegated OAuth mailboxes.
package microsoftcontacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/adapters/contactapi"
	"github.com/d31ma/caduceus/internal/contactsync"
)

const defaultBaseURL = "https://graph.microsoft.com/v1.0/"
const maxResponseBytes = 4 << 20
const extensionName = "com.delma.caduceus"

var (
	ErrAuthentication = errors.New("Microsoft Graph contacts authentication failed")
	ErrProtocol       = errors.New("Microsoft Graph contacts protocol failed")
	ErrSyncToken      = errors.New("Microsoft Graph contacts sync token expired")
	ErrAmbiguous      = errors.New("Microsoft Graph contact create outcome is ambiguous")
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
type folderResponse struct {
	Next  string        `json:"@odata.nextLink"`
	Value []graphFolder `json:"value"`
}
type graphFolder struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}
type contactResponse struct {
	Next  string         `json:"@odata.nextLink"`
	Delta string         `json:"@odata.deltaLink"`
	Value []graphContact `json:"value"`
}
type graphContact struct {
	ID               string           `json:"id,omitempty"`
	ETag             string           `json:"@odata.etag,omitempty"`
	Removed          json.RawMessage  `json:"@removed,omitempty"`
	DisplayName      string           `json:"displayName,omitempty"`
	GivenName        string           `json:"givenName,omitempty"`
	Surname          string           `json:"surname,omitempty"`
	CompanyName      string           `json:"companyName,omitempty"`
	JobTitle         string           `json:"jobTitle,omitempty"`
	EmailAddresses   []graphEmail     `json:"emailAddresses,omitempty"`
	BusinessPhones   []string         `json:"businessPhones,omitempty"`
	HomePhones       []string         `json:"homePhones,omitempty"`
	MobilePhone      string           `json:"mobilePhone,omitempty"`
	BusinessAddress  graphAddress     `json:"businessAddress,omitempty"`
	HomeAddress      graphAddress     `json:"homeAddress,omitempty"`
	OtherAddress     graphAddress     `json:"otherAddress,omitempty"`
	BusinessHomePage string           `json:"businessHomePage,omitempty"`
	NickName         string           `json:"nickName,omitempty"`
	Birthday         string           `json:"birthday,omitempty"`
	PersonalNotes    string           `json:"personalNotes,omitempty"`
	Extensions       []graphExtension `json:"extensions,omitempty"`
}
type graphEmail struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}
type graphAddress struct {
	Street     string `json:"street,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	Country    string `json:"countryOrRegion,omitempty"`
}
type graphExtension struct {
	ODataType     string `json:"@odata.type,omitempty"`
	ExtensionName string `json:"extensionName,omitempty"`
	ID            string `json:"id,omitempty"`
	Idempotency   string `json:"idempotencyKey,omitempty"`
}

func New(options Options) (*Connector, error) {
	if strings.TrimSpace(options.BaseURL) == "" {
		options.BaseURL = defaultBaseURL
	}
	base, err := url.Parse(options.BaseURL)
	if err != nil || !base.IsAbs() || base.User != nil || base.Hostname() == "" || base.RawQuery != "" || base.Fragment != "" || !secureOrLoopback(base) {
		return nil, errors.New("Microsoft Graph contacts API base URL is invalid")
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
	if connector == nil || connector.http == nil || account.Provider != accounts.ProviderMicrosoft || strings.TrimSpace(credentials.AccessToken) == "" {
		return contactsync.Snapshot{}, accounts.ErrUnsupported
	}
	root := connector.mailboxRoot(account.MailboxAddress)
	remoteFolders, err := connector.listFolders(ctx, root+"/contactFolders", credentials.AccessToken)
	if err != nil {
		return contactsync.Snapshot{}, classify(err)
	}
	books := []contactsync.AddressBook{{StableID: stableID(root + "/contacts"), URL: root + "/contacts", Name: "Contacts", Writable: true, CTag: "full-v1"}}
	for _, folder := range remoteFolders {
		bookURL := root + "/contactFolders/" + url.PathEscape(folder.ID) + "/contacts"
		books = append(books, contactsync.AddressBook{StableID: stableID(bookURL), URL: bookURL, Name: fallback(folder.DisplayName, "Contacts"), Writable: true})
	}
	previousBooks := map[string]contactsync.AddressBook{}
	previousContacts := map[string][]contactsync.Contact{}
	for _, book := range state.AddressBooks {
		previousBooks[book.URL] = book
	}
	for _, contact := range state.Contacts {
		previousContacts[contact.AddressBookID] = append(previousContacts[contact.AddressBookID], contact)
	}
	now := connector.clock().UTC()
	snapshot := contactsync.Snapshot{PrincipalURL: root, HomeURL: root + "/contactFolders", AddressBooks: books, SyncedAt: now}
	for index := range snapshot.AddressBooks {
		book := &snapshot.AddressBooks[index]
		contacts := map[string]contactsync.Contact{}
		previous := previousBooks[book.URL]
		if previous.SyncToken != "" {
			for _, contact := range previousContacts[previous.StableID] {
				contact.AddressBookID = book.StableID
				contacts[contact.Href] = contact
			}
		}
		var remote []graphContact
		var token string
		if book.CTag == "full-v1" {
			contacts = map[string]contactsync.Contact{}
			remote, err = connector.listContacts(ctx, book.URL, credentials.AccessToken, false)
		} else {
			target := book.URL + "/delta"
			if previous.SyncToken != "" {
				target = previous.SyncToken
			}
			remote, token, err = connector.delta(ctx, target, credentials.AccessToken)
			if errors.Is(err, ErrSyncToken) {
				contacts = map[string]contactsync.Contact{}
				remote, token, err = connector.delta(ctx, book.URL+"/delta", credentials.AccessToken)
			}
			book.SyncToken = token
		}
		if err != nil {
			return contactsync.Snapshot{}, classify(err)
		}
		for _, value := range remote {
			href := book.URL + "/" + url.PathEscape(value.ID)
			if len(value.Removed) > 0 {
				delete(contacts, href)
				continue
			}
			contact, projectErr := project(account.ID, book.StableID, href, value, now)
			if projectErr != nil {
				return contactsync.Snapshot{}, classify(projectErr)
			}
			contacts[href] = contact
		}
		for _, contact := range contacts {
			snapshot.Contacts = append(snapshot.Contacts, contact)
		}
		if len(snapshot.Contacts) > contactsync.MaxContacts {
			return contactsync.Snapshot{}, contactsync.ErrInvalidSnapshot
		}
	}
	sort.Slice(snapshot.AddressBooks, func(i, j int) bool { return snapshot.AddressBooks[i].StableID < snapshot.AddressBooks[j].StableID })
	sort.Slice(snapshot.Contacts, func(i, j int) bool { return snapshot.Contacts[i].ID < snapshot.Contacts[j].ID })
	return snapshot, nil
}

func (connector *Connector) ApplyMutation(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials, mutation contactsync.Mutation, current contactsync.Contact) (contactsync.MutationOutcome, error) {
	if account.Provider != accounts.ProviderMicrosoft || current.AccountID != account.ID || mutation.Scope.AccountID != account.ID || current.ID != mutation.ContactID || strings.TrimSpace(credentials.AccessToken) == "" {
		return contactsync.OutcomeConflict, nil
	}
	if mutation.Kind != contactsync.MutationCreate && mutation.ExpectedETag != current.ETag {
		return contactsync.OutcomeConflict, nil
	}
	switch mutation.Kind {
	case contactsync.MutationCreate:
		collection := parentURL(current.Href)
		marker := markerValue(mutation.IdempotencyKey)
		found, findErr := connector.findMarker(ctx, collection, credentials.AccessToken, marker)
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
		payload.Extensions = []graphExtension{{ODataType: "microsoft.graph.openTypeExtension", ExtensionName: extensionName, Idempotency: marker}}
		body, _ := json.Marshal(payload)
		_, status, callErr := connector.call(ctx, http.MethodPost, collection, credentials.AccessToken, "", body)
		if callErr == nil && status == http.StatusCreated {
			return contactsync.OutcomeApplied, nil
		}
		return "", classify(callErr)
	case contactsync.MutationUpdate:
		body, _ := json.Marshal(fromContact(mutation.Desired))
		_, status, callErr := connector.call(ctx, http.MethodPatch, current.Href, credentials.AccessToken, current.ETag, body)
		if callErr == nil && status == http.StatusOK {
			return contactsync.OutcomeApplied, nil
		}
		if status == http.StatusPreconditionFailed {
			return contactsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	case contactsync.MutationDelete:
		_, status, callErr := connector.call(ctx, http.MethodDelete, current.Href, credentials.AccessToken, current.ETag, nil)
		if callErr == nil && status == http.StatusNoContent || status == http.StatusNotFound && mutation.State == contactsync.MutationApplying {
			return contactsync.OutcomeApplied, nil
		}
		if status == http.StatusPreconditionFailed {
			return contactsync.OutcomeConflict, nil
		}
		return "", classify(callErr)
	default:
		return "", contactsync.ErrInvalidMutation
	}
}

func (connector *Connector) listFolders(ctx context.Context, target, token string) ([]graphFolder, error) {
	result := []graphFolder{}
	next := target + "?$top=100"
	for page := 0; page < 100 && next != ""; page++ {
		body, _, err := connector.call(ctx, http.MethodGet, next, token, "", nil)
		if err != nil {
			return nil, err
		}
		var response folderResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, ErrProtocol
		}
		for _, folder := range response.Value {
			if folder.ID != "" {
				result = append(result, folder)
			}
		}
		if len(result)+1 > contactsync.MaxAddressBooks {
			return nil, contactsync.ErrInvalidSnapshot
		}
		next = response.Next
	}
	if next != "" {
		return nil, ErrProtocol
	}
	return result, nil
}

func (connector *Connector) listContacts(ctx context.Context, target, token string, expand bool) ([]graphContact, error) {
	next := target + "?$top=100"
	if expand {
		next += "&$expand=" + url.QueryEscape("extensions($filter=id eq '"+extensionName+"')")
	}
	result := []graphContact{}
	for page := 0; page < 100 && next != ""; page++ {
		body, _, err := connector.call(ctx, http.MethodGet, next, token, "", nil)
		if err != nil {
			return nil, err
		}
		var response contactResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, ErrProtocol
		}
		result = append(result, response.Value...)
		if len(result) > contactsync.MaxContacts {
			return nil, contactsync.ErrInvalidSnapshot
		}
		next = response.Next
	}
	if next != "" {
		return nil, ErrProtocol
	}
	return result, nil
}

func (connector *Connector) delta(ctx context.Context, target, token string) ([]graphContact, string, error) {
	result := []graphContact{}
	next := target
	for page := 0; page < 100 && next != ""; page++ {
		body, status, err := connector.call(ctx, http.MethodGet, next, token, "", nil)
		if status == http.StatusGone {
			return nil, "", ErrSyncToken
		}
		if err != nil {
			return nil, "", err
		}
		var response contactResponse
		if json.Unmarshal(body, &response) != nil {
			return nil, "", ErrProtocol
		}
		result = append(result, response.Value...)
		if len(result) > contactsync.MaxContacts {
			return nil, "", contactsync.ErrInvalidSnapshot
		}
		if response.Next != "" {
			next = response.Next
			continue
		}
		if response.Delta == "" {
			return nil, "", ErrProtocol
		}
		return result, response.Delta, nil
	}
	return nil, "", ErrProtocol
}

func (connector *Connector) findMarker(ctx context.Context, collection, token, marker string) (bool, error) {
	contacts, err := connector.listContacts(ctx, collection, token, true)
	if err != nil {
		return false, err
	}
	for _, contact := range contacts {
		for _, extension := range contact.Extensions {
			if (extension.ID == extensionName || extension.ExtensionName == extensionName) && extension.Idempotency == marker {
				return true, nil
			}
		}
	}
	return false, nil
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

func project(accountID accounts.AccountID, bookID, href string, remote graphContact, now time.Time) (contactsync.Contact, error) {
	contact := toContact(remote)
	contact.ID, contact.AccountID, contact.AddressBookID, contact.Href, contact.ETag, contact.UID, contact.RemoteUpdatedAt = contactsync.RemoteContactID(accountID, href), accountID, bookID, href, remote.ETag, remote.ID, now
	raw, err := contactapi.RenderVCard(contact)
	if err != nil {
		return contactsync.Contact{}, err
	}
	contact.RawVCard = raw
	return contact, nil
}

func toContact(remote graphContact) contactsync.Contact {
	contact := contactsync.Contact{Name: remote.DisplayName, GivenName: remote.GivenName, FamilyName: remote.Surname, Company: remote.CompanyName, Title: remote.JobTitle, Notes: remote.PersonalNotes}
	for _, value := range remote.EmailAddresses {
		contact.Emails = append(contact.Emails, contactsync.Value{Value: value.Address})
	}
	for _, value := range remote.BusinessPhones {
		contact.Phones = append(contact.Phones, contactsync.Value{Value: value, Type: "work"})
	}
	for _, value := range remote.HomePhones {
		contact.Phones = append(contact.Phones, contactsync.Value{Value: value, Type: "home"})
	}
	if remote.MobilePhone != "" {
		contact.Phones = append(contact.Phones, contactsync.Value{Value: remote.MobilePhone, Type: "mobile"})
	}
	for _, value := range []struct {
		typeName string
		address  graphAddress
	}{{"work", remote.BusinessAddress}, {"home", remote.HomeAddress}, {"other", remote.OtherAddress}} {
		if value.address != (graphAddress{}) {
			contact.Addresses = append(contact.Addresses, contactsync.Address{StreetAddress: singleLine(value.address.Street), City: value.address.City, Region: value.address.State, PostalCode: value.address.PostalCode, Country: value.address.Country, Type: value.typeName})
		}
	}
	if remote.BusinessHomePage != "" {
		contact.URLs = []contactsync.Value{{Value: remote.BusinessHomePage, Type: "work"}}
	}
	if remote.NickName != "" {
		contact.Nicknames = []string{remote.NickName}
	}
	if len(remote.Birthday) >= 10 {
		contact.Birthday = remote.Birthday[:10]
	}
	return contact
}

func fromContact(contact contactsync.Contact) graphContact {
	result := graphContact{DisplayName: contact.Name, GivenName: contact.GivenName, Surname: contact.FamilyName, CompanyName: contact.Company, JobTitle: contact.Title, PersonalNotes: contact.Notes}
	for _, value := range contact.Emails {
		result.EmailAddresses = append(result.EmailAddresses, graphEmail{Address: value.Value, Name: contact.Name})
	}
	for _, value := range contact.Phones {
		switch strings.ToLower(value.Type) {
		case "home":
			result.HomePhones = append(result.HomePhones, value.Value)
		case "mobile", "cell":
			if result.MobilePhone == "" {
				result.MobilePhone = value.Value
			} else {
				result.BusinessPhones = append(result.BusinessPhones, value.Value)
			}
		default:
			result.BusinessPhones = append(result.BusinessPhones, value.Value)
		}
	}
	for _, value := range contact.Addresses {
		address := graphAddress{Street: joinStreet(value), City: value.City, State: value.Region, PostalCode: value.PostalCode, Country: value.Country}
		switch strings.ToLower(value.Type) {
		case "home":
			result.HomeAddress = address
		case "work", "business":
			result.BusinessAddress = address
		default:
			result.OtherAddress = address
		}
	}
	if len(contact.URLs) > 0 {
		result.BusinessHomePage = contact.URLs[0].Value
	}
	if len(contact.Nicknames) > 0 {
		result.NickName = contact.Nicknames[0]
	}
	if contact.Birthday != "" && !strings.HasPrefix(contact.Birthday, "--") {
		result.Birthday = contact.Birthday + "T00:00:00Z"
	}
	return result
}

func joinStreet(value contactsync.Address) string {
	if value.ExtendedAddress == "" {
		return value.StreetAddress
	}
	if value.StreetAddress == "" {
		return value.ExtendedAddress
	}
	return value.ExtendedAddress + "\n" + value.StreetAddress
}
func singleLine(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", ", ", "\r", ", ", "\n", ", ").Replace(value))
}
func stableID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "book_" + base64.RawURLEncoding.EncodeToString(digest[:18])
}
func markerValue(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:18])
}
func parentURL(target string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		parsed.Path, parsed.RawPath, parsed.RawQuery = path[:slash], "", ""
		return parsed.String()
	}
	return ""
}
func (connector *Connector) mailboxRoot(mailbox string) string {
	return strings.TrimSuffix(connector.base.String(), "/") + "/users/" + url.PathEscape(strings.ToLower(strings.TrimSpace(mailbox)))
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
func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAuthentication) {
		return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "microsoft_contacts_authentication_failed", Message: "Microsoft Contacts authorization expired; reconnect the account."}
	}
	if errors.Is(err, contactapi.ErrInvalidContact) || errors.Is(err, contactsync.ErrInvalidSnapshot) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "microsoft_contacts_invalid_data", Message: "Microsoft Contacts returned invalid contact data."}
	}
	if errors.Is(err, ErrAmbiguous) {
		return accountsync.ConnectorError{Kind: accountsync.FailurePermanent, Code: "microsoft_contacts_create_ambiguous", Message: "Microsoft Contacts has not yet confirmed the contact creation; retry after provider synchronization completes."}
	}
	return accountsync.ConnectorError{Kind: accountsync.FailureTransient, Code: "microsoft_contacts_sync_failed", Message: "Microsoft Contacts synchronization failed."}
}
