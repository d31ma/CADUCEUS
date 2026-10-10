package microsoftoauth

import (
	"context"
	"errors"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

const refreshBeforeExpiry = 2 * time.Minute

type SecretRepository interface {
	GetSecret(context.Context, accounts.Owner, accounts.AccountID, accounts.Provider) (accounts.EncryptedSecret, error)
	RotateSecret(context.Context, accounts.ExternalAccount, accounts.EncryptedSecret, accounts.EncryptedSecret) error
}

type TokenRefresher interface {
	Refresh(context.Context, string) (accounts.Credentials, error)
}

// CredentialSource decrypts an account grant for one worker lease and rotates
// expiring Microsoft tokens with a compare-and-swap before returning them.
type CredentialSource struct {
	repository SecretRepository
	vault      accounts.SecretVault
	refreshers map[accounts.Provider]TokenRefresher
	clock      func() time.Time
}

func NewCredentialSource(repository SecretRepository, vault accounts.SecretVault, refresher TokenRefresher, clock func() time.Time) *CredentialSource {
	return NewCredentialSourceForProviders(repository, vault, map[accounts.Provider]TokenRefresher{accounts.ProviderMicrosoft: refresher}, clock)
}

func NewCredentialSourceForProviders(repository SecretRepository, vault accounts.SecretVault, refreshers map[accounts.Provider]TokenRefresher, clock func() time.Time) *CredentialSource {
	if clock == nil {
		clock = time.Now
	}
	return &CredentialSource{repository: repository, vault: vault, refreshers: refreshers, clock: clock}
}

func (source *CredentialSource) Credentials(ctx context.Context, account accounts.ExternalAccount) (accounts.Credentials, error) {
	if source == nil || source.repository == nil || source.vault == nil {
		return accounts.Credentials{}, accounts.ErrUnsupported
	}
	envelope, credentials, err := source.open(ctx, account)
	refresher, oauth := source.refreshers[account.Provider]
	if err != nil || !oauth || credentials.ExpiresAt.After(source.clock().UTC().Add(refreshBeforeExpiry)) {
		return credentials, err
	}
	if refresher == nil || credentials.RefreshToken == "" {
		clearCredential(&credentials)
		return accounts.Credentials{}, refreshFailure()
	}
	replacement, err := refresher.Refresh(ctx, credentials.RefreshToken)
	clearCredential(&credentials)
	if err != nil {
		return accounts.Credentials{}, refreshFailure()
	}
	sealed, err := source.vault.Seal(ctx, account.Owner, account.ID, account.Provider, replacement)
	if err != nil {
		clearCredential(&replacement)
		return accounts.Credentials{}, err
	}
	if err := source.repository.RotateSecret(ctx, account, envelope, sealed); err != nil {
		clearCredential(&replacement)
		if !errors.Is(err, accounts.ErrConflict) {
			return accounts.Credentials{}, err
		}
		_, current, openErr := source.open(ctx, account)
		if openErr != nil || current.ExpiresAt.Before(source.clock().UTC().Add(refreshBeforeExpiry)) {
			clearCredential(&current)
			return accounts.Credentials{}, refreshFailure()
		}
		return current, nil
	}
	return replacement, nil
}

func (source *CredentialSource) open(ctx context.Context, account accounts.ExternalAccount) (accounts.EncryptedSecret, accounts.Credentials, error) {
	envelope, err := source.repository.GetSecret(ctx, account.Owner, account.ID, account.Provider)
	if err != nil {
		return accounts.EncryptedSecret{}, accounts.Credentials{}, err
	}
	credentials, err := source.vault.Open(ctx, account.Owner, account.ID, account.Provider, envelope)
	return envelope, credentials, err
}

func refreshFailure() error {
	return accountsync.ConnectorError{Kind: accountsync.FailureAuthentication, Code: "provider_oauth_refresh_failed", Message: "Provider authorization expired. Reconnect this account."}
}

func clearCredential(credentials *accounts.Credentials) {
	credentials.Username = ""
	credentials.Password = ""
	credentials.AccessToken = ""
	credentials.RefreshToken = ""
	credentials.ExpiresAt = time.Time{}
}

var _ accountsync.CredentialsSource = (*CredentialSource)(nil)
