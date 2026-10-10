package accounts

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

type Repository interface {
	Create(context.Context, ExternalAccount, EncryptedSecret) (ExternalAccount, error)
	Get(context.Context, Owner, AccountID) (ExternalAccount, error)
	List(context.Context, Owner) ([]ExternalAccount, error)
	Update(context.Context, ExternalAccount, uint64) (ExternalAccount, error)
	ReplaceCredential(context.Context, ExternalAccount, EncryptedSecret, uint64) (ExternalAccount, error)
	MarkDeleting(context.Context, ExternalAccount, uint64) error
}

// SecretRepository is the narrow encrypted-envelope boundary used by sync
// workers. It never returns plaintext credentials.
type SecretRepository interface {
	GetSecret(context.Context, Owner, AccountID, Provider) (EncryptedSecret, error)
}

type SecretVault interface {
	Seal(context.Context, Owner, AccountID, Provider, Credentials) (EncryptedSecret, error)
	Open(context.Context, Owner, AccountID, Provider, EncryptedSecret) (Credentials, error)
	Destroy(context.Context, Owner, AccountID, Provider) error
}

type Validator interface {
	Validate(context.Context, ValidationRequest) (Capabilities, error)
}

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (function ClockFunc) Now() time.Time { return function() }

type IDGenerator interface {
	NewAccountID() (AccountID, error)
}

type IDGeneratorFunc func() (AccountID, error)

func (function IDGeneratorFunc) NewAccountID() (AccountID, error) { return function() }

type Service struct {
	repository Repository
	vault      SecretVault
	validators map[Provider]Validator
	clock      Clock
	ids        IDGenerator
}

func NewService(repository Repository, vault SecretVault, validators map[Provider]Validator, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, vault: vault, validators: validators, clock: clock, ids: ids}
}

func (service *Service) Create(ctx context.Context, owner Owner, request CreateRequest) (ExternalAccount, error) {
	ownerID, normalized, err := normalizeCreate(owner, request)
	if err != nil {
		return ExternalAccount{}, err
	}
	validator := service.validators[normalized.Provider]
	if validator == nil {
		return ExternalAccount{}, ErrUnsupported
	}
	capabilities, err := validator.Validate(ctx, validationRequest(normalized))
	if err != nil {
		return ExternalAccount{}, err
	}
	id, err := service.ids.NewAccountID()
	if err != nil {
		return ExternalAccount{}, err
	}
	createdAt := service.clock.Now().UTC()
	account := newExternalAccount(ownerID, id, normalized, capabilities, createdAt)
	secret, err := service.vault.Seal(ctx, ownerID, id, normalized.Provider, normalized.Credentials)
	if err != nil {
		return ExternalAccount{}, err
	}
	created, err := service.repository.Create(ctx, account, secret)
	if err == nil {
		return created, nil
	}
	return ExternalAccount{}, errors.Join(err, service.vault.Destroy(ctx, ownerID, id, normalized.Provider))
}

// Validate verifies provider connectivity and capability without persisting
// either the account projection or submitted credentials.
func (service *Service) Validate(ctx context.Context, owner Owner, request CreateRequest) (Capabilities, error) {
	_, normalized, err := normalizeCreate(owner, request)
	if err != nil {
		return Capabilities{}, err
	}
	validator := service.validators[normalized.Provider]
	if validator == nil {
		return Capabilities{}, ErrUnsupported
	}
	return validator.Validate(ctx, validationRequest(normalized))
}

// Reconnect revalidates replacement credentials and commits the new encrypted
// envelope with the recovered public state under one repository mutation.
func (service *Service) Reconnect(ctx context.Context, owner Owner, id AccountID, expected uint64, credentials Credentials) (ExternalAccount, error) {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	if !reconnectable(account.State) {
		return ExternalAccount{}, ErrInvalidTransition
	}
	validator := service.validators[account.Provider]
	if validator == nil {
		return ExternalAccount{}, ErrUnsupported
	}
	capabilities, err := validator.Validate(ctx, ValidationRequest{
		Provider: account.Provider, MailboxAddress: account.MailboxAddress,
		Credentials: credentials, IMAP: cloneEndpoint(account.Connection.IMAP), SMTP: cloneEndpoint(account.Connection.SMTP),
		CardDAV:      cloneCardDAVEndpoint(account.Connection.CardDAV),
		CalDAV:       cloneCalDAVEndpoint(account.Connection.CalDAV),
		AuthIdentity: valueOrEmpty(account.AuthIdentity), SharedMailboxAddress: valueOrEmpty(account.SharedMailboxAddress),
	})
	if err != nil {
		return ExternalAccount{}, err
	}
	secret, err := service.vault.Seal(ctx, account.Owner, account.ID, account.Provider, credentials)
	if err != nil {
		return ExternalAccount{}, err
	}
	account.State = StateReady
	account.Capabilities = capabilities
	account.Progress = nil
	account.Failure = nil
	account.Revision++
	account.UpdatedAt = service.clock.Now().UTC()
	return service.repository.ReplaceCredential(ctx, account, secret, expected)
}

func reconnectable(state State) bool {
	return state == StateReady || state == StateIdle || state == StateAuthRequired || state == StateDegraded || state == StatePaused
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validationRequest(request CreateRequest) ValidationRequest {
	return ValidationRequest{
		Provider: request.Provider, MailboxAddress: request.MailboxAddress,
		AuthIdentity: request.AuthIdentity, SharedMailboxAddress: request.SharedMailboxAddress,
		Credentials: request.Credentials, IMAP: request.IMAP, SMTP: request.SMTP,
		CardDAV: request.CardDAV, CalDAV: request.CalDAV,
	}
}

func newExternalAccount(owner Owner, id AccountID, request CreateRequest, capabilities Capabilities, now time.Time) ExternalAccount {
	var authIdentity *string
	if request.AuthIdentity != "" {
		authIdentity = stringPointer(request.AuthIdentity)
	}
	var sharedMailbox *string
	if request.SharedMailboxAddress != "" {
		sharedMailbox = stringPointer(request.SharedMailboxAddress)
	}
	return ExternalAccount{
		ID: id, Owner: owner, Provider: request.Provider, MailboxAddress: request.MailboxAddress,
		AuthIdentity: authIdentity, SharedMailboxAddress: sharedMailbox,
		Label: request.Label, Color: request.Color, Order: request.Order,
		Connection: ConnectionSettings{IMAP: cloneEndpoint(request.IMAP), SMTP: cloneEndpoint(request.SMTP), CardDAV: cloneCardDAVEndpoint(request.CardDAV), CalDAV: cloneCalDAVEndpoint(request.CalDAV), ContainerFolder: request.ContainerFolder},
		State:      StateReady, Capabilities: capabilities, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
}

func cloneEndpoint(endpoint *Endpoint) *Endpoint {
	if endpoint == nil {
		return nil
	}
	clone := *endpoint
	return &clone
}

func cloneCardDAVEndpoint(endpoint *CardDAVEndpoint) *CardDAVEndpoint {
	if endpoint == nil {
		return nil
	}
	clone := *endpoint
	return &clone
}

func cloneCalDAVEndpoint(endpoint *CalDAVEndpoint) *CalDAVEndpoint {
	if endpoint == nil {
		return nil
	}
	clone := *endpoint
	return &clone
}

func stringPointer(value string) *string { return &value }

func (service *Service) Get(ctx context.Context, owner Owner, id AccountID) (ExternalAccount, error) {
	ownerID, err := normalizedOwner(owner)
	if err != nil {
		return ExternalAccount{}, err
	}
	return service.repository.Get(ctx, ownerID, id)
}

func (service *Service) List(ctx context.Context, owner Owner) ([]ExternalAccount, error) {
	ownerID, err := normalizedOwner(owner)
	if err != nil {
		return nil, err
	}
	accounts, err := service.repository.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	sort.Slice(accounts, func(left, right int) bool {
		if accounts[left].Order == accounts[right].Order {
			return accounts[left].ID < accounts[right].ID
		}
		return accounts[left].Order < accounts[right].Order
	})
	return accounts, nil
}

func (service *Service) Pause(ctx context.Context, owner Owner, id AccountID, expected uint64) (ExternalAccount, error) {
	return service.changeState(ctx, owner, id, expected, StatePaused)
}

func (service *Service) Resume(ctx context.Context, owner Owner, id AccountID, expected uint64) (ExternalAccount, error) {
	return service.changeState(ctx, owner, id, expected, StateReady)
}

// UpdatePresentation compare-and-swaps non-secret account navigation metadata
// without changing provider credentials, endpoints, capabilities, or state.
func (service *Service) UpdatePresentation(ctx context.Context, owner Owner, id AccountID, expected uint64, update PresentationUpdate) (ExternalAccount, error) {
	normalized, err := normalizePresentationUpdate(update)
	if err != nil {
		return ExternalAccount{}, err
	}
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	if account.State == StateDeleting {
		return ExternalAccount{}, ErrInvalidTransition
	}
	if normalized.Label != nil {
		account.Label = *normalized.Label
	}
	if normalized.Color != nil {
		account.Color = *normalized.Color
	}
	if normalized.Order != nil {
		account.Order = *normalized.Order
	}
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) BeginSync(ctx context.Context, owner Owner, id AccountID, expected uint64) (ExternalAccount, error) {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	if account.State, err = Transition(account.State, StateSyncing); err != nil {
		return ExternalAccount{}, err
	}
	now := service.clock.Now().UTC()
	account.Progress = &SyncProgress{Phase: PhaseConnecting, StartedAt: now, UpdatedAt: now}
	account.Failure = nil
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) ReportProgress(ctx context.Context, owner Owner, id AccountID, expected uint64, update SyncProgressUpdate) (ExternalAccount, error) {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	if err := validateProgress(account, update); err != nil {
		return ExternalAccount{}, err
	}
	now := service.clock.Now().UTC()
	startedAt := now
	if account.Progress != nil {
		startedAt = account.Progress.StartedAt
	}
	account.Progress = &SyncProgress{
		Phase: update.Phase, Completed: update.Completed, Total: update.Total,
		StartedAt: startedAt, UpdatedAt: now,
	}
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) CompleteSync(ctx context.Context, owner Owner, id AccountID, expected uint64) (ExternalAccount, error) {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	if account.State, err = Transition(account.State, StateIdle); err != nil {
		return ExternalAccount{}, err
	}
	now := service.clock.Now().UTC()
	progress := SyncProgress{Phase: PhaseComplete, StartedAt: now, UpdatedAt: now}
	if account.Progress != nil {
		progress.Completed = account.Progress.Completed
		progress.Total = account.Progress.Total
		progress.StartedAt = account.Progress.StartedAt
	}
	account.Progress = &progress
	account.Failure = nil
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) RecordFailure(ctx context.Context, owner Owner, id AccountID, expected uint64, failure SyncFailure) (ExternalAccount, error) {
	if err := validateFailure(failure); err != nil {
		return ExternalAccount{}, err
	}
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	target := StateDegraded
	if failure.Authentication {
		target = StateAuthRequired
		failure.Retryable = false
		failure.RetryAt = nil
	}
	if account.State, err = Transition(account.State, target); err != nil {
		return ExternalAccount{}, err
	}
	now := service.clock.Now().UTC()
	account.Failure = &Failure{
		Code: failure.Code, Message: strings.TrimSpace(failure.Message), Retryable: failure.Retryable,
		OccurredAt: now, RetryAt: failure.RetryAt,
	}
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) Delete(ctx context.Context, owner Owner, id AccountID, expected uint64) error {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return err
	}
	if _, err := Transition(account.State, StateDeleting); err != nil {
		return err
	}
	account.State = StateDeleting
	account.Revision++
	account.UpdatedAt = service.clock.Now().UTC()
	return service.repository.MarkDeleting(ctx, account, expected)
}

func (service *Service) changeState(ctx context.Context, owner Owner, id AccountID, expected uint64, target State) (ExternalAccount, error) {
	account, err := service.accountForMutation(ctx, owner, id, expected)
	if err != nil {
		return ExternalAccount{}, err
	}
	next, err := Transition(account.State, target)
	if err != nil {
		return ExternalAccount{}, err
	}
	account.State = next
	if target == StateReady {
		account.Failure = nil
		account.Progress = nil
	}
	return service.saveMutation(ctx, account, expected)
}

func (service *Service) accountForMutation(ctx context.Context, owner Owner, id AccountID, expected uint64) (ExternalAccount, error) {
	account, err := service.Get(ctx, owner, id)
	if err != nil {
		return ExternalAccount{}, err
	}
	if account.Revision != expected {
		return ExternalAccount{}, ErrConflict
	}
	return account, nil
}

func (service *Service) saveMutation(ctx context.Context, account ExternalAccount, expected uint64) (ExternalAccount, error) {
	account.Revision++
	account.UpdatedAt = service.clock.Now().UTC()
	return service.repository.Update(ctx, account, expected)
}

func validateProgress(account ExternalAccount, update SyncProgressUpdate) error {
	if account.State != StateSyncing {
		return ErrInvalidTransition
	}
	nextOrder, valid := syncPhaseOrder(update.Phase)
	if !valid || update.Phase == PhaseComplete || update.Total != nil && *update.Total < update.Completed {
		return ErrInvalid
	}
	if account.Progress == nil {
		return nil
	}
	currentOrder, valid := syncPhaseOrder(account.Progress.Phase)
	if !valid || nextOrder < currentOrder || nextOrder == currentOrder && update.Completed < account.Progress.Completed {
		return ErrInvalid
	}
	return nil
}

func validateFailure(failure SyncFailure) error {
	message := strings.TrimSpace(failure.Message)
	if !failureCode.MatchString(failure.Code) || message == "" || len(message) > 512 || containsSecretMarker(message) {
		return ErrInvalid
	}
	return nil
}

func containsSecretMarker(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range []string{"password=", "access_token", "refresh_token", "authorization:"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func normalizedOwner(owner Owner) (Owner, error) {
	return normalizeOwner(owner)
}
