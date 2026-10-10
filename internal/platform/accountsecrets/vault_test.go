package accountsecrets

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestVaultSealsAndOpensCredentialsWithoutPlaintext(t *testing.T) {
	vault, err := New(2, map[uint32][]byte{2: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	credentials := accounts.Credentials{
		Username: "alice@remote.example", Password: "correct horse battery staple",
		RefreshToken: "refresh-secret",
	}
	envelope, err := vault.Seal(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, credentials)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if envelope.KeyVersion != 2 || envelope.Nonce == "" || envelope.Ciphertext == "" {
		t.Fatalf("Seal() envelope = %#v", envelope)
	}
	if strings.Contains(envelope.Ciphertext, credentials.Password) || strings.Contains(envelope.Ciphertext, credentials.RefreshToken) {
		t.Fatal("ciphertext contains plaintext secret")
	}
	opened, err := vault.Open(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, envelope)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened.Username != credentials.Username || opened.Password != credentials.Password || opened.RefreshToken != credentials.RefreshToken {
		t.Fatalf("Open() = %#v", opened)
	}
}

func TestVaultBindsCiphertextToOwnerAndAccount(t *testing.T) {
	vault, _ := New(1, map[uint32][]byte{1: []byte("0123456789abcdef0123456789abcdef")})
	envelope, err := vault.Seal(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, accounts.Credentials{Password: "secret"})
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if _, err := vault.Open(context.Background(), ownerBob(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, envelope); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("foreign owner Open() error = %v", err)
	}
	if _, err := vault.Open(context.Background(), ownerAlice(), "ext_BBBBBBBBBBBBBBBBBBBB", accounts.ProviderIMAP, envelope); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("foreign account Open() error = %v", err)
	}
	if _, err := vault.Open(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderGmail, envelope); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("foreign provider Open() error = %v", err)
	}
}

func TestVaultUsesUniqueNonces(t *testing.T) {
	vault, _ := New(1, map[uint32][]byte{1: []byte("0123456789abcdef0123456789abcdef")})
	first, _ := vault.Seal(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, accounts.Credentials{Password: "secret"})
	second, _ := vault.Seal(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, accounts.Credentials{Password: "secret"})
	if first.Nonce == second.Nonce || first.Ciphertext == second.Ciphertext {
		t.Fatal("two seals reused a nonce")
	}
}

func TestVaultRejectsUnsafeKeysAndMalformedEnvelopes(t *testing.T) {
	if _, err := New(1, map[uint32][]byte{1: []byte("short")}); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("New(short key) error = %v", err)
	}
	if _, err := New(2, map[uint32][]byte{1: []byte("0123456789abcdef0123456789abcdef")}); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("New(missing active key) error = %v", err)
	}
	vault, _ := New(1, map[uint32][]byte{1: []byte("0123456789abcdef0123456789abcdef")})
	malformed := []accounts.EncryptedSecret{
		{KeyVersion: 99, Nonce: "x", Ciphertext: "y"},
		{KeyVersion: 1, Nonce: "not-base64!", Ciphertext: "y"},
		{KeyVersion: 1, Nonce: base64.RawURLEncoding.EncodeToString([]byte("short")), Ciphertext: "y"},
	}
	for _, envelope := range malformed {
		if _, err := vault.Open(context.Background(), ownerAlice(), "ext_AAAAAAAAAAAAAAAAAAAA", accounts.ProviderIMAP, envelope); err == nil {
			t.Fatalf("Open(%#v) succeeded", envelope)
		}
	}
}

func ownerAlice() accounts.Owner {
	return accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"}
}

func ownerBob() accounts.Owner {
	return accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-bob", Address: "bob@example.com"}
}
