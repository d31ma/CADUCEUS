package microsoftoauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestCredentialSourceRefreshesAndPersistsExpiringMicrosoftGrant(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	account := oauthAccount()
	repository := &memorySecrets{secret: accounts.EncryptedSecret{KeyVersion: 1, Ciphertext: "old"}}
	vault := &memoryVault{opened: accounts.Credentials{AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: now.Add(time.Minute)}}
	refresher := &memoryRefresher{grant: accounts.Credentials{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: now.Add(time.Hour)}}
	source := NewCredentialSource(repository, vault, refresher, func() time.Time { return now })

	grant, err := source.Credentials(context.Background(), account)
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	if grant.AccessToken != "new-access" || refresher.received != "old-refresh" || repository.rotations != 1 || vault.sealed.AccessToken != "new-access" {
		t.Fatalf("grant=%#v refresher=%#v repository=%#v vault=%#v", grant, refresher, repository, vault)
	}
}

func TestCredentialSourceDoesNotRefreshFreshOrNonMicrosoftCredentials(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	for _, account := range []accounts.ExternalAccount{oauthAccount(), {ID: "ext_BBBBBBBBBBBBBBBBBBBB", Provider: accounts.ProviderIMAP}} {
		repository := &memorySecrets{secret: accounts.EncryptedSecret{KeyVersion: 1, Ciphertext: "old"}}
		credentials := accounts.Credentials{Password: "password"}
		if account.Provider == accounts.ProviderMicrosoft {
			credentials = accounts.Credentials{AccessToken: "fresh", RefreshToken: "refresh", ExpiresAt: now.Add(time.Hour)}
		}
		vault := &memoryVault{opened: credentials}
		refresher := &memoryRefresher{}
		source := NewCredentialSource(repository, vault, refresher, func() time.Time { return now })
		if got, err := source.Credentials(context.Background(), account); err != nil || got.AccessToken != credentials.AccessToken || got.Password != credentials.Password || refresher.received != "" {
			t.Fatalf("Credentials(%s) = %#v, %v refresher=%#v", account.Provider, got, err, refresher)
		}
	}
}

func TestCredentialSourceFailsClosedWhenRefreshFails(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	repository := &memorySecrets{secret: accounts.EncryptedSecret{KeyVersion: 1, Ciphertext: "old"}}
	vault := &memoryVault{opened: accounts.Credentials{AccessToken: "old", RefreshToken: "refresh", ExpiresAt: now}}
	source := NewCredentialSource(repository, vault, &memoryRefresher{err: errors.New("provider detail")}, func() time.Time { return now })
	if _, err := source.Credentials(context.Background(), oauthAccount()); err == nil || err.Error() != "provider_oauth_refresh_failed: Provider authorization expired. Reconnect this account." {
		t.Fatalf("Credentials() error = %v", err)
	}
}

func oauthAccount() accounts.ExternalAccount {
	return accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: accounts.Owner{TenantID: "tenant", PrincipalID: "principal", Address: "owner@example.com"}, Provider: accounts.ProviderMicrosoft}
}

type memorySecrets struct {
	secret    accounts.EncryptedSecret
	rotations int
}

func (repository *memorySecrets) GetSecret(context.Context, accounts.Owner, accounts.AccountID, accounts.Provider) (accounts.EncryptedSecret, error) {
	return repository.secret, nil
}
func (repository *memorySecrets) RotateSecret(_ context.Context, _ accounts.ExternalAccount, old, replacement accounts.EncryptedSecret) error {
	if old != repository.secret {
		return accounts.ErrConflict
	}
	repository.secret = replacement
	repository.rotations++
	return nil
}

type memoryVault struct {
	opened accounts.Credentials
	sealed accounts.Credentials
}

func (vault *memoryVault) Open(context.Context, accounts.Owner, accounts.AccountID, accounts.Provider, accounts.EncryptedSecret) (accounts.Credentials, error) {
	return vault.opened, nil
}
func (vault *memoryVault) Seal(_ context.Context, _ accounts.Owner, _ accounts.AccountID, _ accounts.Provider, credentials accounts.Credentials) (accounts.EncryptedSecret, error) {
	vault.sealed = credentials
	return accounts.EncryptedSecret{KeyVersion: 2, Ciphertext: "new"}, nil
}
func (vault *memoryVault) Destroy(context.Context, accounts.Owner, accounts.AccountID, accounts.Provider) error {
	return nil
}

type memoryRefresher struct {
	grant    accounts.Credentials
	received string
	err      error
}

func (refresher *memoryRefresher) Refresh(_ context.Context, token string) (accounts.Credentials, error) {
	refresher.received = token
	return refresher.grant, refresher.err
}
