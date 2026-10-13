package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOAuthConnectionCreatesMicrosoftSharedMailboxWithoutExposingTokens(t *testing.T) {
	owner := Owner{TenantID: "tenant-1", PrincipalID: "principal-1", Address: "owner@example.com"}
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	transactions := &memoryOAuthTransactions{}
	creator := &recordingOAuthCreator{account: ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA"}}
	provider := &recordingOAuthProvider{grant: Credentials{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: now.Add(time.Hour)}}
	service := NewOAuthConnectionService(transactions, passthroughOAuthVault{}, provider, creator, ClockFunc(func() time.Time { return now }), strings.NewReader(strings.Repeat("s", 96)))

	authorization, err := service.Begin(context.Background(), owner, OAuthConnectionRequest{
		Provider: ProviderMicrosoft, MailboxAddress: "shared@example.com", AuthIdentity: "delegate@example.com",
		SharedMailboxAddress: "shared@example.com", Label: "Support", Color: "#336699", Order: 2,
	})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if authorization.AuthorizationURL == "" || strings.Contains(authorization.AuthorizationURL, "access-secret") || strings.Contains(authorization.AuthorizationURL, "refresh-secret") {
		t.Fatalf("Begin() authorization = %#v", authorization)
	}
	if len(transactions.items) != 1 || transactions.items[0].StateHash == authorization.State {
		t.Fatalf("transaction did not retain only a state digest: %#v", transactions.items)
	}

	account, err := service.Complete(context.Background(), owner, authorization.State, "authorization-code")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if account.ID == "" || creator.request.Provider != ProviderMicrosoft || creator.request.AuthIdentity != "delegate@example.com" || creator.request.SharedMailboxAddress != "shared@example.com" {
		t.Fatalf("created request = %#v account = %#v", creator.request, account)
	}
	if creator.request.Credentials.AccessToken != "access-secret" || creator.request.IMAP.Host != "outlook.office365.com" || creator.request.SMTP.Host != "smtp.office365.com" {
		t.Fatalf("created Microsoft connection = %#v", creator.request)
	}
	if _, err := service.Complete(context.Background(), owner, authorization.State, "authorization-code"); !errors.Is(err, ErrOAuthTransaction) {
		t.Fatalf("replayed Complete() error = %v", err)
	}
}

func TestOAuthConnectionIsOwnerBoundAndExpires(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	clockNow := now
	transactions := &memoryOAuthTransactions{}
	service := NewOAuthConnectionService(transactions, passthroughOAuthVault{}, &recordingOAuthProvider{}, &recordingOAuthCreator{}, ClockFunc(func() time.Time { return clockNow }), strings.NewReader(strings.Repeat("x", 192)))
	owner := Owner{TenantID: "tenant-1", PrincipalID: "principal-1", Address: "owner@example.com"}
	request := OAuthConnectionRequest{Provider: ProviderMicrosoft, MailboxAddress: "delegate@example.com", AuthIdentity: "delegate@example.com", Label: "Microsoft", Color: "#336699"}

	first, err := service.Begin(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	foreign := Owner{TenantID: owner.TenantID, PrincipalID: "principal-2", Address: "other@example.com"}
	if _, err := service.Complete(context.Background(), foreign, first.State, "code"); !errors.Is(err, ErrOAuthTransaction) {
		t.Fatalf("foreign Complete() error = %v", err)
	}

	second, err := service.Begin(context.Background(), owner, request)
	if err != nil {
		t.Fatalf("second Begin() error = %v", err)
	}
	clockNow = now.Add(11 * time.Minute)
	if _, err := service.Complete(context.Background(), owner, second.State, "code"); !errors.Is(err, ErrOAuthTransaction) {
		t.Fatalf("expired Complete() error = %v", err)
	}
}

func TestOAuthConnectionReconnectsExistingMicrosoftAccount(t *testing.T) {
	owner := Owner{TenantID: "tenant-1", PrincipalID: "principal-1", Address: "owner@example.com"}
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	creator := &recordingOAuthCreator{existing: ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: ProviderMicrosoft,
		MailboxAddress: "shared@example.com", AuthIdentity: stringPointer("delegate@example.com"),
		SharedMailboxAddress: stringPointer("shared@example.com"), Label: "Support", Color: "#336699", Revision: 7,
	}}
	service := NewOAuthConnectionService(&memoryOAuthTransactions{}, passthroughOAuthVault{},
		&recordingOAuthProvider{grant: Credentials{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: now.Add(time.Hour)}},
		creator, ClockFunc(func() time.Time { return now }), strings.NewReader(strings.Repeat("r", 96)))

	authorization, err := service.Begin(context.Background(), owner, OAuthConnectionRequest{
		AccountID: creator.existing.ID, Revision: 7, Provider: ProviderMicrosoft,
		// Caller-controlled presentation and mailbox fields are deliberately ignored for reconnects.
		MailboxAddress: "attacker@example.com", AuthIdentity: "attacker@example.com", Label: "Wrong", Color: "#FFFFFF",
	})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	account, err := service.Complete(context.Background(), owner, authorization.State, "authorization-code")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if creator.created || creator.reconnectID != creator.existing.ID || creator.reconnectRevision != 7 || creator.reconnectCredentials.RefreshToken != "new-refresh" {
		t.Fatalf("reconnect = %#v created=%v account=%#v", creator, creator.created, account)
	}
	if account.MailboxAddress != "shared@example.com" {
		t.Fatalf("reconnected account = %#v", account)
	}
}

func TestOAuthConnectionCreatesGmailAccountWithFixedEndpoints(t *testing.T) {
	owner := Owner{TenantID: "tenant-1", PrincipalID: "principal-1", Address: "owner@example.com"}
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	creator := &recordingOAuthCreator{account: ExternalAccount{ID: "ext_GGGGGGGGGGGGGGGGGGGG"}}
	service := NewProviderOAuthConnectionService(&memoryOAuthTransactions{}, passthroughOAuthVault{},
		&recordingOAuthProvider{grant: Credentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: now.Add(time.Hour)}}, creator,
		OAuthConnectionProfile{Provider: ProviderGmail, IMAP: Endpoint{Host: "imap.gmail.com", Port: 993, Security: SecurityTLS}, SMTP: Endpoint{Host: "smtp.gmail.com", Port: 465, Security: SecurityTLS}},
		ClockFunc(func() time.Time { return now }), strings.NewReader(strings.Repeat("g", 96)))
	authorization, err := service.Begin(context.Background(), owner, OAuthConnectionRequest{Provider: ProviderGmail, MailboxAddress: "owner@gmail.com", AuthIdentity: "owner@gmail.com", Label: "Gmail", Color: "#4285F4"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), owner, authorization.State, "code"); err != nil {
		t.Fatal(err)
	}
	if creator.request.Provider != ProviderGmail || creator.request.IMAP.Host != "imap.gmail.com" || creator.request.SMTP.Host != "smtp.gmail.com" {
		t.Fatalf("Gmail request = %#v", creator.request)
	}
	if _, err := service.Begin(context.Background(), owner, OAuthConnectionRequest{Provider: ProviderGmail, MailboxAddress: "shared@gmail.com", AuthIdentity: "owner@gmail.com", SharedMailboxAddress: "shared@gmail.com", Label: "Wrong", Color: "#4285F4"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("shared Gmail Begin() error = %v", err)
	}
}

type memoryOAuthTransactions struct{ items []OAuthTransaction }

func (store *memoryOAuthTransactions) Put(_ context.Context, transaction OAuthTransaction) error {
	store.items = append(store.items, transaction)
	return nil
}

func (store *memoryOAuthTransactions) Take(_ context.Context, owner Owner, provider Provider, stateHash string, now time.Time) (OAuthTransaction, error) {
	for index, transaction := range store.items {
		if transaction.Provider != provider || transaction.StateHash != stateHash || !sameOwner(transaction.Owner, owner) || !transaction.ExpiresAt.After(now) {
			continue
		}
		store.items = append(store.items[:index], store.items[index+1:]...)
		return transaction, nil
	}
	return OAuthTransaction{}, ErrOAuthTransaction
}

type passthroughOAuthVault struct{}

func (passthroughOAuthVault) Seal(_ context.Context, _ Owner, _ AccountID, _ Provider, credentials Credentials) (EncryptedSecret, error) {
	return EncryptedSecret{KeyVersion: 1, Ciphertext: credentials.Password}, nil
}
func (passthroughOAuthVault) Open(_ context.Context, _ Owner, _ AccountID, _ Provider, secret EncryptedSecret) (Credentials, error) {
	return Credentials{Password: secret.Ciphertext}, nil
}
func (passthroughOAuthVault) Destroy(context.Context, Owner, AccountID, Provider) error { return nil }

type recordingOAuthProvider struct{ grant Credentials }

func (provider *recordingOAuthProvider) AuthorizationURL(state, challenge string) (string, error) {
	return "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize?state=" + state + "&code_challenge=" + challenge, nil
}
func (provider *recordingOAuthProvider) Exchange(_ context.Context, code, verifier string) (Credentials, error) {
	if code == "" || verifier == "" {
		return Credentials{}, ErrOAuthGrant
	}
	return provider.grant, nil
}

type recordingOAuthCreator struct {
	request              CreateRequest
	account              ExternalAccount
	existing             ExternalAccount
	created              bool
	reconnectID          AccountID
	reconnectRevision    uint64
	reconnectCredentials Credentials
}

func (creator *recordingOAuthCreator) Create(_ context.Context, _ Owner, request CreateRequest) (ExternalAccount, error) {
	creator.request = request
	creator.created = true
	return creator.account, nil
}

func (creator *recordingOAuthCreator) Get(_ context.Context, owner Owner, id AccountID) (ExternalAccount, error) {
	if creator.existing.ID != id || !sameOwner(creator.existing.Owner, owner) {
		return ExternalAccount{}, ErrNotFound
	}
	return creator.existing, nil
}

func (creator *recordingOAuthCreator) Reconnect(_ context.Context, _ Owner, id AccountID, revision uint64, credentials Credentials) (ExternalAccount, error) {
	creator.reconnectID, creator.reconnectRevision, creator.reconnectCredentials = id, revision, credentials
	return creator.existing, nil
}

func oauthStateHashForTest(state string) string {
	digest := sha256.Sum256([]byte(state))
	return hex.EncodeToString(digest[:])
}
