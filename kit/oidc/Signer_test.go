package oidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testSigner struct {
	rsaKey   *rsa.PrivateKey
	ecdsaKey *ecdsa.PrivateKey
}

var sharedSigner testSigner
var sharedSignerOnce sync.Once

func newTestSigner(t *testing.T) testSigner {
	t.Helper()
	sharedSignerOnce.Do(func() {
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}
		sharedSigner = testSigner{rsaKey: rsaKey, ecdsaKey: ecdsaKey}
	})
	return sharedSigner
}

func encode(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func rsaJwk(key *rsa.PublicKey, keyId string, use string) JsonWebKey {
	return JsonWebKey{KeyType: "RSA", KeyId: keyId, Algorithm: "RS256", Use: use, Modulus: encode(key.N.Bytes()), Exponent: encode(big.NewInt(int64(key.E)).Bytes())}
}

func ecdsaJwk(key *ecdsa.PublicKey, keyId string) JsonWebKey {
	point, err := key.Bytes()
	if err != nil {
		panic(err)
	}
	return JsonWebKey{KeyType: "EC", KeyId: keyId, Algorithm: "ES256", Use: "sig", Curve: "P-256", X: encode(point[1:33]), Y: encode(point[33:])}
}

func (signer testSigner) keySet() KeySet {
	return KeySet{Keys: []JsonWebKey{
		rsaJwk(&signer.rsaKey.PublicKey, "enc-1", "enc"),
		rsaJwk(&signer.rsaKey.PublicKey, "rsa-1", "sig"),
		ecdsaJwk(&signer.ecdsaKey.PublicKey, "ec-1"),
	}}
}

func signingInput(t *testing.T, header map[string]any, claims map[string]any) string {
	t.Helper()
	rawHeader, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	rawClaims, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return encode(rawHeader) + "." + encode(rawClaims)
}

func (signer testSigner) rsaToken(t *testing.T, keyId string, claims map[string]any) string {
	t.Helper()
	return rsaTokenWith(t, signer.rsaKey, keyId, claims)
}

func rsaTokenWith(t *testing.T, key *rsa.PrivateKey, keyId string, claims map[string]any) string {
	t.Helper()
	input := signingInput(t, map[string]any{"alg": "RS256", "kid": keyId, "typ": "JWT"}, claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + encode(signature)
}

func (signer testSigner) ecdsaToken(t *testing.T, keyId string, claims map[string]any) string {
	t.Helper()
	input := signingInput(t, map[string]any{"alg": "ES256", "kid": keyId, "typ": "JWT"}, claims)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, signer.ecdsaKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return input + "." + encode(signature)
}

func testNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func validClaims() map[string]any {
	return map[string]any{
		"iss":          "https://id.mesail.test/realms/example.com",
		"sub":          "member-1",
		"aud":          []string{"mesail-api", "account"},
		"azp":          "mesail-identity",
		"exp":          testNow().Unix() + 60,
		"iat":          testNow().Unix(),
		"typ":          "Bearer",
		"name":         "Ana Example",
		"email":        "ana@example.com",
		"realm_access": map[string]any{"roles": []string{"mesail-member", "default-roles-example.com"}},
	}
}

func claimsWith(name string, value any) map[string]any {
	claims := validClaims()
	claims[name] = value
	return claims
}

func claimsWithout(name string) map[string]any {
	claims := validClaims()
	delete(claims, name)
	return claims
}

type keyServer struct {
	server  *httptest.Server
	fetches *atomic.Int64
}

func newKeyServer(t *testing.T, set KeySet) keyServer {
	t.Helper()
	fetches := &atomic.Int64{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/example.com/certs", func(writer http.ResponseWriter, request *http.Request) {
		fetches.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(set)
	})
	mux.HandleFunc("GET /realms/broken.example/certs", func(writer http.ResponseWriter, request *http.Request) {
		fetches.Add(1)
		_, _ = writer.Write([]byte("not json"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return keyServer{server: server, fetches: fetches}
}

type clock struct {
	now time.Time
}

func (current *clock) Now() time.Time {
	return current.now
}

func (keys keyServer) verifier(t *testing.T, current *clock) ExactIssuerVerifier {
	t.Helper()
	return keys.verifierFor(t, current, "example.com")
}

func (keys keyServer) verifierFor(t *testing.T, current *clock, realm string) ExactIssuerVerifier {
	t.Helper()
	source := CachedKeySource{Fetcher: HttpKeyFetcher{Client: keys.server.Client()}, Cache: JsonFileCache{Folder: t.TempDir()}, Now: current.Now}
	return ExactIssuerVerifier{Issuer: "https://id.mesail.test/realms/" + realm, Jwks: keys.server.URL + "/realms/" + realm + "/certs", Keys: source, Rules: ClaimRules{Audience: "mesail-api"}}
}
