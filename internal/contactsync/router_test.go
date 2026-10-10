package contactsync

import (
	"context"
	"errors"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

type routerConnector struct {
	syncAccount     accounts.AccountID
	mutationAccount accounts.AccountID
}

func (connector *routerConnector) Sync(_ context.Context, account accounts.ExternalAccount, _ accounts.Credentials, _ State) (Snapshot, error) {
	connector.syncAccount = account.ID
	return Snapshot{PrincipalURL: "provider://principal"}, nil
}

func (connector *routerConnector) ApplyMutation(_ context.Context, account accounts.ExternalAccount, _ accounts.Credentials, _ Mutation, _ Contact) (MutationOutcome, error) {
	connector.mutationAccount = account.ID
	return OutcomeApplied, nil
}

func TestRouterDispatchesContactsByProvider(t *testing.T) {
	google := &routerConnector{}
	microsoft := &routerConnector{}
	router := NewRouter(map[accounts.Provider]Connector{accounts.ProviderGmail: google, accounts.ProviderMicrosoft: microsoft})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft}
	snapshot, err := router.Sync(context.Background(), account, accounts.Credentials{}, State{})
	if err != nil || snapshot.PrincipalURL != "provider://principal" || microsoft.syncAccount != account.ID || google.syncAccount != "" {
		t.Fatalf("Sync()=%#v, %v google=%q microsoft=%q", snapshot, err, google.syncAccount, microsoft.syncAccount)
	}
	outcome, err := router.ApplyMutation(context.Background(), account, accounts.Credentials{}, Mutation{}, Contact{})
	if err != nil || outcome != OutcomeApplied || microsoft.mutationAccount != account.ID || google.mutationAccount != "" {
		t.Fatalf("ApplyMutation()=%q, %v google=%q microsoft=%q", outcome, err, google.mutationAccount, microsoft.mutationAccount)
	}
}

func TestRouterRejectsUnconfiguredContactProvider(t *testing.T) {
	router := NewRouter(nil)
	account := accounts.ExternalAccount{Provider: accounts.ProviderIMAP}
	if _, err := router.Sync(context.Background(), account, accounts.Credentials{}, State{}); !errors.Is(err, accounts.ErrUnsupported) {
		t.Fatalf("Sync() error=%v", err)
	}
	if _, err := router.ApplyMutation(context.Background(), account, accounts.Credentials{}, Mutation{}, Contact{}); !errors.Is(err, accounts.ErrUnsupported) {
		t.Fatalf("ApplyMutation() error=%v", err)
	}
}

func TestEnabledRequiresContactTransport(t *testing.T) {
	account := accounts.ExternalAccount{Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Contacts: true}}
	if !Enabled(account) {
		t.Fatal("Gmail Contacts capability should enable provider contact sync")
	}
	account.Capabilities.Contacts = false
	if Enabled(account) {
		t.Fatal("Gmail without Contacts capability should not sync contacts")
	}
	account.Provider = accounts.ProviderIMAP
	account.Connection.CardDAV = &accounts.CardDAVEndpoint{URL: "https://dav.example.test/"}
	if !Enabled(account) {
		t.Fatal("CardDAV connection should enable contact sync")
	}
}
