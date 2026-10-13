package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceCreatesRedactedValidatedAccount(t *testing.T) {
	repository := newMemoryRepository()
	vault := &recordingVault{}
	validator := &recordingValidator{capabilities: Capabilities{MailRead: true, MailWrite: true, Send: true}}
	service := NewService(repository, vault, map[Provider]Validator{ProviderIMAP: validator}, fixedClock(), fixedIDs("ext_AAAAAAAAAAAAAAAAAAAA"))

	account, err := service.Create(context.Background(), aliceOwner(), CreateRequest{
		Provider:       ProviderIMAP,
		MailboxAddress: " Alice@Remote.Example ",
		Label:          " Work ",
		Color:          "#336699",
		Credentials: Credentials{
			Username: "alice@remote.example",
			Password: "correct horse battery staple",
		},
		IMAP: &Endpoint{Host: "imap.remote.example", Port: 993, Security: SecurityTLS},
		SMTP: &Endpoint{Host: "smtp.remote.example", Port: 587, Security: SecurityStartTLS},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if account.ID != "ext_AAAAAAAAAAAAAAAAAAAA" || !sameOwner(account.Owner, aliceOwner()) {
		t.Fatalf("Create() identity = %#v", account)
	}
	if account.MailboxAddress != "alice@remote.example" || account.Label != "Work" {
		t.Fatalf("Create() normalized fields = %#v", account)
	}
	if account.State != StateReady || account.Revision != 1 || !account.Capabilities.Send {
		t.Fatalf("Create() lifecycle = %#v", account)
	}
	if account.Connection.IMAP == nil || account.Connection.IMAP.Host != "imap.remote.example" || account.Connection.SMTP == nil {
		t.Fatalf("Create() connection settings = %#v", account.Connection)
	}
	if validator.lastCredentials.Password != "correct horse battery staple" {
		t.Fatal("validator did not receive the submitted credential")
	}
	if vault.lastCredentials.Password != "correct horse battery staple" {
		t.Fatal("vault did not receive the submitted credential")
	}
	if repository.createdSecret.Ciphertext == "" {
		t.Fatal("repository did not receive a sealed secret")
	}
	if repository.createdSecret.Ciphertext == "correct horse battery staple" {
		t.Fatal("repository received a plaintext password")
	}
}

func TestServiceNormalizesOptionalCardDAVConnectionWithoutPersistingItsPassword(t *testing.T) {
	repository := newMemoryRepository()
	vault := &recordingVault{}
	validator := &recordingValidator{capabilities: Capabilities{MailRead: true, MailWrite: true, Send: true, Contacts: true}}
	service := NewService(repository, vault, map[Provider]Validator{ProviderIMAP: validator}, fixedClock(), fixedIDs("ext_AAAAAAAAAAAAAAAAAAAA"))

	account, err := service.Create(context.Background(), aliceOwner(), CreateRequest{
		Provider: ProviderIMAP, MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		Credentials: Credentials{Username: "alice@remote.example", Password: "app-password"},
		IMAP:        &Endpoint{Host: "imap.remote.example", Port: 993, Security: SecurityTLS},
		SMTP:        &Endpoint{Host: "smtp.remote.example", Port: 465, Security: SecurityTLS},
		CardDAV:     &CardDAVEndpoint{URL: "https://DAV.Remote.Example", Username: " alice-dav "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Connection.CardDAV == nil || account.Connection.CardDAV.URL != "https://dav.remote.example/" || account.Connection.CardDAV.Username != "alice-dav" || !account.Capabilities.Contacts {
		t.Fatalf("CardDAV connection = %#v capabilities=%#v", account.Connection.CardDAV, account.Capabilities)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "app-password") {
		t.Fatalf("public account leaked credentials: %s", encoded)
	}
}

func TestServiceNormalizesOptionalCalDAVConnectionWithoutPersistingItsPassword(t *testing.T) {
	repository := newMemoryRepository()
	vault := &recordingVault{}
	validator := &recordingValidator{capabilities: Capabilities{MailRead: true, MailWrite: true, Send: true, Calendar: true}}
	service := NewService(repository, vault, map[Provider]Validator{ProviderIMAP: validator}, fixedClock(), fixedIDs("ext_AAAAAAAAAAAAAAAAAAAA"))

	account, err := service.Create(context.Background(), aliceOwner(), CreateRequest{
		Provider: ProviderIMAP, MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		Credentials: Credentials{Username: "alice@remote.example", Password: "app-password"},
		IMAP:        &Endpoint{Host: "imap.remote.example", Port: 993, Security: SecurityTLS},
		SMTP:        &Endpoint{Host: "smtp.remote.example", Port: 465, Security: SecurityTLS},
		CalDAV:      &CalDAVEndpoint{URL: "https://DAV.Remote.Example", Username: " alice-dav "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Connection.CalDAV == nil || account.Connection.CalDAV.URL != "https://dav.remote.example/" || account.Connection.CalDAV.Username != "alice-dav" || !account.Capabilities.Calendar {
		t.Fatalf("CalDAV connection = %#v capabilities=%#v", account.Connection.CalDAV, account.Capabilities)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "app-password") {
		t.Fatalf("public account leaked credentials: %s", encoded)
	}
}

func TestServiceRejectsUnsafeCardDAVConnectionBeforeValidation(t *testing.T) {
	service := NewService(newMemoryRepository(), &recordingVault{}, map[Provider]Validator{ProviderIMAP: &recordingValidator{}}, fixedClock(), fixedIDs("unused"))
	base := CreateRequest{Provider: ProviderIMAP, MailboxAddress: "alice@example.com", Label: "Work", Color: "#336699",
		IMAP: &Endpoint{Host: "imap.example.com", Port: 993, Security: SecurityTLS}, SMTP: &Endpoint{Host: "smtp.example.com", Port: 465, Security: SecurityTLS}}
	for _, endpoint := range []*CardDAVEndpoint{
		{URL: "http://dav.example.com"},
		{URL: "https://user:password@dav.example.com"},
		{URL: "https://dav.example.com/#fragment"},
		{URL: "file:///etc/passwd"},
	} {
		request := base
		request.CardDAV = endpoint
		if _, err := service.Validate(context.Background(), aliceOwner(), request); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Validate(%q) error=%v, want ErrInvalid", endpoint.URL, err)
		}
	}
}

func TestServiceRejectsMalformedAndUnsupportedAccountsBeforePersistence(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("ext_AAAAAAAAAAAAAAAAAAAA"))

	tests := []CreateRequest{
		{Provider: "pop3", MailboxAddress: "alice@example.com", Label: "Work", Color: "#336699"},
		{Provider: ProviderIMAP, MailboxAddress: "not-an-email", Label: "Work", Color: "#336699"},
		{Provider: ProviderIMAP, MailboxAddress: "alice@example.com", Label: "", Color: "#336699"},
		{Provider: ProviderIMAP, MailboxAddress: "alice@example.com", Label: "Work", Color: "red"},
	}
	for _, request := range tests {
		if _, err := service.Create(context.Background(), aliceOwner(), request); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Create(%#v) error = %v, want ErrInvalid", request, err)
		}
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository Create() calls = %d, want 0", repository.createCalls)
	}
}

func TestServiceScopesReadsAndMutationsToExactOwner(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		State: StateIdle, Revision: 4,
	}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))

	if _, err := service.Get(context.Background(), bobOwner(), "ext_AAAAAAAAAAAAAAAAAAAA"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign Get() error = %v, want ErrNotFound", err)
	}
	if _, err := service.Pause(context.Background(), bobOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 4); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign Pause() error = %v, want ErrNotFound", err)
	}
	if repository.updateCalls != 0 {
		t.Fatalf("foreign mutation reached repository Update() %d times", repository.updateCalls)
	}
}

func TestServiceEnforcesLifecycleAndRevision(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		State: StateIdle, Revision: 4,
	}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))

	paused, err := service.Pause(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 4)
	if err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if paused.State != StatePaused || paused.Revision != 5 {
		t.Fatalf("Pause() = %#v", paused)
	}
	if _, err := service.Resume(context.Background(), aliceOwner(), paused.ID, 4); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Resume() error = %v, want ErrConflict", err)
	}
	resumed, err := service.Resume(context.Background(), aliceOwner(), paused.ID, 5)
	if err != nil || resumed.State != StateReady || resumed.Revision != 6 {
		t.Fatalf("Resume() = %#v, %v", resumed, err)
	}
}

func TestTransitionRejectsUnsafeStateChanges(t *testing.T) {
	if _, err := Transition(StatePaused, StateSyncing); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("paused -> syncing error = %v, want ErrInvalidTransition", err)
	}
	if next, err := Transition(StateDegraded, StateSyncing); err != nil || next != StateSyncing {
		t.Fatalf("degraded -> syncing = %q, %v", next, err)
	}
	if next, err := Transition(StateAuthRequired, StateReady); err != nil || next != StateReady {
		t.Fatalf("auth_required -> ready = %q, %v", next, err)
	}
}

func TestServiceRecordsMonotonicSyncProgressAndCompletion(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		State: StateReady, Revision: 1,
	}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))

	syncing, err := service.BeginSync(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 1)
	if err != nil || syncing.State != StateSyncing || syncing.Progress == nil || syncing.Progress.Phase != PhaseConnecting {
		t.Fatalf("BeginSync() = %#v, %v", syncing, err)
	}
	total := uint64(20)
	progressed, err := service.ReportProgress(context.Background(), aliceOwner(), syncing.ID, 2, SyncProgressUpdate{
		Phase: PhaseHeaders, Completed: 10, Total: &total,
	})
	if err != nil || progressed.Progress == nil || progressed.Progress.Completed != 10 || progressed.Revision != 3 {
		t.Fatalf("ReportProgress() = %#v, %v", progressed, err)
	}
	if _, err := service.ReportProgress(context.Background(), aliceOwner(), syncing.ID, 3, SyncProgressUpdate{
		Phase: PhaseFolders, Completed: 5, Total: &total,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("regressive ReportProgress() error = %v, want ErrInvalid", err)
	}
	completed, err := service.CompleteSync(context.Background(), aliceOwner(), syncing.ID, 3)
	if err != nil || completed.State != StateIdle || completed.Progress == nil || completed.Progress.Phase != PhaseComplete || completed.Failure != nil {
		t.Fatalf("CompleteSync() = %#v, %v", completed, err)
	}
}

func TestServiceClassifiesRedactedSyncFailures(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		State: StateSyncing, Revision: 7,
	}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))

	failed, err := service.RecordFailure(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 7, SyncFailure{
		Code: "authentication_failed", Message: "Authentication failed; reconnect the account.", Authentication: true,
	})
	if err != nil || failed.State != StateAuthRequired || failed.Failure == nil || failed.Failure.Retryable {
		t.Fatalf("RecordFailure(authentication) = %#v, %v", failed, err)
	}
	if _, err := service.RecordFailure(context.Background(), aliceOwner(), failed.ID, 8, SyncFailure{
		Code: "unsafe code!", Message: "password=secret", Retryable: true,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("RecordFailure(unsafe) error = %v, want ErrInvalid", err)
	}
}

func TestServiceListsInStableOrderAndMarksDeleting(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_BBBBBBBBBBBBBBBBBBBB"] = ExternalAccount{ID: "ext_BBBBBBBBBBBBBBBBBBBB", Owner: aliceOwner(), Order: 2, State: StateIdle, Revision: 1}
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Order: 1, State: StateIdle, Revision: 3}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))

	listed, err := service.List(context.Background(), aliceOwner())
	if err != nil || len(listed) != 2 || listed[0].ID != "ext_AAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	if err := service.Delete(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 3); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"].State != StateDeleting {
		t.Fatalf("Delete() state = %q", repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"].State)
	}
}

func TestServiceUpdatesOnlyValidatedPresentationMetadata(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699", Order: 2,
		State: StateIdle, Revision: 3,
	}
	service := NewService(repository, &recordingVault{}, nil, fixedClock(), fixedIDs("unused"))
	label, color, order := " Personal ", "#AABBCC", uint16(0)
	updated, err := service.UpdatePresentation(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 3, PresentationUpdate{
		Label: &label, Color: &color, Order: &order,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Label != "Personal" || updated.Color != "#AABBCC" || updated.Order != 0 || updated.Revision != 4 || updated.State != StateIdle {
		t.Fatalf("UpdatePresentation() = %#v", updated)
	}
	bad := "javascript:red"
	if _, err := service.UpdatePresentation(context.Background(), aliceOwner(), updated.ID, 4, PresentationUpdate{Color: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid color error = %v", err)
	}
	if _, err := service.UpdatePresentation(context.Background(), aliceOwner(), updated.ID, 3, PresentationUpdate{Label: &label}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update error = %v", err)
	}
}

func TestServiceRevalidatesAndAtomicallyReplacesCredentials(t *testing.T) {
	repository := newMemoryRepository()
	repository.accounts["ext_AAAAAAAAAAAAAAAAAAAA"] = ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: aliceOwner(), Provider: ProviderIMAP,
		MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		Connection: ConnectionSettings{
			IMAP: &Endpoint{Host: "imap.remote.example", Port: 993, Security: SecurityTLS},
			SMTP: &Endpoint{Host: "smtp.remote.example", Port: 587, Security: SecurityStartTLS},
		},
		State: StateAuthRequired, Failure: &Failure{Code: "authentication_failed", Message: "Reconnect", Retryable: false}, Revision: 4,
	}
	validator := &recordingValidator{capabilities: Capabilities{MailRead: true, MailWrite: true, Send: true}}
	vault := &recordingVault{}
	service := NewService(repository, vault, map[Provider]Validator{ProviderIMAP: validator}, fixedClock(), fixedIDs("unused"))

	reconnected, err := service.Reconnect(context.Background(), aliceOwner(), "ext_AAAAAAAAAAAAAAAAAAAA", 4, Credentials{
		Username: "alice@remote.example", Password: "new-password",
	})
	if err != nil {
		t.Fatalf("Reconnect() error = %v", err)
	}
	if reconnected.State != StateReady || reconnected.Revision != 5 || reconnected.Failure != nil || !reconnected.Capabilities.Send {
		t.Fatalf("Reconnect() = %#v", reconnected)
	}
	if validator.lastCredentials.Password != "new-password" || vault.lastCredentials.Password != "new-password" {
		t.Fatal("Reconnect() did not validate and seal the replacement credential")
	}
	if repository.reconnectCalls != 1 || repository.createdSecret.Ciphertext == "" {
		t.Fatalf("Reconnect() persistence calls=%d secret=%#v", repository.reconnectCalls, repository.createdSecret)
	}
}

type memoryRepository struct {
	accounts       map[AccountID]ExternalAccount
	createdSecret  EncryptedSecret
	createCalls    int
	updateCalls    int
	reconnectCalls int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{accounts: map[AccountID]ExternalAccount{}}
}

func (repository *memoryRepository) Create(_ context.Context, account ExternalAccount, secret EncryptedSecret) (ExternalAccount, error) {
	repository.createCalls++
	for _, existing := range repository.accounts {
		if sameOwner(existing.Owner, account.Owner) && existing.Provider == account.Provider && existing.MailboxAddress == account.MailboxAddress && existing.State != StateDeleting {
			return ExternalAccount{}, ErrConflict
		}
	}
	repository.accounts[account.ID] = account
	repository.createdSecret = secret
	return account, nil
}

func (repository *memoryRepository) Get(_ context.Context, owner Owner, id AccountID) (ExternalAccount, error) {
	account, found := repository.accounts[id]
	if !found || !sameOwner(account.Owner, owner) {
		return ExternalAccount{}, ErrNotFound
	}
	return account, nil
}

func (repository *memoryRepository) List(_ context.Context, owner Owner) ([]ExternalAccount, error) {
	result := []ExternalAccount{}
	for _, account := range repository.accounts {
		if sameOwner(account.Owner, owner) {
			result = append(result, account)
		}
	}
	return result, nil
}

func (repository *memoryRepository) Update(_ context.Context, account ExternalAccount, expected uint64) (ExternalAccount, error) {
	repository.updateCalls++
	current, found := repository.accounts[account.ID]
	if !found || !sameOwner(current.Owner, account.Owner) {
		return ExternalAccount{}, ErrNotFound
	}
	if current.Revision != expected {
		return ExternalAccount{}, ErrConflict
	}
	repository.accounts[account.ID] = account
	return account, nil
}

func (repository *memoryRepository) MarkDeleting(_ context.Context, replacement ExternalAccount, expected uint64) error {
	account, err := repository.Get(context.Background(), replacement.Owner, replacement.ID)
	if err != nil {
		return err
	}
	if account.Revision != expected {
		return ErrConflict
	}
	repository.accounts[replacement.ID] = replacement
	return nil
}

func (repository *memoryRepository) ReplaceCredential(_ context.Context, replacement ExternalAccount, secret EncryptedSecret, expected uint64) (ExternalAccount, error) {
	repository.reconnectCalls++
	current, err := repository.Get(context.Background(), replacement.Owner, replacement.ID)
	if err != nil {
		return ExternalAccount{}, err
	}
	if current.Revision != expected {
		return ExternalAccount{}, ErrConflict
	}
	repository.accounts[replacement.ID] = replacement
	repository.createdSecret = secret
	return replacement, nil
}

type recordingVault struct {
	lastCredentials Credentials
}

func (vault *recordingVault) Seal(_ context.Context, owner Owner, id AccountID, _ Provider, credentials Credentials) (EncryptedSecret, error) {
	vault.lastCredentials = credentials
	return EncryptedSecret{KeyVersion: 1, Nonce: "nonce", Ciphertext: "sealed:" + owner.PrincipalID + ":" + string(id)}, nil
}

func (vault *recordingVault) Open(context.Context, Owner, AccountID, Provider, EncryptedSecret) (Credentials, error) {
	return vault.lastCredentials, nil
}

func (vault *recordingVault) Destroy(context.Context, Owner, AccountID, Provider) error { return nil }

type recordingValidator struct {
	capabilities    Capabilities
	lastCredentials Credentials
}

func (validator *recordingValidator) Validate(_ context.Context, request ValidationRequest) (Capabilities, error) {
	validator.lastCredentials = request.Credentials
	return validator.capabilities, nil
}

func fixedClock() Clock {
	return ClockFunc(func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC) })
}

func fixedIDs(id string) IDGenerator {
	return IDGeneratorFunc(func() (AccountID, error) { return AccountID(id), nil })
}

func aliceOwner() Owner {
	return Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
}

func bobOwner() Owner {
	return Owner{TenantID: "tenant-1", PrincipalID: "principal-bob", Address: "bob@example.com"}
}
