// Package accounts owns optional external mailbox account configuration and
// synchronization state. It contains no provider, transport, or persistence
// implementation.
package accounts

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type AccountID string
type Provider string
type State string
type SecurityMode string
type SyncPhase string
type ValidationService string
type ValidationCode string

// Owner is the SESAME identity that owns an external account. Address is
// display metadata; TenantID and PrincipalID form the authorization key.
type Owner struct {
	TenantID    string `json:"-"`
	PrincipalID string `json:"-"`
	Address     string `json:"-"`
}

const (
	ProviderGmail     Provider = "gmail"
	ProviderMicrosoft Provider = "microsoft"
	ProviderIMAP      Provider = "imap"

	StateReady        State = "ready"
	StateSyncing      State = "syncing"
	StateIdle         State = "idle"
	StateAuthRequired State = "auth_required"
	StateDegraded     State = "degraded"
	StatePaused       State = "paused"
	StateDeleting     State = "deleting"

	SecurityTLS      SecurityMode = "tls"
	SecurityStartTLS SecurityMode = "starttls"
	SecurityNone     SecurityMode = "none"

	PhaseConnecting SyncPhase = "connecting"
	PhaseFolders    SyncPhase = "folders"
	PhaseHeaders    SyncPhase = "headers"
	PhaseBodies     SyncPhase = "bodies"
	PhaseContacts   SyncPhase = "contacts"
	PhaseCalendar   SyncPhase = "calendar"
	PhaseComplete   SyncPhase = "complete"

	ValidationIMAP    ValidationService = "imap"
	ValidationSMTP    ValidationService = "smtp"
	ValidationCardDAV ValidationService = "carddav"
	ValidationCalDAV  ValidationService = "caldav"

	ValidationConnection     ValidationCode = "connection_failed"
	ValidationAuthentication ValidationCode = "authentication_failed"
	ValidationMailbox        ValidationCode = "mailbox_missing"
	ValidationProtocol       ValidationCode = "protocol_failed"
)

var (
	ErrInvalid           = errors.New("invalid external account")
	ErrNotFound          = errors.New("external account not found")
	ErrConflict          = errors.New("external account revision conflict")
	ErrUnsupported       = errors.New("external account provider is not configured")
	ErrInvalidTransition = errors.New("invalid external account state transition")

	hexColor    = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	hostName    = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	failureCode = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)
)

// ValidationFailure is intentionally detail-free: provider banners and
// authentication responses are untrusted and can reflect submitted secrets.
type ValidationFailure struct {
	Service ValidationService
	Code    ValidationCode
}

func (failure ValidationFailure) Error() string {
	return fmt.Sprintf("%s validation failed: %s", failure.Service, failure.Code)
}

func IsValidationFailure(err error, service ValidationService, code ValidationCode) bool {
	var failure ValidationFailure
	return errors.As(err, &failure) && failure.Service == service && failure.Code == code
}

// Endpoint is one operator-confirmed IMAP or SMTP endpoint. Network policy is
// enforced again by the adapter when it resolves and connects.
type Endpoint struct {
	Host             string       `json:"host"`
	Port             uint16       `json:"port"`
	Security         SecurityMode `json:"security"`
	Username         string       `json:"username,omitempty"`
	AllowInsecureTLS bool         `json:"allowInsecureTLS"`
}

// CardDAVEndpoint is the operator-confirmed bootstrap URL for an optional
// address-book service. Credentials are never stored here: CardDAV reuses the
// external account's encrypted credential envelope unless Username overrides
// only the authentication identity.
type CardDAVEndpoint struct {
	URL              string `json:"url"`
	Username         string `json:"username,omitempty"`
	AllowInsecureTLS bool   `json:"allowInsecureTLS"`
}

// CalDAVEndpoint is the operator-confirmed bootstrap URL for an optional
// calendar service. It reuses the account's encrypted password/app password.
type CalDAVEndpoint struct {
	URL              string `json:"url"`
	Username         string `json:"username,omitempty"`
	AllowInsecureTLS bool   `json:"allowInsecureTLS"`
}

type ConnectionSettings struct {
	IMAP            *Endpoint        `json:"imap,omitempty"`
	SMTP            *Endpoint        `json:"smtp,omitempty"`
	CardDAV         *CardDAVEndpoint `json:"carddav,omitempty"`
	CalDAV          *CalDAVEndpoint  `json:"caldav,omitempty"`
	ContainerFolder string           `json:"containerFolder,omitempty"`
}

// Credentials is intentionally not JSON serializable. It exists only between
// the authenticated request boundary, provider validator, and SecretVault.
type Credentials struct {
	Username     string    `json:"-"`
	Password     string    `json:"-"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
}

type Capabilities struct {
	MailRead      bool `json:"mailRead"`
	MailWrite     bool `json:"mailWrite"`
	Send          bool `json:"send"`
	Contacts      bool `json:"contacts"`
	Calendar      bool `json:"calendar"`
	Labels        bool `json:"labels"`
	SharedMailbox bool `json:"sharedMailbox"`
}

type SyncProgress struct {
	Phase     SyncPhase `json:"phase"`
	Completed uint64    `json:"completed"`
	Total     *uint64   `json:"total"`
	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Failure struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Retryable  bool       `json:"retryable"`
	OccurredAt time.Time  `json:"occurredAt"`
	RetryAt    *time.Time `json:"retryAt"`
}

// SyncProgressUpdate is the worker-owned, secret-free progress projection.
// Total is optional because providers do not always expose a stable count.
type SyncProgressUpdate struct {
	Phase     SyncPhase
	Completed uint64
	Total     *uint64
}

// SyncFailure is the classified worker result accepted by the account domain.
// Message must already be safe for operators and end users to see.
type SyncFailure struct {
	Code           string
	Message        string
	Retryable      bool
	Authentication bool
	RetryAt        *time.Time
}

// ExternalAccount is the redacted public and durable account projection.
// Owner participates in every repository key but is never returned as JSON.
type ExternalAccount struct {
	ID                   AccountID          `json:"id"`
	Owner                Owner              `json:"-"`
	Provider             Provider           `json:"provider"`
	MailboxAddress       string             `json:"mailboxAddress"`
	AuthIdentity         *string            `json:"authIdentity"`
	SharedMailboxAddress *string            `json:"sharedMailboxAddress"`
	Label                string             `json:"label"`
	Color                string             `json:"color"`
	Order                uint16             `json:"order"`
	State                State              `json:"state"`
	Connection           ConnectionSettings `json:"connection"`
	Capabilities         Capabilities       `json:"capabilities"`
	Progress             *SyncProgress      `json:"progress"`
	Failure              *Failure           `json:"failure"`
	Revision             uint64             `json:"revision"`
	CreatedAt            time.Time          `json:"createdAt"`
	UpdatedAt            time.Time          `json:"updatedAt"`
}

type EncryptedSecret struct {
	KeyVersion uint32 `json:"keyVersion"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type CreateRequest struct {
	Provider             Provider
	MailboxAddress       string
	AuthIdentity         string
	SharedMailboxAddress string
	Label                string
	Color                string
	Order                uint16
	ContainerFolder      string
	Credentials          Credentials
	IMAP                 *Endpoint
	SMTP                 *Endpoint
	CardDAV              *CardDAVEndpoint
	CalDAV               *CalDAVEndpoint
}

type ValidationRequest struct {
	Provider             Provider
	MailboxAddress       string
	AuthIdentity         string
	SharedMailboxAddress string
	Credentials          Credentials
	IMAP                 *Endpoint
	SMTP                 *Endpoint
	CardDAV              *CardDAVEndpoint
	CalDAV               *CalDAVEndpoint
}

// PresentationUpdate changes only browser-facing account metadata. Pointer
// fields distinguish an omitted value from the valid zero order.
type PresentationUpdate struct {
	Label *string
	Color *string
	Order *uint16
}

func normalizePresentationUpdate(update PresentationUpdate) (PresentationUpdate, error) {
	if update.Label == nil && update.Color == nil && update.Order == nil {
		return PresentationUpdate{}, ErrInvalid
	}
	if update.Label != nil {
		label := strings.TrimSpace(*update.Label)
		if label == "" || len(label) > 64 || strings.ContainsAny(label, "\r\n\x00") {
			return PresentationUpdate{}, ErrInvalid
		}
		update.Label = &label
	}
	if update.Color != nil {
		color := strings.TrimSpace(*update.Color)
		if !hexColor.MatchString(color) {
			return PresentationUpdate{}, ErrInvalid
		}
		update.Color = &color
	}
	if update.Order != nil && *update.Order > 999 {
		return PresentationUpdate{}, ErrInvalid
	}
	return update, nil
}

func normalizeCreate(owner Owner, request CreateRequest) (Owner, CreateRequest, error) {
	normalizedOwner, err := normalizeOwner(owner)
	if err != nil {
		return Owner{}, CreateRequest{}, err
	}
	request.MailboxAddress, err = normalizeAddress(request.MailboxAddress)
	if err != nil {
		return Owner{}, CreateRequest{}, fmt.Errorf("%w: mailbox address", ErrInvalid)
	}
	request.Label = strings.TrimSpace(request.Label)
	request.Color = strings.TrimSpace(request.Color)
	request.ContainerFolder = strings.TrimSpace(request.ContainerFolder)
	if !validProvider(request.Provider) || request.Label == "" || len(request.Label) > 64 || !hexColor.MatchString(request.Color) || request.Order > 999 {
		return Owner{}, CreateRequest{}, ErrInvalid
	}
	if len(request.ContainerFolder) > 255 || strings.ContainsAny(request.ContainerFolder, "\r\n\x00") {
		return Owner{}, CreateRequest{}, fmt.Errorf("%w: container folder", ErrInvalid)
	}
	if request.AuthIdentity != "" {
		request.AuthIdentity, err = normalizeAddress(request.AuthIdentity)
		if err != nil {
			return Owner{}, CreateRequest{}, fmt.Errorf("%w: authentication identity", ErrInvalid)
		}
	}
	if request.SharedMailboxAddress != "" {
		request.SharedMailboxAddress, err = normalizeAddress(request.SharedMailboxAddress)
		if err != nil || request.Provider != ProviderMicrosoft || request.AuthIdentity == "" {
			return Owner{}, CreateRequest{}, fmt.Errorf("%w: shared mailbox", ErrInvalid)
		}
		request.MailboxAddress = request.SharedMailboxAddress
	}
	if request.Provider == ProviderIMAP {
		if err := validateEndpoint(request.IMAP); err != nil {
			return Owner{}, CreateRequest{}, err
		}
		if err := validateEndpoint(request.SMTP); err != nil {
			return Owner{}, CreateRequest{}, err
		}
		if request.CardDAV != nil {
			if err := validateCardDAVEndpoint(request.CardDAV); err != nil {
				return Owner{}, CreateRequest{}, err
			}
		}
		if request.CalDAV != nil {
			if err := validateCalDAVEndpoint(request.CalDAV); err != nil {
				return Owner{}, CreateRequest{}, err
			}
		}
	}
	return normalizedOwner, request, nil
}

func normalizeOwner(owner Owner) (Owner, error) {
	owner.TenantID = strings.TrimSpace(owner.TenantID)
	owner.PrincipalID = strings.TrimSpace(owner.PrincipalID)
	address, err := normalizeAddress(owner.Address)
	if err != nil || owner.TenantID == "" || owner.PrincipalID == "" || len(owner.TenantID) > 128 || len(owner.PrincipalID) > 128 {
		return Owner{}, fmt.Errorf("%w: owner identity", ErrInvalid)
	}
	owner.Address = address
	return owner, nil
}

func sameOwner(left, right Owner) bool {
	return left.TenantID == right.TenantID && left.PrincipalID == right.PrincipalID
}

func normalizeAddress(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed || len(trimmed) > 254 {
		return "", ErrInvalid
	}
	parts := strings.Split(parsed.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return "", ErrInvalid
	}
	return strings.ToLower(parsed.Address), nil
}

func validateEndpoint(endpoint *Endpoint) error {
	if endpoint == nil {
		return fmt.Errorf("%w: endpoint is required", ErrInvalid)
	}
	endpoint.Host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(endpoint.Host), "."))
	if endpoint.Host == "" || endpoint.Port == 0 ||
		(endpoint.Security != SecurityTLS && endpoint.Security != SecurityStartTLS && endpoint.Security != SecurityNone) {
		return fmt.Errorf("%w: endpoint", ErrInvalid)
	}
	if net.ParseIP(endpoint.Host) == nil && (!hostName.MatchString(endpoint.Host) || !strings.Contains(endpoint.Host, ".")) {
		return fmt.Errorf("%w: endpoint host", ErrInvalid)
	}
	return nil
}

func validateCardDAVEndpoint(endpoint *CardDAVEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("%w: CardDAV endpoint", ErrInvalid)
	}
	normalizedURL, normalizedUsername, err := normalizeDAVEndpoint(endpoint.URL, endpoint.Username, "CardDAV")
	if err != nil {
		return err
	}
	endpoint.URL, endpoint.Username = normalizedURL, normalizedUsername
	return nil
}

func validateCalDAVEndpoint(endpoint *CalDAVEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("%w: CalDAV endpoint", ErrInvalid)
	}
	normalizedURL, normalizedUsername, err := normalizeDAVEndpoint(endpoint.URL, endpoint.Username, "CalDAV")
	if err != nil {
		return err
	}
	endpoint.URL, endpoint.Username = normalizedURL, normalizedUsername
	return nil
}

func normalizeDAVEndpoint(rawURL, rawUsername, service string) (string, string, error) {
	if len(rawURL) > 2048 {
		return "", "", fmt.Errorf("%w: %s endpoint", ErrInvalid, service)
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || parsed.Fragment != "" || parsed.Scheme != "https" {
		return "", "", fmt.Errorf("%w: %s endpoint", ErrInvalid, service)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Host = strings.ToLower(parsed.Host)
	username := strings.TrimSpace(rawUsername)
	if len(username) > 1024 || strings.ContainsAny(username, "\r\n\x00") {
		return "", "", fmt.Errorf("%w: %s username", ErrInvalid, service)
	}
	return parsed.String(), username, nil
}

func validProvider(provider Provider) bool {
	return provider == ProviderGmail || provider == ProviderMicrosoft || provider == ProviderIMAP
}

func syncPhaseOrder(phase SyncPhase) (int, bool) {
	order := map[SyncPhase]int{
		PhaseConnecting: 0,
		PhaseFolders:    1,
		PhaseHeaders:    2,
		PhaseBodies:     3,
		PhaseContacts:   4,
		PhaseCalendar:   5,
		PhaseComplete:   6,
	}
	value, found := order[phase]
	return value, found
}

// Transition applies the explicit durable lifecycle. A state not listed for
// the current state is rejected rather than inferred.
func Transition(current, target State) (State, error) {
	allowed := map[State]map[State]bool{
		StateReady:        {StateSyncing: true, StatePaused: true, StateAuthRequired: true, StateDegraded: true, StateDeleting: true},
		StateSyncing:      {StateIdle: true, StatePaused: true, StateAuthRequired: true, StateDegraded: true, StateDeleting: true},
		StateIdle:         {StateSyncing: true, StatePaused: true, StateAuthRequired: true, StateDegraded: true, StateDeleting: true},
		StateAuthRequired: {StateReady: true, StateDeleting: true},
		StateDegraded:     {StateSyncing: true, StatePaused: true, StateAuthRequired: true, StateDeleting: true},
		StatePaused:       {StateReady: true, StateDeleting: true},
		StateDeleting:     {},
	}
	if !allowed[current][target] {
		return current, fmt.Errorf("%w: %s to %s", ErrInvalidTransition, current, target)
	}
	return target, nil
}
