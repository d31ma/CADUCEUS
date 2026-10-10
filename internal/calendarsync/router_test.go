package calendarsync

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

func (connector *routerConnector) ApplyMutation(_ context.Context, account accounts.ExternalAccount, _ accounts.Credentials, _ Mutation, _ Resource) (MutationOutcome, error) {
	connector.mutationAccount = account.ID
	return OutcomeApplied, nil
}

func TestRouterDispatchesSyncAndMutationByProvider(t *testing.T) {
	google := &routerConnector{}
	microsoft := &routerConnector{}
	router := NewRouter(map[accounts.Provider]Connector{
		accounts.ProviderGmail:     google,
		accounts.ProviderMicrosoft: microsoft,
	})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail}
	snapshot, err := router.Sync(context.Background(), account, accounts.Credentials{}, State{})
	if err != nil || snapshot.PrincipalURL != "provider://principal" || google.syncAccount != account.ID || microsoft.syncAccount != "" {
		t.Fatalf("Sync() = %#v, %v; google=%q microsoft=%q", snapshot, err, google.syncAccount, microsoft.syncAccount)
	}
	outcome, err := router.ApplyMutation(context.Background(), account, accounts.Credentials{}, Mutation{}, Resource{})
	if err != nil || outcome != OutcomeApplied || google.mutationAccount != account.ID || microsoft.mutationAccount != "" {
		t.Fatalf("ApplyMutation() = %q, %v; google=%q microsoft=%q", outcome, err, google.mutationAccount, microsoft.mutationAccount)
	}
}

func TestRouterRejectsUnconfiguredProviders(t *testing.T) {
	router := NewRouter(nil)
	account := accounts.ExternalAccount{Provider: accounts.ProviderIMAP}
	if _, err := router.Sync(context.Background(), account, accounts.Credentials{}, State{}); !errors.Is(err, accounts.ErrUnsupported) {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := router.ApplyMutation(context.Background(), account, accounts.Credentials{}, Mutation{}, Resource{}); !errors.Is(err, accounts.ErrUnsupported) {
		t.Fatalf("ApplyMutation() error = %v", err)
	}
}

func TestEnabledRequiresConfiguredCalendarTransport(t *testing.T) {
	account := accounts.ExternalAccount{Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Calendar: true}}
	if !Enabled(account) {
		t.Fatal("Gmail calendar capability should enable provider calendar sync")
	}
	account.Capabilities.Calendar = false
	if Enabled(account) {
		t.Fatal("Gmail account without calendar capability should not sync calendars")
	}
	account.Provider = accounts.ProviderIMAP
	account.Connection.CalDAV = &accounts.CalDAVEndpoint{URL: "https://dav.example.test/"}
	if !Enabled(account) {
		t.Fatal("CalDAV connection should enable calendar sync")
	}
}
