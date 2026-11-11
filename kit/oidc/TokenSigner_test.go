package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

type staticKeys struct {
	set      KeySet
	jwksUrls *[]string
}

func (keys staticKeys) Key(jwksUrl string, keyId string) (JsonWebKey, error) {
	*keys.jwksUrls = append(*keys.jwksUrls, jwksUrl)
	return keys.set.Find(keyId)
}

func generatedKeyText(t *testing.T) string {
	t.Helper()
	key, err := SigningKey{}.Generated()
	if err != nil {
		t.Fatal(err)
	}
	text, err := key.Text()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func signerDocument(current string, previous string) string {
	return `{"yon":{"oidc":{"issuer":"https://mail.example.net/api/v1/identity","jwks":"https://mail.example.net/api/v1/identity/jwks","audience":"mesail-api"},` +
		`"token":{"issuer":"https://mail.example.net/api/v1/identity"},"identity":{"signing":{"key":"` + current + `","previous":"` + previous + `"}}}}`
}

func configuredSigner(t *testing.T, current string, previous string) TokenSigner {
	t.Helper()
	signer := TokenSignerFactory{}.FromJson(signerDocument(current, previous))
	if signer.Problem != nil {
		t.Fatal(signer.Problem)
	}
	signer.Now = func() time.Time { return testNow() }
	return signer
}

func memberToken() MesailToken {
	return MesailToken{Subject: "person-1", AuthorizedParty: "mesail-web", Domain: "example.com", RealmRoles: []string{"mesail-member", "mesail-admin"}, Name: "Ana Example", Username: "ana", Email: "ana@example.com", SessionId: "session-1"}
}

func mesailVerifier(set KeySet, jwksUrls *[]string) ExactIssuerVerifier {
	settings := Settings{Issuer: "https://mail.example.net/api/v1/identity", Jwks: "https://mail.example.net/api/v1/identity/jwks", Audience: "mesail-api"}
	verifier := VerifierFactory{}.Mesail(settings)
	verifier.Keys = staticKeys{set: set, jwksUrls: jwksUrls}
	return verifier
}

func signedClaims(t *testing.T, signer TokenSigner, token MesailToken) map[string]any {
	t.Helper()
	issued, err := signer.Sign(token)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.Split(issued.AccessToken, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestSignedTokenPassesTheMesailVerifier(t *testing.T) {
	signer := configuredSigner(t, generatedKeyText(t), "")
	issued, err := signer.Sign(memberToken())
	if err != nil {
		t.Fatal(err)
	}
	set, err := signer.KeySet()
	if err != nil {
		t.Fatal(err)
	}
	urls := []string{}
	caller, err := NewGate(mesailVerifier(set, &urls), testNow).Identify("Bearer " + issued.AccessToken)
	if err != nil || caller.DomainId != "example.com" || caller.MemberId != "person-1" || caller.Username != "ana" || caller.SessionId != "session-1" || len(caller.Roles) != 2 || issued.ExpiresIn != 60 || urls[0] != "https://mail.example.net/api/v1/identity/jwks" {
		t.Fatalf("got %+v, %v, %d, %v", caller, err, issued.ExpiresIn, urls)
	}
}

func TestSignedTokenHasTheContractClaims(t *testing.T) {
	claims := signedClaims(t, configuredSigner(t, generatedKeyText(t), ""), memberToken())
	if claims["iss"] != "https://mail.example.net/api/v1/identity" || claims["aud"].([]any)[0] != "mesail-api" || claims["azp"] != "mesail-web" || claims["domain"] != "example.com" || claims["preferred_username"] != "ana" || claims["email"] != "ana@example.com" || claims["name"] != "Ana Example" || claims["sid"] != "session-1" {
		t.Fatalf("got %v", claims)
	}
}

func TestSignedTokenLivesSixtySeconds(t *testing.T) {
	claims := signedClaims(t, configuredSigner(t, generatedKeyText(t), ""), memberToken())
	issuedAt := int64(claims["iat"].(float64))
	if issuedAt != testNow().Unix() || int64(claims["nbf"].(float64)) != issuedAt || int64(claims["exp"].(float64)) != issuedAt+60 || claims["jti"] == "" || claims["typ"] != nil {
		t.Fatalf("got %v", claims)
	}
}

func TestSignedTokensHaveDifferentIds(t *testing.T) {
	signer := configuredSigner(t, generatedKeyText(t), "")
	first := signedClaims(t, signer, memberToken())
	second := signedClaims(t, signer, memberToken())
	if first["jti"] == second["jti"] {
		t.Fatalf("got the same id %v", first["jti"])
	}
}

func TestInstallWideTokenHasNoEmail(t *testing.T) {
	token := MesailToken{Subject: "service", AuthorizedParty: "mesail-service", Domain: InstallWide{}.DomainId(), RealmRoles: []string{"mesail-service"}}
	claims := signedClaims(t, configuredSigner(t, generatedKeyText(t), ""), token)
	_, hasEmail := claims["email"]
	if claims["domain"] != "*" || hasEmail {
		t.Fatalf("got %v", claims)
	}
}

func TestTokenHeaderNamesTheKeyThumbprint(t *testing.T) {
	signer := configuredSigner(t, generatedKeyText(t), "")
	issued, err := signer.Sign(memberToken())
	if err != nil {
		t.Fatal(err)
	}
	token, err := TokenParser{}.Parse(issued.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	public, err := signer.Current.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	canonical := `{"crv":"P-256","kty":"EC","x":"` + public.X + `","y":"` + public.Y + `"}`
	digest := sha256.Sum256([]byte(canonical))
	expected := base64.RawURLEncoding.EncodeToString(digest[:])
	if token.Header.KeyId != expected || public.KeyId != expected || token.Header.Algorithm != "ES256" || token.Header.Type != "JWT" || public.Use != "sig" || public.Algorithm != "ES256" {
		t.Fatalf("got %+v and %+v, want %s", token.Header, public, expected)
	}
}

func TestKeySetPublishesThePreviousKeyAfterTheCurrentOne(t *testing.T) {
	signer := configuredSigner(t, generatedKeyText(t), generatedKeyText(t))
	set, err := signer.KeySet()
	if err != nil {
		t.Fatal(err)
	}
	currentId, _ := signer.Current.KeyId()
	previousId, _ := signer.Previous[0].KeyId()
	if len(set.Keys) != 2 || set.Keys[0].KeyId != currentId || set.Keys[1].KeyId != previousId || currentId == previousId {
		t.Fatalf("got %+v", set)
	}
}

func TestATokenOfThePreviousKeyStillVerifiesDuringARotation(t *testing.T) {
	oldKey := generatedKeyText(t)
	before := configuredSigner(t, oldKey, "")
	issued, err := before.Sign(memberToken())
	if err != nil {
		t.Fatal(err)
	}
	after := configuredSigner(t, generatedKeyText(t), oldKey)
	set, err := after.KeySet()
	if err != nil {
		t.Fatal(err)
	}
	urls := []string{}
	_, err = mesailVerifier(set, &urls).Verify(issued.AccessToken, testNow())
	if err != nil {
		t.Fatal(err)
	}
}

func TestATokenOfAnUnpublishedKeyIsRefused(t *testing.T) {
	issued, err := configuredSigner(t, generatedKeyText(t), "").Sign(memberToken())
	if err != nil {
		t.Fatal(err)
	}
	set, err := configuredSigner(t, generatedKeyText(t), "").KeySet()
	if err != nil {
		t.Fatal(err)
	}
	urls := []string{}
	_, err = mesailVerifier(set, &urls).Verify(issued.AccessToken, testNow())
	refusedBy(t, err)
}

func TestTokenSignerFactoryNeedsTheIssuer(t *testing.T) {
	document := `{"yon":{"oidc":{"audience":"mesail-api"},"identity":{"signing":{"key":"` + generatedKeyText(t) + `"}}}}`
	signer := TokenSignerFactory{}.FromJson(document)
	if !errors.Is(signer.Problem, NotConfigured{}) {
		t.Fatalf("got %v", signer.Problem)
	}
}

func TestTokenSignerFactoryNeedsTheAudience(t *testing.T) {
	document := `{"yon":{"token":{"issuer":"https://x"},"identity":{"signing":{"key":"` + generatedKeyText(t) + `"}}}}`
	signer := TokenSignerFactory{}.FromJson(document)
	if !errors.Is(signer.Problem, NotConfigured{}) {
		t.Fatalf("got %v", signer.Problem)
	}
}

func TestTokenSignerFactoryRefusesAMissingKey(t *testing.T) {
	signer := TokenSignerFactory{}.FromJson(signerDocument("", ""))
	if !errors.Is(signer.Problem, NotConfigured{}) {
		t.Fatalf("got %v", signer.Problem)
	}
}

func TestTokenSignerFactoryRefusesABrokenPreviousKey(t *testing.T) {
	signer := TokenSignerFactory{}.FromJson(signerDocument(generatedKeyText(t), "bm90IGEga2V5"))
	if !errors.Is(signer.Problem, NotConfigured{}) {
		t.Fatalf("got %v", signer.Problem)
	}
}

func TestTokenSignerFactoryRefusesBrokenJson(t *testing.T) {
	signer := TokenSignerFactory{}.FromJson("{")
	if !errors.Is(signer.Problem, NotConfigured{}) {
		t.Fatalf("got %v", signer.Problem)
	}
}

func TestAnUnconfiguredSignerNeitherSignsNorPublishes(t *testing.T) {
	signer := TokenSignerFactory{}.FromJson("{}")
	_, signErr := signer.Sign(memberToken())
	_, setErr := signer.KeySet()
	if !errors.Is(signErr, NotConfigured{}) || !errors.Is(setErr, NotConfigured{}) {
		t.Fatalf("got %v, %v", signErr, setErr)
	}
}

func TestConfiguredSignerReadsTheEnvironment(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", signerDocument(generatedKeyText(t), ""))
	signer := TokenSignerFactory{}.Configured()
	if signer.Problem != nil || signer.Issuer != "https://mail.example.net/api/v1/identity" || signer.Audience != "mesail-api" || len(signer.Previous) != 0 {
		t.Fatalf("got %+v", signer)
	}
}

func brokenCurveKey() SigningKey {
	params := *elliptic.P256().Params()
	params.Name = "copy of P-256"
	key, _ := SigningKey{}.Generated()
	key.Private.PublicKey.Curve = &params
	return key
}

func TestSignerWithAnUnsupportedCurveFails(t *testing.T) {
	signer := configuredSigner(t, generatedKeyText(t), "")
	signer.Current = brokenCurveKey()
	_, signErr := signer.Sign(memberToken())
	_, setErr := signer.KeySet()
	_, publicErr := brokenCurveKey().PublicKey()
	if signErr == nil || setErr == nil || publicErr == nil {
		t.Fatalf("got %v, %v, %v", signErr, setErr, publicErr)
	}
}

func TestSigningWithAnEmptyCurveFails(t *testing.T) {
	key := SigningKey{Private: &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: &elliptic.CurveParams{Name: "empty", N: new(big.Int)}}, D: big.NewInt(1)}}
	_, err := key.Sign("input")
	if err == nil {
		t.Fatal("signed with an empty curve")
	}
}
