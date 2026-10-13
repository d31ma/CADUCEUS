// Package carddav implements bounded CardDAV discovery and synchronization.
// It owns only protocol concerns; durable tenant isolation and conflict
// journals live behind the contactsync store port.
package carddav

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
	"github.com/d31ma/caduceus/internal/contactsync"
)

const (
	StatusMultiStatus       = 207
	defaultMaxResponseBytes = int64(8 << 20)
	maximumAddressBooks     = 100
	maximumContacts         = 5000
)

var (
	ErrAuthentication   = errors.New("CardDAV authentication failed")
	ErrProtocol         = errors.New("CardDAV protocol failed")
	ErrResponseTooLarge = errors.New("CardDAV response exceeds limit")
	ErrUnsafeEndpoint   = errors.New("unsafe CardDAV endpoint")
	ErrSyncTokenInvalid = errors.New("CardDAV sync token is invalid")
	ErrPrecondition     = errors.New("CardDAV resource changed remotely")
)

type Options struct {
	HTTPClient       *http.Client
	MaxResponseBytes int64
	Resolver         IPResolver
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
	PrincipalURL string        `json:"principalUrl"`
	HomeURL      string        `json:"homeUrl"`
	AddressBooks []AddressBook `json:"addressBooks"`
}

type AddressBook struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	SyncToken string `json:"syncToken,omitempty"`
	CTag      string `json:"ctag,omitempty"`
}

type RemoteContact struct {
	Href  string `json:"href"`
	ETag  string `json:"etag"`
	VCard string `json:"vcard"`
}

type Delta struct {
	SyncToken string
	Changed   []RemoteContact
	Deleted   []string
}

func (client *Client) Fetch(ctx context.Context, target string, auth Auth) (RemoteContact, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return RemoteContact{}, ErrUnsafeEndpoint
	}
	request.Header.Set("Accept", "text/vcard,text/x-vcard;q=0.9")
	response, err := client.doResourceRequest(request, auth)
	if err != nil {
		return RemoteContact{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RemoteContact{}, ErrProtocol
	}
	limit := int64(contactsync.MaxVCardBytes)
	if client.maxBytes < limit {
		limit = client.maxBytes
	}
	if response.ContentLength > limit {
		return RemoteContact{}, ErrResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return RemoteContact{}, ErrProtocol
	}
	if int64(len(payload)) > limit || len(payload) == 0 {
		return RemoteContact{}, ErrResponseTooLarge
	}
	if response.Request == nil || response.Request.URL == nil {
		return RemoteContact{}, ErrProtocol
	}
	return RemoteContact{Href: response.Request.URL.String(), ETag: strings.TrimSpace(response.Header.Get("ETag")), VCard: string(payload)}, nil
}

func (client *Client) Put(ctx context.Context, target, vcard, expectedETag string, create bool, auth Auth) (string, error) {
	if len(vcard) == 0 || len(vcard) > contactsync.MaxVCardBytes {
		return "", ErrProtocol
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target, strings.NewReader(vcard))
	if err != nil {
		return "", ErrUnsafeEndpoint
	}
	request.Header.Set("Content-Type", "text/vcard; charset=utf-8")
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
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/xml,text/plain;q=0.5")
	}
	request.Header.Set("User-Agent", "CADUCEUS-CardDAV/1")
	request.SetBasicAuth(auth.Username, auth.Password)
	request = request.WithContext(withInsecureTLS(request.Context(), auth.AllowInsecureTLS))
	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, ErrUnsafeEndpoint) {
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
	client := options.HTTPClient
	if client == nil {
		var err error
		client, err = newSafeHTTPClient(options.Resolver, options.PrivateCIDRs, options.Timeout)
		if err != nil {
			return nil, err
		}
	}
	limit := options.MaxResponseBytes
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	return &Client{httpClient: client, maxBytes: limit}, nil
}

func (client *Client) Discover(ctx context.Context, endpoint *accounts.CardDAVEndpoint, credentials accounts.Credentials) (Discovery, error) {
	if client == nil || endpoint == nil || client.httpClient == nil {
		return Discovery{}, ErrUnsafeEndpoint
	}
	bootstrap, err := parseEndpoint(endpoint.URL)
	if err != nil {
		return Discovery{}, err
	}
	if bootstrap.Path == "" || bootstrap.Path == "/" {
		bootstrap.Path = "/.well-known/carddav"
	}
	auth := Auth{Username: strings.TrimSpace(endpoint.Username), Password: credentials.Password, AllowInsecureTLS: endpoint.AllowInsecureTLS}
	if auth.Username == "" {
		auth.Username = strings.TrimSpace(credentials.Username)
	}
	if auth.Username == "" || auth.Password == "" {
		return Discovery{}, ErrAuthentication
	}

	bootstrapResponses, effective, err := client.request(ctx, "PROPFIND", bootstrap.String(), "0", currentPrincipalRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	principalRef := firstSuccessfulProperty(bootstrapResponses, func(property davProperty) string { return property.CurrentUserPrincipal.Href })
	principalURL, err := resolveReference(effective, principalRef)
	if err != nil {
		return Discovery{}, ErrProtocol
	}
	principalResponses, principalEffective, err := client.request(ctx, "PROPFIND", principalURL, "0", addressBookHomeRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	homeRef := firstSuccessfulProperty(principalResponses, func(property davProperty) string { return property.AddressBookHomeSet.Href })
	homeURL, err := resolveReference(principalEffective, homeRef)
	if err != nil {
		return Discovery{}, ErrProtocol
	}
	homeResponses, homeEffective, err := client.request(ctx, "PROPFIND", homeURL, "1", addressBookListRequest, auth)
	if err != nil {
		return Discovery{}, err
	}
	books := make([]AddressBook, 0)
	for _, response := range homeResponses {
		for _, property := range successfulProperties(response) {
			if !property.ResourceType.AddressBook {
				continue
			}
			bookURL, resolveErr := resolveReference(homeEffective, response.Href)
			if resolveErr != nil {
				return Discovery{}, ErrProtocol
			}
			name := strings.TrimSpace(property.DisplayName)
			if name == "" {
				name = "Contacts"
			}
			books = append(books, AddressBook{URL: bookURL, Name: name, SyncToken: strings.TrimSpace(property.SyncToken), CTag: strings.TrimSpace(property.CTag)})
			if len(books) > maximumAddressBooks {
				return Discovery{}, ErrProtocol
			}
		}
	}
	if len(books) == 0 {
		return Discovery{}, ErrProtocol
	}
	return Discovery{PrincipalURL: principalURL, HomeURL: homeURL, AddressBooks: books}, nil
}

func (client *Client) InitialSync(ctx context.Context, book AddressBook, auth Auth) ([]RemoteContact, error) {
	responses, effective, err := client.request(ctx, "REPORT", book.URL, "1", addressBookQueryRequest, auth)
	if err != nil {
		return nil, err
	}
	contacts := make([]RemoteContact, 0)
	for _, response := range responses {
		for _, property := range successfulProperties(response) {
			if strings.TrimSpace(property.AddressData) == "" {
				continue
			}
			href, resolveErr := resolveReference(effective, response.Href)
			if resolveErr != nil {
				return nil, ErrProtocol
			}
			contacts = append(contacts, RemoteContact{Href: href, ETag: strings.TrimSpace(property.GetETag), VCard: property.AddressData})
			if len(contacts) > maximumContacts {
				return nil, ErrProtocol
			}
		}
	}
	return contacts, nil
}

// IncrementalSync applies RFC 6578 sync-collection and retrieves bodies only
// for changed hrefs. Callers fall back to InitialSync when the server rejects
// an expired token.
func (client *Client) IncrementalSync(ctx context.Context, book AddressBook, previousToken string, auth Auth) (Delta, error) {
	if strings.TrimSpace(previousToken) == "" {
		return Delta{}, ErrSyncTokenInvalid
	}
	body := syncCollectionRequest(strings.TrimSpace(previousToken))
	document, effective, err := client.requestDocument(ctx, "REPORT", book.URL, "1", body, auth)
	if err != nil {
		var status davStatusError
		if errors.As(err, &status) {
			switch status.Code {
			case http.StatusUnauthorized:
				return Delta{}, ErrAuthentication
			case http.StatusForbidden, http.StatusConflict:
				return Delta{}, ErrSyncTokenInvalid
			}
		}
		return Delta{}, err
	}
	delta := Delta{SyncToken: strings.TrimSpace(document.SyncToken)}
	if delta.SyncToken == "" {
		return Delta{}, ErrProtocol
	}
	type changedReference struct{ raw, canonical string }
	changedHrefs := make([]changedReference, 0)
	seen := map[string]bool{}
	for _, response := range document.Responses {
		href := strings.TrimSpace(response.Href)
		if href == "" || seen[href] {
			continue
		}
		canonical, resolveErr := resolveReference(effective, href)
		if resolveErr != nil {
			return Delta{}, ErrProtocol
		}
		if strings.Contains(response.Status, " 404 ") {
			delta.Deleted = append(delta.Deleted, canonical)
			seen[href] = true
			continue
		}
		for _, property := range successfulProperties(response) {
			if strings.TrimSpace(property.GetETag) != "" {
				changedHrefs = append(changedHrefs, changedReference{raw: href, canonical: canonical})
				seen[href] = true
				break
			}
		}
		if len(changedHrefs)+len(delta.Deleted) > maximumContacts {
			return Delta{}, ErrProtocol
		}
	}
	for start := 0; start < len(changedHrefs); start += 100 {
		end := start + 100
		if end > len(changedHrefs) {
			end = len(changedHrefs)
		}
		rawHrefs := make([]string, 0, end-start)
		for _, reference := range changedHrefs[start:end] {
			rawHrefs = append(rawHrefs, reference.raw)
		}
		responses, multigetEffective, requestErr := client.request(ctx, "REPORT", effective, "1", addressBookMultigetRequest(rawHrefs), auth)
		if requestErr != nil {
			return Delta{}, requestErr
		}
		for _, response := range responses {
			for _, property := range successfulProperties(response) {
				if strings.TrimSpace(property.AddressData) != "" {
					href, resolveErr := resolveReference(multigetEffective, response.Href)
					if resolveErr != nil {
						return Delta{}, ErrProtocol
					}
					delta.Changed = append(delta.Changed, RemoteContact{Href: href, ETag: strings.TrimSpace(property.GetETag), VCard: property.AddressData})
				}
			}
		}
	}
	if len(delta.Changed) != len(changedHrefs) {
		return Delta{}, ErrProtocol
	}
	return delta, nil
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
	request.Header.Set("User-Agent", "CADUCEUS-CardDAV/1")
	request.SetBasicAuth(auth.Username, auth.Password)
	request = request.WithContext(withInsecureTLS(request.Context(), auth.AllowInsecureTLS))
	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, ErrUnsafeEndpoint) {
			return davMultistatus{}, "", ErrUnsafeEndpoint
		}
		return davMultistatus{}, "", fmt.Errorf("%w: request", ErrProtocol)
	}
	defer response.Body.Close()
	if response.StatusCode != StatusMultiStatus {
		return davMultistatus{}, "", davStatusError{Code: response.StatusCode}
	}
	if response.ContentLength > client.maxBytes {
		return davMultistatus{}, "", ErrResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, client.maxBytes+1))
	if err != nil {
		return davMultistatus{}, "", ErrProtocol
	}
	if int64(len(payload)) > client.maxBytes {
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
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || parsed.Fragment != "" ||
		parsed.Scheme != "https" {
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

type davStatusError struct{ Code int }

func (failure davStatusError) Error() string {
	return fmt.Sprintf("CardDAV HTTP status %d", failure.Code)
}

type davPropstat struct {
	Property davProperty `xml:"prop"`
	Status   string      `xml:"status"`
}

type davProperty struct {
	CurrentUserPrincipal hrefProperty `xml:"current-user-principal"`
	AddressBookHomeSet   hrefProperty `xml:"addressbook-home-set"`
	DisplayName          string       `xml:"displayname"`
	ResourceType         resourceType `xml:"resourcetype"`
	SyncToken            string       `xml:"sync-token"`
	CTag                 string       `xml:"getctag"`
	GetETag              string       `xml:"getetag"`
	AddressData          string       `xml:"address-data"`
}

type hrefProperty struct {
	Href string `xml:"href"`
}

type resourceType struct {
	AddressBook bool
}

func (value *resourceType) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			if current.Name.Local == "addressbook" {
				value.AddressBook = true
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

const currentPrincipalRequest = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`

const addressBookHomeRequest = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav"><d:prop><card:addressbook-home-set/></d:prop></d:propfind>`

const addressBookListRequest = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/"><d:prop><d:displayname/><d:resourcetype/><d:sync-token/><cs:getctag/></d:prop></d:propfind>`

const addressBookQueryRequest = `<?xml version="1.0" encoding="utf-8"?>
<card:addressbook-query xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav"><d:prop><d:getetag/><card:address-data/></d:prop><card:filter><card:prop-filter name="FN"/></card:filter></card:addressbook-query>`

func syncCollectionRequest(token string) string {
	return `<?xml version="1.0" encoding="utf-8"?><d:sync-collection xmlns:d="DAV:"><d:sync-token>` + xmlText(token) + `</d:sync-token><d:sync-level>1</d:sync-level><d:prop><d:getetag/></d:prop></d:sync-collection>`
}

func addressBookMultigetRequest(hrefs []string) string {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="utf-8"?><card:addressbook-multiget xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav"><d:prop><d:getetag/><card:address-data/></d:prop>`)
	for _, href := range hrefs {
		body.WriteString("<d:href>")
		body.WriteString(xmlText(href))
		body.WriteString("</d:href>")
	}
	body.WriteString(`</card:addressbook-multiget>`)
	return body.String()
}

func xmlText(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}
