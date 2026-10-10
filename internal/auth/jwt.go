package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/d31ma/sesame/clients/go/sesame"
)

// clockSkew is the tolerance applied to the time-bound claims. It absorbs the
// small disagreement between this process and the engine that signed the
// token, and nothing more.
const clockSkew = 30 * time.Second

// keyRefreshInterval bounds how often an unknown key identifier may force a
// JWKS fetch. Without it, a stream of tokens carrying forged identifiers would
// be a request amplifier against the engine.
const keyRefreshInterval = time.Minute

// ErrToken reports a token that failed verification. Callers must not
// distinguish its causes to a client: a specific reason tells an attacker
// which half of the credential to change.
var ErrToken = errors.New("access token is not valid")

type verifyingKey struct {
	key *ecdsa.PublicKey
}

// Claims is the verified content of one SESAME access token.
type Claims struct {
	Issuer       string `json:"iss"`
	Subject      string `json:"sub"`
	Audience     string `json:"aud"`
	ExpiresAt    int64  `json:"exp"`
	IssuedAt     int64  `json:"iat"`
	NotBefore    int64  `json:"nbf"`
	ID           string `json:"jti"`
	Scope        string `json:"scope"`
	SessionID    string `json:"sid"`
	TenantID     string `json:"tenant_id"`
	Confirmation any    `json:"cnf,omitempty"`
	// Assurance appears on ID tokens and never on access tokens. It is
	// decoded here so the two can be told apart.
	Assurance string `json:"acr,omitempty"`
	Nonce     string `json:"nonce,omitempty"`
}

// VerifyToken checks one access token's signature, issuer, audience, and time
// bounds against the engine's published keys.
//
// This proves the token was minted by this deployment and has not been
// altered. It cannot prove the session behind it still stands, because
// revocation is not written into an already-signed token. Callers that act on
// a request must use Authorize, which asks the engine.
func (engine *Engine) VerifyToken(ctx context.Context, token string) (Claims, error) {
	header, claims, signingInput, signature, err := splitToken(token)
	if err != nil {
		return Claims{}, err
	}
	if header.Algorithm != "ES256" {
		// An unexpected algorithm is the classic downgrade: "none", or an HMAC
		// verified against the public key. Only the one algorithm this
		// deployment signs with is accepted.
		return Claims{}, fmt.Errorf("%w: unsupported algorithm", ErrToken)
	}
	key, err := engine.verifyingKey(ctx, header.KeyID)
	if err != nil {
		return Claims{}, err
	}
	if !verifyES256(key, signingInput, signature) {
		return Claims{}, fmt.Errorf("%w: signature does not verify", ErrToken)
	}
	if claims.Issuer != engine.options.Issuer {
		return Claims{}, fmt.Errorf("%w: issuer is not this deployment", ErrToken)
	}
	if _, known := engine.clientByID(claims.Audience); !known {
		return Claims{}, fmt.Errorf("%w: audience is not a registered client", ErrToken)
	}
	now := time.Now()
	if claims.ExpiresAt == 0 || now.After(time.Unix(claims.ExpiresAt, 0).Add(clockSkew)) {
		return Claims{}, fmt.Errorf("%w: token has expired", ErrToken)
	}
	if claims.NotBefore != 0 && now.Before(time.Unix(claims.NotBefore, 0).Add(-clockSkew)) {
		return Claims{}, fmt.Errorf("%w: token is not yet valid", ErrToken)
	}
	if claims.Subject == "" || claims.TenantID == "" || claims.SessionID == "" {
		return Claims{}, fmt.Errorf("%w: token is missing a required claim", ErrToken)
	}
	// An ID token is signed by the same key, for the same audience, with the
	// same session and tenant. Only its claim set distinguishes it: it carries
	// the assurance and nonce of the login it describes and no scope, while an
	// access token always carries a scope. Accepting one as the other is the
	// token-confusion class, so the difference is checked rather than assumed.
	if claims.Scope == "" || claims.Assurance != "" || claims.Nonce != "" {
		return Claims{}, fmt.Errorf("%w: this is not an access token", ErrToken)
	}
	return claims, nil
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

func splitToken(token string) (tokenHeader, Claims, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: malformed token", ErrToken)
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: malformed header", ErrToken)
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: malformed claims", ErrToken)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: malformed signature", ErrToken)
	}
	var header tokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: undecodable header", ErrToken)
	}
	var claims Claims
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return tokenHeader{}, Claims{}, "", nil, fmt.Errorf("%w: undecodable claims", ErrToken)
	}
	return header, claims, parts[0] + "." + parts[1], signature, nil
}

func verifyES256(key *ecdsa.PublicKey, signingInput string, signature []byte) bool {
	digest := sha256.Sum256([]byte(signingInput))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	return ecdsa.Verify(key, digest[:], r, s)
}

func (engine *Engine) verifyingKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error) {
	if keyID == "" {
		return nil, fmt.Errorf("%w: token names no signing key", ErrToken)
	}
	engine.keysMu.RLock()
	key, found := engine.keys[keyID]
	lastFetch := engine.keysFetch
	engine.keysMu.RUnlock()
	if found {
		return key.key, nil
	}
	if time.Since(lastFetch) < keyRefreshInterval {
		return nil, fmt.Errorf("%w: unknown signing key", ErrToken)
	}
	if err := engine.refreshKeys(ctx); err != nil {
		return nil, err
	}
	engine.keysMu.RLock()
	key, found = engine.keys[keyID]
	engine.keysMu.RUnlock()
	if !found {
		return nil, fmt.Errorf("%w: unknown signing key", ErrToken)
	}
	return key.key, nil
}

func (engine *Engine) refreshKeys(ctx context.Context) error {
	callCtx, cancel := engine.call(ctx)
	defer cancel()
	published, err := engine.client.SigningKeys(callCtx)
	if err != nil {
		return fmt.Errorf("read SESAME signing keys: %w", err)
	}
	keys := make(map[string]verifyingKey, len(published.Keys))
	for _, jwk := range published.Keys {
		key, err := publicKey(jwk)
		if err != nil {
			// One unusable key must not discard the rest of the set.
			continue
		}
		keys[jwk.KeyID] = verifyingKey{key: key}
	}
	if len(keys) == 0 {
		return errors.New("SESAME published no usable signing key")
	}
	engine.keysMu.Lock()
	engine.keys = keys
	engine.keysFetch = time.Now()
	engine.keysMu.Unlock()
	return nil
}

// publicKey converts one published P-256 JWK into a verifying key. Anything
// that is not an ES256 signing key on P-256 is refused rather than coerced.
func publicKey(jwk sesame.JWK) (*ecdsa.PublicKey, error) {
	if jwk.KeyType != "EC" || jwk.Curve != "P-256" || jwk.Algorithm != "ES256" {
		return nil, fmt.Errorf("unsupported signing key %s/%s/%s", jwk.KeyType, jwk.Curve, jwk.Algorithm)
	}
	if jwk.Use != "" && jwk.Use != "sig" {
		return nil, fmt.Errorf("signing key %q is published for %q", jwk.KeyID, jwk.Use)
	}
	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil || len(x) != 32 {
		return nil, errors.New("signing key has an invalid x coordinate")
	}
	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil || len(y) != 32 {
		return nil, errors.New("signing key has an invalid y coordinate")
	}
	key := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}
	if !key.Curve.IsOnCurve(key.X, key.Y) {
		return nil, errors.New("signing key is not a point on P-256")
	}
	return key, nil
}
