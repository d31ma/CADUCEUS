package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// LoginState is what a client learns about an in-flight authentication. It
// never distinguishes an unknown identifier from a wrong credential: the
// engine deliberately does not, and neither does this boundary.
type LoginState struct {
	TransactionID   string   `json:"transactionId"`
	State           string   `json:"state"`
	Assurance       string   `json:"assurance,omitempty"`
	RequiredFactors []string `json:"requiredFactors"`
	FailureCode     string   `json:"failureCode,omitempty"`
	AttemptsLeft    int      `json:"attemptsLeft"`
	Satisfied       bool     `json:"satisfied"`
}

// TokenSet is the credential a browser holds after login.
type TokenSet struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	IDToken      string `json:"idToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
	Scope        string `json:"scope"`
	SessionID    string `json:"sessionId"`
	PrincipalID  string `json:"principalId"`
	TenantID     string `json:"tenantId"`
	Domain       string `json:"domain"`
	Address      string `json:"address"`
	Assurance    string `json:"assurance"`
}

// LoginBegin starts one authentication transaction for a mailbox address.
func (engine *Engine) LoginBegin(ctx context.Context, address string) (LoginState, error) {
	parsed, err := ParseAddress(address)
	if err != nil {
		return LoginState{}, err
	}
	tenantID, err := engine.TenantID(ctx)
	if err != nil {
		return LoginState{}, err
	}
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.AuthenticationBegin(callCtx, tenantID, sesame.PrincipalIdentifier{
		Namespace: identifierNamespace,
		Value:     parsed.String(),
	})
	if err != nil {
		return LoginState{}, fmt.Errorf("begin authentication: %w", err)
	}
	return engine.loginState(result), nil
}

// identifierNamespace is the namespace every mailbox principal is created in.
const identifierNamespace = "email"

func (engine *Engine) loginState(result sesame.AuthenticationResult) LoginState {
	return LoginState{
		TransactionID:   result.TransactionID,
		State:           result.State,
		Assurance:       result.Assurance,
		RequiredFactors: requiredFactors(engine.options.Policy),
		FailureCode:     result.FailureCode,
		AttemptsLeft:    result.AttemptsLeft,
		Satisfied:       engine.satisfied(result),
	}
}

// satisfied reports whether the deployment's policy is met, which is a
// stricter question than whether the engine would issue a session. The engine
// will complete a transaction at password assurance; CADUCEUS will not.
func (engine *Engine) satisfied(result sesame.AuthenticationResult) bool {
	if result.State == StateFailed || result.TransactionID == "" {
		return false
	}
	if result.Assurance != AssuranceMFA {
		return false
	}
	if engine.options.Policy != PolicyPasswordAndPasskey {
		return true
	}
	return engine.ledger.verified(result.TransactionID, FactorPassword, time.Now())
}

// VerifyPassword supplies a password to a running transaction.
func (engine *Engine) VerifyPassword(ctx context.Context, transactionID, password string) (LoginState, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.AuthenticationVerifyPassword(callCtx, transactionID, password)
	if err != nil {
		return LoginState{}, fmt.Errorf("verify password: %w", err)
	}
	if result.State != StateFailed {
		engine.ledger.record(transactionID, FactorPassword, time.Now())
	}
	return engine.loginState(result), nil
}

// PasskeyOptions returns what a browser needs to answer a passkey challenge.
func (engine *Engine) PasskeyOptions(
	ctx context.Context,
	transactionID string,
) (sesame.PasskeyAuthenticationRequest, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	options, err := engine.client.PasskeyOptions(callCtx, transactionID)
	if err != nil {
		return sesame.PasskeyAuthenticationRequest{}, fmt.Errorf("passkey options: %w", err)
	}
	return options, nil
}

// VerifyPasskey advances a transaction with a WebAuthn assertion.
func (engine *Engine) VerifyPasskey(
	ctx context.Context,
	transactionID, credentialID string,
	authenticatorData, clientDataJSON, signature []byte,
) (LoginState, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.AuthenticationVerifyPasskey(
		callCtx, transactionID, credentialID, authenticatorData, clientDataJSON, signature,
	)
	if err != nil {
		return LoginState{}, fmt.Errorf("verify passkey: %w", err)
	}
	if result.State != StateFailed {
		engine.ledger.record(transactionID, FactorPasskey, time.Now())
	}
	return engine.loginState(result), nil
}

// VerifyTOTP advances a transaction with a time-based code.
func (engine *Engine) VerifyTOTP(ctx context.Context, transactionID, code string) (LoginState, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.AuthenticationVerifyTOTP(callCtx, transactionID, code)
	if err != nil {
		return LoginState{}, fmt.Errorf("verify TOTP: %w", err)
	}
	if result.State != StateFailed {
		engine.ledger.record(transactionID, FactorTOTP, time.Now())
	}
	return engine.loginState(result), nil
}

// VerifyRecoveryCode advances a transaction with a single-use recovery code.
func (engine *Engine) VerifyRecoveryCode(ctx context.Context, transactionID, code string) (LoginState, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	result, err := engine.client.AuthenticationVerifyRecoveryCode(callCtx, transactionID, code)
	if err != nil {
		return LoginState{}, fmt.Errorf("verify recovery code: %w", err)
	}
	if result.State != StateFailed {
		engine.ledger.record(transactionID, FactorRecoveryCode, time.Now())
	}
	return engine.loginState(result), nil
}

// LoginComplete issues the session and exchanges it for a token set.
//
// The exchange is the ordinary authorization-code flow with PKCE, run entirely
// inside this process: CADUCEUS is both the authorization host and the client,
// so no browser redirect is involved. It is used rather than handing the
// session secret to the browser because a signed, short-lived, audience-bound
// token can be verified without the engine, while a session secret is a
// bearer credential with none of those properties.
func (engine *Engine) LoginComplete(ctx context.Context, transactionID string) (TokenSet, error) {
	state, err := engine.transactionState(ctx, transactionID)
	if err != nil {
		return TokenSet{}, err
	}
	if !state.Satisfied {
		// The transaction is still live and the caller may still satisfy it,
		// so the factors it has already proven are kept. Forgetting them here
		// would make a premature completion attempt an unrecoverable login.
		return TokenSet{}, fmt.Errorf("%w: %s", ErrPolicy, missingFactorReason(engine.options.Policy, state))
	}

	// From here the transaction is consumed whatever happens next.
	defer engine.ledger.forget(transactionID)

	completeCtx, cancel := engine.call(ctx)
	session, err := engine.client.AuthenticationComplete(
		completeCtx, transactionID, engine.options.SessionLifetime,
	)
	cancel()
	if err != nil {
		return TokenSet{}, fmt.Errorf("complete authentication: %w", err)
	}
	if session.Assurance != AssuranceMFA {
		// The engine issued a session at an assurance this deployment does not
		// accept. Do not hand it out; end it.
		_ = engine.RevokeSession(ctx, session.SessionID, "assurance below policy")
		return TokenSet{}, fmt.Errorf("%w: session assurance is %s", ErrPolicy, session.Assurance)
	}
	return engine.exchange(ctx, session)
}

func (engine *Engine) transactionState(ctx context.Context, transactionID string) (LoginState, error) {
	// The engine has no "read transaction" operation: state is reported by the
	// verify that produced it. The ledger holds the factors, so the assurance
	// is reconstructed from them rather than guessed.
	now := time.Now()
	assurance := ""
	switch {
	case engine.ledger.verified(transactionID, FactorPasskey, now),
		engine.ledger.verified(transactionID, FactorTOTP, now),
		engine.ledger.verified(transactionID, FactorRecoveryCode, now):
		assurance = AssuranceMFA
	case engine.ledger.verified(transactionID, FactorPassword, now):
		assurance = AssurancePassword
	default:
		return LoginState{}, fmt.Errorf("%w: no factor has been verified", ErrPolicy)
	}
	result := sesame.AuthenticationResult{
		TransactionID: transactionID,
		State:         StateAwaitingFactor,
		Assurance:     assurance,
	}
	return engine.loginState(result), nil
}

func missingFactorReason(policy string, state LoginState) string {
	if state.Assurance != AssuranceMFA {
		return "a second factor is required"
	}
	if policy == PolicyPasswordAndPasskey {
		return "this deployment requires the password factor as well"
	}
	return "the transaction is not complete"
}

// exchange turns one issued session into a token set.
func (engine *Engine) exchange(ctx context.Context, session sesame.IssuedSession) (TokenSet, error) {
	registration, err := engine.clientFor(ctx, session.TenantID)
	if err != nil {
		return TokenSet{}, err
	}
	verifier, challenge, err := pkce()
	if err != nil {
		return TokenSet{}, err
	}

	authorizeCtx, cancel := engine.call(ctx)
	interaction, err := engine.client.Authorize(authorizeCtx, sesame.AuthorizationRequest{
		ClientID:            registration.ClientID,
		RedirectURI:         engine.options.RedirectURI,
		ResponseType:        "code",
		Scopes:              []string{"openid", "offline_access"},
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	cancel()
	if err != nil {
		return TokenSet{}, fmt.Errorf("start authorization: %w", err)
	}

	completeCtx, cancel := engine.call(ctx)
	authorized, err := engine.client.InteractionComplete(
		completeCtx, interaction.InteractionID, interaction.Secret,
		session.SessionID, session.Secret,
	)
	cancel()
	if err != nil {
		return TokenSet{}, fmt.Errorf("complete authorization: %w", err)
	}

	tokenCtx, cancel := engine.call(ctx)
	tokens, err := engine.client.TokenExchange(tokenCtx, sesame.TokenRequest{
		GrantType:    "authorization_code",
		Code:         authorized.Code,
		RedirectURI:  engine.options.RedirectURI,
		ClientID:     registration.ClientID,
		ClientSecret: registration.Secret,
		CodeVerifier: verifier,
	})
	cancel()
	if err != nil {
		return TokenSet{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	identity, err := engine.principalIdentity(ctx, session.PrincipalID)
	if err != nil {
		return TokenSet{}, err
	}
	return TokenSet{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IDToken:      tokens.IDToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
		Scope:        tokens.Scope,
		SessionID:    session.SessionID,
		PrincipalID:  session.PrincipalID,
		TenantID:     session.TenantID,
		Domain:       identity.Domain,
		Address:      identity.Address,
		Assurance:    session.Assurance,
	}, nil
}

// Refresh rotates one refresh token into a new token set. The presented token
// is consumed: SESAME detects reuse and ends the whole family.
func (engine *Engine) Refresh(ctx context.Context, refreshToken, clientID string) (TokenSet, error) {
	registration, found := engine.clientByID(clientID)
	if !found {
		return TokenSet{}, fmt.Errorf("%w: unknown client", ErrToken)
	}
	callCtx, cancel := engine.call(ctx)
	tokens, err := engine.client.TokenExchange(callCtx, sesame.TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     registration.ClientID,
		ClientSecret: registration.Secret,
	})
	cancel()
	if err != nil {
		return TokenSet{}, fmt.Errorf("refresh token: %w", err)
	}
	claims, err := engine.VerifyToken(ctx, tokens.AccessToken)
	if err != nil {
		return TokenSet{}, err
	}
	identity, err := engine.principalIdentity(ctx, claims.Subject)
	if err != nil {
		return TokenSet{}, err
	}
	return TokenSet{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IDToken:      tokens.IDToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
		Scope:        tokens.Scope,
		SessionID:    claims.SessionID,
		PrincipalID:  claims.Subject,
		TenantID:     claims.TenantID,
		Domain:       identity.Domain,
		Address:      identity.Address,
	}, nil
}

// Logout ends the session one ID token speaks for, which also ends every
// refresh grant resting on it.
func (engine *Engine) Logout(ctx context.Context, idToken string) error {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if _, err := engine.client.Logout(callCtx, idToken, "", ""); err != nil {
		return fmt.Errorf("log out: %w", err)
	}
	return nil
}

// RevokeSession ends one session by identifier.
func (engine *Engine) RevokeSession(ctx context.Context, sessionID, reason string) error {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	if err := engine.client.SessionRevoke(callCtx, sessionID, reason); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func pkce() (verifier string, challenge string, err error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buffer)
	digest := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

// Identity is the mailbox view of one principal.
type Identity struct {
	PrincipalID string `json:"principalId"`
	TenantID    string `json:"tenantId"`
	Address     string `json:"address"`
	Domain      string `json:"domain"`
	Local       string `json:"local"`
}

// principalIdentity resolves a principal to the mailbox it owns.
func (engine *Engine) principalIdentity(ctx context.Context, principalID string) (Identity, error) {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	principal, err := engine.client.PrincipalGetByID(callCtx, principalID)
	if err != nil {
		return Identity{}, fmt.Errorf("resolve principal: %w", err)
	}
	if principal.Identifier.Namespace != identifierNamespace {
		return Identity{}, errors.New("principal carries no mailbox identifier")
	}
	address, err := ParseAddress(principal.Identifier.Value)
	if err != nil {
		return Identity{}, fmt.Errorf("principal identifier is not a mailbox: %w", err)
	}
	return Identity{
		PrincipalID: principal.ID,
		TenantID:    principal.TenantID,
		Address:     address.String(),
		Domain:      address.Domain,
		Local:       address.Local,
	}, nil
}
