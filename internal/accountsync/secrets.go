package accountsync

import (
	"context"

	"github.com/d31ma/caduceus/internal/accounts"
)

// VaultCredentials opens the exact account-owned encrypted envelope only for
// the duration of one worker call.
type VaultCredentials struct {
	repository accounts.SecretRepository
	vault      accounts.SecretVault
}

func NewVaultCredentials(repository accounts.SecretRepository, vault accounts.SecretVault) *VaultCredentials {
	return &VaultCredentials{repository: repository, vault: vault}
}

func (source *VaultCredentials) Credentials(ctx context.Context, account accounts.ExternalAccount) (accounts.Credentials, error) {
	envelope, err := source.repository.GetSecret(ctx, account.Owner, account.ID, account.Provider)
	if err != nil {
		return accounts.Credentials{}, err
	}
	return source.vault.Open(ctx, account.Owner, account.ID, account.Provider, envelope)
}
