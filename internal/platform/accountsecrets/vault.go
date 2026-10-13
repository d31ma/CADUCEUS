// Package accountsecrets encrypts external-provider credentials before they
// cross the persistence boundary.
package accountsecrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

const maximumPlaintextBytes = 64 << 10

var (
	ErrInvalidKeyring    = errors.New("invalid external account credential keyring")
	ErrInvalidCiphertext = errors.New("invalid external account credential ciphertext")
)

// Vault is an AES-256-GCM keyring. It copies keys at construction so callers
// cannot rotate or corrupt the active material behind its back.
type Vault struct {
	active uint32
	keys   map[uint32][]byte
	random io.Reader
}

func New(active uint32, keys map[uint32][]byte) (*Vault, error) {
	if active == 0 || len(keys) == 0 {
		return nil, ErrInvalidKeyring
	}
	copied := make(map[uint32][]byte, len(keys))
	for version, key := range keys {
		if version == 0 || len(key) != 32 {
			return nil, ErrInvalidKeyring
		}
		copied[version] = append([]byte(nil), key...)
	}
	if _, found := copied[active]; !found {
		return nil, ErrInvalidKeyring
	}
	return &Vault{active: active, keys: copied, random: rand.Reader}, nil
}

type credentialDocument struct {
	Username     string    `json:"username,omitempty"`
	Password     string    `json:"password,omitempty"`
	AccessToken  string    `json:"accessToken,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

func (vault *Vault) Seal(
	ctx context.Context,
	owner accounts.Owner,
	id accounts.AccountID,
	provider accounts.Provider,
	credentials accounts.Credentials,
) (accounts.EncryptedSecret, error) {
	if err := ctx.Err(); err != nil {
		return accounts.EncryptedSecret{}, err
	}
	plaintext, err := json.Marshal(credentialDocument{
		Username: credentials.Username, Password: credentials.Password,
		AccessToken: credentials.AccessToken, RefreshToken: credentials.RefreshToken,
		ExpiresAt: credentials.ExpiresAt,
	})
	if err != nil || len(plaintext) > maximumPlaintextBytes {
		return accounts.EncryptedSecret{}, ErrInvalidCiphertext
	}
	gcm, err := newGCM(vault.keys[vault.active])
	if err != nil {
		return accounts.EncryptedSecret{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(vault.random, nonce); err != nil {
		return accounts.EncryptedSecret{}, fmt.Errorf("generate credential nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, associatedData(owner, id, provider, vault.active))
	return accounts.EncryptedSecret{
		KeyVersion: vault.active,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}, nil
}

func (vault *Vault) Open(
	ctx context.Context,
	owner accounts.Owner,
	id accounts.AccountID,
	provider accounts.Provider,
	envelope accounts.EncryptedSecret,
) (accounts.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return accounts.Credentials{}, err
	}
	key, found := vault.keys[envelope.KeyVersion]
	if !found {
		return accounts.Credentials{}, ErrInvalidCiphertext
	}
	gcm, err := newGCM(key)
	if err != nil {
		return accounts.Credentials{}, err
	}
	nonce, ciphertext, err := decodeEnvelope(envelope, gcm)
	if err != nil {
		return accounts.Credentials{}, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, associatedData(owner, id, provider, envelope.KeyVersion))
	if err != nil || len(plaintext) > maximumPlaintextBytes {
		return accounts.Credentials{}, ErrInvalidCiphertext
	}
	var document credentialDocument
	if err := json.Unmarshal(plaintext, &document); err != nil {
		return accounts.Credentials{}, ErrInvalidCiphertext
	}
	return accounts.Credentials{
		Username: document.Username, Password: document.Password,
		AccessToken: document.AccessToken, RefreshToken: document.RefreshToken,
		ExpiresAt: document.ExpiresAt,
	}, nil
}

// Destroy is intentionally a no-op: ciphertext lifetime belongs to the
// repository transaction. The method lets a remote secret-store adapter revoke
// a reference without changing the feature service contract.
func (vault *Vault) Destroy(context.Context, accounts.Owner, accounts.AccountID, accounts.Provider) error {
	return nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidKeyring
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidKeyring
	}
	return gcm, nil
}

func decodeEnvelope(envelope accounts.EncryptedSecret, gcm cipher.AEAD) ([]byte, []byte, error) {
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, nil, ErrInvalidCiphertext
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) < gcm.Overhead() || len(ciphertext) > maximumPlaintextBytes+gcm.Overhead() {
		return nil, nil, ErrInvalidCiphertext
	}
	return nonce, ciphertext, nil
}

func associatedData(owner accounts.Owner, id accounts.AccountID, provider accounts.Provider, version uint32) []byte {
	return []byte(strings.Join([]string{
		"caduceus.external-account-secret.v1",
		owner.TenantID,
		owner.PrincipalID,
		string(id),
		string(provider),
		strconv.FormatUint(uint64(version), 10),
	}, "\x00"))
}
