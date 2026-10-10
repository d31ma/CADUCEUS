package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const oauthTransactionLifetime = 10 * time.Minute

var (
	ErrOAuthTransaction = errors.New("OAuth transaction is invalid or expired")
	ErrOAuthGrant       = errors.New("OAuth authorization could not be completed")
)

// OAuthConnectionRequest contains only the non-secret account draft retained
// while the browser is at the provider authorization screen.
type OAuthConnectionRequest struct {
	AccountID            AccountID `json:"accountId,omitempty"`
	Revision             uint64    `json:"revision,omitempty"`
	Provider             Provider  `json:"provider"`
	MailboxAddress       string    `json:"mailboxAddress"`
	AuthIdentity         string    `json:"authIdentity"`
	SharedMailboxAddress string    `json:"sharedMailboxAddress,omitempty"`
	Label                string    `json:"label"`
	Color                string    `json:"color"`
	Order                uint16    `json:"order"`
}

type OAuthAuthorization struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

// OAuthTransaction stores a digest of browser state and an encrypted PKCE
// verifier. The raw state is returned only to the browser and provider.
type OAuthTransaction struct {
	StateHash  string                 `json:"stateHash"`
	Owner      Owner                  `json:"-"`
	Provider   Provider               `json:"provider"`
	Request    OAuthConnectionRequest `json:"request"`
	Verifier   EncryptedSecret        `json:"verifier"`
	VerifierID AccountID              `json:"verifierId"`
	CreatedAt  time.Time              `json:"createdAt"`
	ExpiresAt  time.Time              `json:"expiresAt"`
}

type OAuthTransactionRepository interface {
	Put(context.Context, OAuthTransaction) error
	Take(context.Context, Owner, Provider, string, time.Time) (OAuthTransaction, error)
}

type OAuthProviderClient interface {
	AuthorizationURL(state, challenge string) (string, error)
	Exchange(context.Context, string, string) (Credentials, error)
}

type OAuthAccountConnector interface {
	Create(context.Context, Owner, CreateRequest) (ExternalAccount, error)
	Get(context.Context, Owner, AccountID) (ExternalAccount, error)
	Reconnect(context.Context, Owner, AccountID, uint64, Credentials) (ExternalAccount, error)
}

// OAuthConnectionService owns state/PKCE generation and completion. Provider
// HTTP transport and FYLO persistence remain replaceable adapters.
type OAuthConnectionService struct {
	transactions OAuthTransactionRepository
	vault        SecretVault
	provider     OAuthProviderClient
	accounts     OAuthAccountConnector
	profile      OAuthConnectionProfile
	clock        Clock
	random       io.Reader
}

type OAuthConnectionProfile struct {
	Provider    Provider
	IMAP        Endpoint
	SMTP        Endpoint
	AllowShared bool
}

func NewOAuthConnectionService(transactions OAuthTransactionRepository, vault SecretVault, provider OAuthProviderClient, accounts OAuthAccountConnector, clock Clock, random io.Reader) *OAuthConnectionService {
	return NewProviderOAuthConnectionService(transactions, vault, provider, accounts, OAuthConnectionProfile{
		Provider: ProviderMicrosoft, AllowShared: true,
		IMAP: Endpoint{Host: "outlook.office365.com", Port: 993, Security: SecurityTLS},
		SMTP: Endpoint{Host: "smtp.office365.com", Port: 587, Security: SecurityStartTLS},
	}, clock, random)
}

func NewProviderOAuthConnectionService(transactions OAuthTransactionRepository, vault SecretVault, provider OAuthProviderClient, accounts OAuthAccountConnector, profile OAuthConnectionProfile, clock Clock, random io.Reader) *OAuthConnectionService {
	return &OAuthConnectionService{transactions: transactions, vault: vault, provider: provider, accounts: accounts, profile: profile, clock: clock, random: random}
}

func (service *OAuthConnectionService) Begin(ctx context.Context, owner Owner, request OAuthConnectionRequest) (OAuthAuthorization, error) {
	if service == nil || service.transactions == nil || service.vault == nil || service.provider == nil || service.accounts == nil || service.clock == nil || service.random == nil {
		return OAuthAuthorization{}, ErrUnsupported
	}
	owner, err := normalizeOwner(owner)
	if err != nil {
		return OAuthAuthorization{}, err
	}
	if request.AccountID != "" {
		if request.Revision == 0 {
			return OAuthAuthorization{}, ErrInvalid
		}
		account, accountErr := service.accounts.Get(ctx, owner, request.AccountID)
		if accountErr != nil {
			return OAuthAuthorization{}, accountErr
		}
		if account.Provider != service.profile.Provider {
			return OAuthAuthorization{}, ErrUnsupported
		}
		if account.Revision != request.Revision {
			return OAuthAuthorization{}, ErrConflict
		}
		request = oauthRequestFromAccount(account)
	}
	owner, request, _, err = normalizeOAuthRequest(owner, request, service.profile)
	if err != nil {
		return OAuthAuthorization{}, err
	}
	state, err := randomToken(service.random)
	if err != nil {
		return OAuthAuthorization{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	verifier, err := randomToken(service.random)
	if err != nil {
		return OAuthAuthorization{}, fmt.Errorf("generate OAuth verifier: %w", err)
	}
	stateHash := oauthStateHash(state)
	verifierID := AccountID("oauth_" + stateHash[:24])
	envelope, err := service.vault.Seal(ctx, owner, verifierID, service.profile.Provider, Credentials{Password: verifier})
	if err != nil {
		return OAuthAuthorization{}, err
	}
	now := service.clock.Now().UTC()
	transaction := OAuthTransaction{
		StateHash: stateHash, Owner: owner, Provider: service.profile.Provider, Request: request,
		Verifier: envelope, VerifierID: verifierID, CreatedAt: now, ExpiresAt: now.Add(oauthTransactionLifetime),
	}
	if err := service.transactions.Put(ctx, transaction); err != nil {
		return OAuthAuthorization{}, errors.Join(err, service.vault.Destroy(ctx, owner, verifierID, service.profile.Provider))
	}
	challengeDigest := sha256.Sum256([]byte(verifier))
	authorizationURL, err := service.provider.AuthorizationURL(state, base64.RawURLEncoding.EncodeToString(challengeDigest[:]))
	if err != nil {
		return OAuthAuthorization{}, err
	}
	return OAuthAuthorization{AuthorizationURL: authorizationURL, State: state, ExpiresAt: transaction.ExpiresAt}, nil
}

func (service *OAuthConnectionService) Complete(ctx context.Context, owner Owner, state, code string) (ExternalAccount, error) {
	if service == nil || service.transactions == nil || service.vault == nil || service.provider == nil || service.accounts == nil || service.clock == nil {
		return ExternalAccount{}, ErrUnsupported
	}
	owner, err := normalizeOwner(owner)
	if err != nil || invalidOAuthValue(state, 1024) || invalidOAuthValue(code, 8192) {
		return ExternalAccount{}, ErrOAuthTransaction
	}
	transaction, err := service.transactions.Take(ctx, owner, service.profile.Provider, oauthStateHash(state), service.clock.Now().UTC())
	if err != nil {
		return ExternalAccount{}, ErrOAuthTransaction
	}
	decrypted, err := service.vault.Open(ctx, owner, transaction.VerifierID, service.profile.Provider, transaction.Verifier)
	if err != nil || invalidOAuthValue(decrypted.Password, 1024) {
		return ExternalAccount{}, ErrOAuthTransaction
	}
	credentials, err := service.provider.Exchange(ctx, code, decrypted.Password)
	decrypted.Password = ""
	if err != nil {
		return ExternalAccount{}, ErrOAuthGrant
	}
	_, _, create, err := normalizeOAuthRequest(owner, transaction.Request, service.profile)
	if err != nil {
		return ExternalAccount{}, ErrOAuthTransaction
	}
	create.Credentials = credentials
	if transaction.Request.AccountID != "" {
		return service.accounts.Reconnect(ctx, owner, transaction.Request.AccountID, transaction.Request.Revision, credentials)
	}
	return service.accounts.Create(ctx, owner, create)
}

func oauthRequestFromAccount(account ExternalAccount) OAuthConnectionRequest {
	return OAuthConnectionRequest{
		AccountID: account.ID, Revision: account.Revision, Provider: account.Provider,
		MailboxAddress: account.MailboxAddress, AuthIdentity: valueOrEmpty(account.AuthIdentity),
		SharedMailboxAddress: valueOrEmpty(account.SharedMailboxAddress), Label: account.Label,
		Color: account.Color, Order: account.Order,
	}
}

func normalizeOAuthRequest(owner Owner, request OAuthConnectionRequest, profile OAuthConnectionProfile) (Owner, OAuthConnectionRequest, CreateRequest, error) {
	if profile.Provider != ProviderMicrosoft && profile.Provider != ProviderGmail || request.Provider != profile.Provider || profile.IMAP.Host == "" || profile.SMTP.Host == "" {
		return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, ErrUnsupported
	}
	authIdentity, err := normalizeAddress(request.AuthIdentity)
	if err != nil {
		return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, fmt.Errorf("%w: authentication identity", ErrInvalid)
	}
	request.AuthIdentity = authIdentity
	if strings.TrimSpace(request.MailboxAddress) == "" {
		request.MailboxAddress = authIdentity
	}
	mailbox, err := normalizeAddress(request.MailboxAddress)
	if err != nil {
		return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, fmt.Errorf("%w: mailbox address", ErrInvalid)
	}
	request.MailboxAddress = mailbox
	if request.SharedMailboxAddress != "" {
		if !profile.AllowShared {
			return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, fmt.Errorf("%w: shared mailbox", ErrInvalid)
		}
		shared, sharedErr := normalizeAddress(request.SharedMailboxAddress)
		if sharedErr != nil || shared != mailbox {
			return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, fmt.Errorf("%w: shared mailbox", ErrInvalid)
		}
		request.SharedMailboxAddress = shared
	} else if mailbox != authIdentity {
		return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, fmt.Errorf("%w: mailbox identity", ErrInvalid)
	}
	create := CreateRequest{
		Provider: profile.Provider, MailboxAddress: mailbox, AuthIdentity: authIdentity,
		SharedMailboxAddress: request.SharedMailboxAddress, Label: request.Label, Color: request.Color, Order: request.Order,
		IMAP: cloneEndpoint(&profile.IMAP), SMTP: cloneEndpoint(&profile.SMTP),
	}
	normalizedOwner, normalizedCreate, err := normalizeCreate(owner, create)
	if err != nil {
		return Owner{}, OAuthConnectionRequest{}, CreateRequest{}, err
	}
	request.Label, request.Color, request.Order = normalizedCreate.Label, normalizedCreate.Color, normalizedCreate.Order
	return normalizedOwner, request, normalizedCreate, nil
}

func randomToken(source io.Reader) (string, error) {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(source, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func oauthStateHash(state string) string {
	digest := sha256.Sum256([]byte(state))
	return hex.EncodeToString(digest[:])
}

func invalidOAuthValue(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}
