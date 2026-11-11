//go:build blackbox

package blackbox

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

type publicKey struct {
	KeyType string `json:"kty"`
	Curve   string `json:"crv"`
	KeyId   string `json:"kid"`
	Use     string `json:"use"`
}

type keySet struct {
	Keys []publicKey `json:"keys"`
}

func signingKeys(t *testing.T, stack Stack) keySet {
	t.Helper()
	var keys keySet
	stack.Call(t, http.MethodGet, "/api/v1/identity/jwks", "", nil).Expect(t, http.StatusOK).Decode(t, &keys)
	return keys
}

func hasKey(keys keySet, keyId string) bool {
	for _, key := range keys.Keys {
		if key.KeyId == keyId && key.KeyType == "EC" && key.Curve == "P-256" && key.Use == "sig" {
			return true
		}
	}
	return false
}

func serviceClientSecret(t *testing.T) string {
	t.Helper()
	secret := os.Getenv("MESAIL_SERVICE_CLIENT_SECRET")
	if secret == "" {
		t.Skip("MESAIL_SERVICE_CLIENT_SECRET is not set")
	}
	return secret
}

func serviceTokenReply(t *testing.T, stack Stack, secret string, domainId string) Reply {
	t.Helper()
	request := map[string]string{"clientId": "mesail-service", "clientSecret": secret, "domainId": domainId}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/service-token", "", request)
}

func TestAcId5BrowserGetsAMesailTokenNotAKeycloakToken(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	header := tokenHeader(t, ana.AccessToken)
	claims := tokenClaims(t, ana.AccessToken)
	issuer, _ := claims["iss"].(string)
	if header["alg"] != "ES256" || !strings.HasSuffix(issuer, "/api/v1/identity") || strings.Contains(issuer, "/realms/") {
		t.Fatalf("want an ES256 token from identity, got header %v and issuer %q", header, issuer)
	}
	if claims["domain"] != "example.com" || claims["azp"] != "mesail-web" || claims["aud"] != "mesail-api" {
		t.Fatalf("want domain example.com, azp mesail-web and aud mesail-api, got %v", claims)
	}
}

func TestAcId5AccessTokenLivesSixtySeconds(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	claims := tokenClaims(t, ana.AccessToken)
	issuedAt, _ := claims["iat"].(float64)
	expiresAt, _ := claims["exp"].(float64)
	if expiresAt-issuedAt != 60 || ana.ExpiresIn != 60 {
		t.Fatalf("want 60 seconds, got iat %v exp %v expiresIn %d", issuedAt, expiresAt, ana.ExpiresIn)
	}
}

func TestKeySetPublishesTheKeyThatSignedTheToken(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	keyId, _ := tokenHeader(t, ana.AccessToken)["kid"].(string)
	if !hasKey(signingKeys(t, stack), keyId) {
		t.Fatalf("the key set has no EC P-256 signing key %q", keyId)
	}
}

func TestAcId1TokenSignedByNoPublishedKeyIsRefused(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	parts := strings.Split(ana.AccessToken, ".")
	tampered := parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2]))
	stack.Call(t, http.MethodGet, "/api/v1/identity/me", tampered, nil).Expect(t, http.StatusUnauthorized)
}

func TestServiceTokenNamesTheRequestedDomain(t *testing.T) {
	stack := NewStack(t)
	var issued struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int    `json:"expiresIn"`
	}
	serviceTokenReply(t, stack, serviceClientSecret(t), "example.com").Expect(t, http.StatusOK).Decode(t, &issued)
	claims := tokenClaims(t, issued.AccessToken)
	if claims["domain"] != "example.com" || claims["azp"] != "mesail-service" || issued.ExpiresIn != 60 {
		t.Fatalf("want a 60 s service token for example.com, got %v", claims)
	}
}

func TestServiceTokenForTheWholeInstallNamesTheStar(t *testing.T) {
	stack := NewStack(t)
	var issued struct {
		AccessToken string `json:"accessToken"`
	}
	serviceTokenReply(t, stack, serviceClientSecret(t), "*").Expect(t, http.StatusOK).Decode(t, &issued)
	if tokenClaims(t, issued.AccessToken)["domain"] != "*" {
		t.Fatal("want domain * on an install-wide service token")
	}
}

func TestServiceTokenRefusesAWrongSecret(t *testing.T) {
	stack := NewStack(t)
	serviceTokenReply(t, stack, "not-the-secret", "example.com").Expect(t, http.StatusUnauthorized)
}

func TestServiceTokenRefusesABadDomain(t *testing.T) {
	stack := NewStack(t)
	serviceTokenReply(t, stack, serviceClientSecret(t), "Not A Domain").ExpectError(t, http.StatusBadRequest, "invalid_request")
}

func TestServiceTokenIsNotAUserSession(t *testing.T) {
	stack := NewStack(t)
	var issued struct {
		AccessToken string `json:"accessToken"`
	}
	serviceTokenReply(t, stack, serviceClientSecret(t), "example.com").Expect(t, http.StatusOK).Decode(t, &issued)
	stack.Call(t, http.MethodGet, "/api/v1/messaging/conversations", issued.AccessToken, nil).Expect(t, http.StatusForbidden)
}
