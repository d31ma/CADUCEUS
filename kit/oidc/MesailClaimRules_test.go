package oidc

import (
	"errors"
	"testing"
	"time"
)

func mesailClaims() map[string]any {
	return map[string]any{
		"iss":                "https://mail.example.net/api/v1/identity",
		"sub":                "person-1",
		"aud":                "mesail-api",
		"azp":                "mesail-web",
		"exp":                testNow().Unix() + 60,
		"nbf":                testNow().Unix(),
		"iat":                testNow().Unix(),
		"domain":             "example.com",
		"preferred_username": "ana",
		"realm_access":       map[string]any{"roles": []string{"mesail-member"}},
	}
}

func mesailClaimsWith(name string, value any) map[string]any {
	claims := mesailClaims()
	claims[name] = value
	return claims
}

func mesailClaimsWithout(name string) map[string]any {
	claims := mesailClaims()
	delete(claims, name)
	return claims
}

func verifiedMesail(t *testing.T, claims map[string]any) (Caller, error) {
	t.Helper()
	signer := newTestSigner(t)
	urls := []string{}
	verifier := mesailVerifier(signer.keySet(), &urls)
	return NewGate(verifier, testNow).Identify("Bearer " + signer.ecdsaToken(t, "ec-1", claims))
}

func TestMesailVerifierAcceptsAWebToken(t *testing.T) {
	caller, err := verifiedMesail(t, mesailClaims())
	if err != nil || caller.DomainId != "example.com" || caller.Username != "ana" || caller.Roles[0] != "Member" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestMesailVerifierAcceptsAnInstallWideServiceToken(t *testing.T) {
	claims := mesailClaimsWith("domain", "*")
	claims["azp"] = "mesail-service"
	caller, err := verifiedMesail(t, claims)
	if err != nil || caller.DomainId != (InstallWide{}).DomainId() || !(InstallWide{}).Matches(caller.DomainId) {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestMesailVerifierRefusesAKeycloakClient(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("azp", "mesail-identity"))
	refusedBy(t, err)
}

func TestMesailVerifierRefusesATokenWithoutADomain(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWithout("domain"))
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAnInvalidDomain(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("domain", "Example.COM"))
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAnotherIssuer(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("iss", "https://mail.example.net/api/v1/identity/"))
	unknown := UnknownIssuer{}
	if !errors.As(err, &unknown) {
		t.Fatalf("got %v", err)
	}
}

func TestMesailVerifierRefusesAKeycloakRealmIssuer(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("iss", "https://id.mesail.test/realms/example.com"))
	unknown := UnknownIssuer{}
	if !errors.As(err, &unknown) {
		t.Fatalf("got %v", err)
	}
}

func TestMesailVerifierRefusesAnExpiredToken(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("exp", testNow().Unix()-31))
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAnotherAudience(t *testing.T) {
	_, err := verifiedMesail(t, mesailClaimsWith("aud", "account"))
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAnUnknownKey(t *testing.T) {
	signer := newTestSigner(t)
	urls := []string{}
	_, err := mesailVerifier(signer.keySet(), &urls).Verify(signer.ecdsaToken(t, "other", mesailClaims()), testNow())
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAForgedSignature(t *testing.T) {
	signer := newTestSigner(t)
	urls := []string{}
	_, err := mesailVerifier(signer.keySet(), &urls).Verify(signer.rsaToken(t, "ec-1", mesailClaims()), testNow())
	refusedBy(t, err)
}

func TestMesailVerifierRefusesAMalformedToken(t *testing.T) {
	urls := []string{}
	_, err := mesailVerifier(KeySet{}, &urls).Verify("not-a-token", testNow())
	refusedBy(t, err)
}

func TestExactIssuerVerifierWithoutAnIssuerRefusesEverything(t *testing.T) {
	signer := newTestSigner(t)
	urls := []string{}
	verifier := mesailVerifier(signer.keySet(), &urls)
	verifier.Issuer = ""
	_, err := verifier.Verify(signer.ecdsaToken(t, "ec-1", mesailClaimsWith("iss", "")), testNow())
	unknown := UnknownIssuer{}
	if !errors.As(err, &unknown) {
		t.Fatalf("got %v", err)
	}
}

func TestOldModeUsesTheRealmAndIgnoresADomainClaim(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	token := signer.rsaToken(t, "rsa-1", claimsWith("domain", "other.example"))
	caller, err := NewGate(keys.verifier(t, &clock{now: testNow()}), testNow).Identify("Bearer " + token)
	if err != nil || caller.DomainId != "example.com" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestConfiguredGateChoosesTheMesailModeWithoutARealmPlaceholder(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"issuer":"https://mail.example.net/api/v1/identity","jwks":"https://mail.example.net/api/v1/identity/jwks","audience":"mesail-api"}}}`)
	verifier, isMesail := VerifierFactory{}.FromEnvironment().(ExactIssuerVerifier)
	if !isMesail || verifier.Issuer != "https://mail.example.net/api/v1/identity" || verifier.Jwks != "https://mail.example.net/api/v1/identity/jwks" {
		t.Fatalf("got %#v", verifier)
	}
}

func TestConfiguredGateKeepsTheRealmModeWithARealmPlaceholder(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"issuer":"https://id.mesail.test/realms/{realm}","jwks":"https://id.mesail.test/realms/{realm}/certs","audience":"mesail-api"}}}`)
	_, isRealm := VerifierFactory{}.FromEnvironment().(JwksVerifier)
	if !isRealm {
		t.Fatal("the realm mode was not chosen")
	}
}

func TestKeycloakVerifierUsesTheRealmIssuerAndCertificates(t *testing.T) {
	verifier := VerifierFactory{}.Keycloak("http://127.0.0.1:8180/idp/realms/mesail/protocol/openid-connect")
	rules, isRealmRules := verifier.Rules.(ClaimRules)
	if verifier.Issuer != "http://127.0.0.1:8180/idp/realms/mesail" || verifier.Jwks != "http://127.0.0.1:8180/idp/realms/mesail/protocol/openid-connect/certs" || !isRealmRules || rules.Audience != "mesail-identity" {
		t.Fatalf("got %#v", verifier)
	}
}

func TestInstallWideDoesNotMatchADomain(t *testing.T) {
	if (InstallWide{}).Matches("example.com") {
		t.Fatal("a domain matched the install-wide id")
	}
}

func TestMesailClaimRulesReportEveryProblem(t *testing.T) {
	claims := Claims{Audience: []string{"account"}, AuthorizedParty: "other", ExpiresAt: 1}
	err := MesailClaimRules{Audience: "mesail-api"}.Check(claims, time.Unix(100, 0))
	if err == nil || len(err.(interface{ Unwrap() []error }).Unwrap()) != 5 {
		t.Fatalf("got %v", err)
	}
}
