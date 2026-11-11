package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func refusedBy(t *testing.T, err error) {
	t.Helper()
	var refused RefusedToken
	var unknown UnknownIssuer
	if !errors.As(err, &refused) && !errors.As(err, &unknown) {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestVerifyAcceptsAnRs256KeycloakToken(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	claims, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", validClaims()), testNow())
	if err != nil || claims.Subject != "member-1" || claims.Email != "ana@example.com" || claims.Name != "Ana Example" {
		t.Fatalf("got %+v, %v", claims, err)
	}
}

func TestVerifyAcceptsAnEs256Token(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.ecdsaToken(t, "ec-1", validClaims()), testNow())
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyAcceptsASingleStringAudience(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("aud", "mesail-api")), testNow())
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyAcceptsAServiceToken(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("azp", "mesail-service")), testNow())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGateBuildsTheCallerFromARealToken(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	gate := NewGate(keys.verifier(t, &clock{now: testNow()}), testNow)
	caller, err := gate.Identify("Bearer " + signer.rsaToken(t, "rsa-1", validClaims()))
	want := Caller{DomainId: "example.com", MemberId: "member-1", DisplayName: "Ana Example", Email: "ana@example.com", Roles: []string{"Member"}}
	if err != nil || !reflect.DeepEqual(caller, want) {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestVerifyRefusesATokenSignedByAnotherKey(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, err = keys.verifier(t, &clock{now: testNow()}).Verify(rsaTokenWith(t, other, "rsa-1", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesATamperedClaim(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	original := strings.Split(signer.rsaToken(t, "rsa-1", validClaims()), ".")
	tampered := strings.Split(signer.rsaToken(t, "rsa-1", claimsWith("sub", "member-2")), ".")
	forged := original[0] + "." + tampered[1] + "." + original[2]
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(forged, testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnUnsignedTokenWithoutFetchingKeys(t *testing.T) {
	keys := newKeyServer(t, newTestSigner(t).keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify("eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.", testNow())
	refusedBy(t, err)
	if keys.fetches.Load() != 0 {
		t.Fatalf("fetched keys %d times", keys.fetches.Load())
	}
}

func TestVerifyRefusesAnHs256Token(t *testing.T) {
	keys := newKeyServer(t, newTestSigner(t).keySet())
	token := signingInput(t, map[string]any{"alg": "HS256", "kid": "rsa-1"}, validClaims()) + ".c2ln"
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(token, testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnExpiredToken(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("exp", testNow().Unix()-30)), testNow())
	refusedBy(t, err)
}

func TestVerifyAllowsThirtySecondsOfClockSkew(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("exp", testNow().Unix()-29)), testNow())
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRefusesATokenWithoutExpiry(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWithout("exp")), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesATokenThatIsNotValidYet(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("nbf", testNow().Unix()+31)), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnotherAudience(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("aud", []string{"account"})), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnUnknownClient(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("azp", "mesail-web")), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnIdToken(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("typ", "ID")), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesATokenWithoutSubject(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWithout("sub")), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnotherIssuerHostWithoutFetchingKeys(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("iss", "https://evil.test/realms/example.com")), testNow())
	refusedBy(t, err)
	if keys.fetches.Load() != 0 {
		t.Fatalf("fetched keys %d times", keys.fetches.Load())
	}
}

func TestVerifyRefusesARealmNameThatIsNotADomain(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("iss", "https://id.mesail.test/realms/Example.com/../x")), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnUnknownKeyId(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-9", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnEncryptionKey(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "enc-1", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnRs256TokenNamingAnEcKey(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "ec-1", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyRefusesAnEs256TokenNamingAnRsaKey(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.ecdsaToken(t, "rsa-1", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyFailsWhenTheRealmHasNoKeys(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("iss", "https://id.mesail.test/realms/other.example")), testNow())
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestVerifyFailsWhenTheKeySetIsNotJson(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	_, err := keys.verifier(t, &clock{now: testNow()}).Verify(signer.rsaToken(t, "rsa-1", claimsWith("iss", "https://id.mesail.test/realms/broken.example")), testNow())
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestVerifyFailsWhenKeycloakIsDown(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	verifier := keys.verifier(t, &clock{now: testNow()})
	keys.server.Close()
	_, err := verifier.Verify(signer.rsaToken(t, "rsa-1", validClaims()), testNow())
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestVerifyRefusesAJwksPatternWithoutTheRealm(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	verifier := keys.verifier(t, &clock{now: testNow()})
	verifier.Settings.Jwks = keys.server.URL + "/certs"
	_, err := verifier.Verify(signer.rsaToken(t, "rsa-1", validClaims()), testNow())
	refusedBy(t, err)
}

func TestVerifyUsesTheCachedKeySet(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	current := &clock{now: testNow()}
	verifier := keys.verifier(t, current)
	token := signer.rsaToken(t, "rsa-1", claimsWith("exp", testNow().Unix()+600))
	_, first := verifier.Verify(token, current.now)
	current.now = current.now.Add(200 * time.Second)
	_, second := verifier.Verify(token, current.now)
	if first != nil || second != nil || keys.fetches.Load() != 1 {
		t.Fatalf("got %v, %v after %d fetches", first, second, keys.fetches.Load())
	}
}

func TestVerifyRefetchesAnOldKeySet(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	current := &clock{now: testNow()}
	verifier := keys.verifier(t, current)
	token := signer.rsaToken(t, "rsa-1", claimsWith("exp", testNow().Unix()+600))
	_, _ = verifier.Verify(token, current.now)
	current.now = current.now.Add(301 * time.Second)
	_, err := verifier.Verify(token, current.now)
	if err != nil || keys.fetches.Load() != 2 {
		t.Fatalf("got %v after %d fetches", err, keys.fetches.Load())
	}
}

func TestVerifyWaitsBeforeRefetchingForAnUnknownKeyId(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	current := &clock{now: testNow()}
	verifier := keys.verifier(t, current)
	_, _ = verifier.Verify(signer.rsaToken(t, "rsa-1", validClaims()), current.now)
	_, soon := verifier.Verify(signer.rsaToken(t, "rsa-9", validClaims()), current.now.Add(5*time.Second))
	fetchesSoon := keys.fetches.Load()
	current.now = current.now.Add(11 * time.Second)
	_, _ = verifier.Verify(signer.rsaToken(t, "rsa-9", validClaims()), current.now)
	if soon == nil || fetchesSoon != 1 || keys.fetches.Load() != 2 {
		t.Fatalf("got %v, fetches %d then %d", soon, fetchesSoon, keys.fetches.Load())
	}
}

func TestVerifyRefetchesACacheFromTheFuture(t *testing.T) {
	signer := newTestSigner(t)
	keys := newKeyServer(t, signer.keySet())
	current := &clock{now: testNow()}
	verifier := keys.verifier(t, current)
	_, _ = verifier.Verify(signer.rsaToken(t, "rsa-1", validClaims()), current.now)
	current.now = current.now.Add(-time.Hour)
	_, _ = verifier.Verify(signer.rsaToken(t, "rsa-1", claimsWith("exp", current.now.Unix()+60)), current.now)
	if keys.fetches.Load() != 2 {
		t.Fatalf("fetched %d times", keys.fetches.Load())
	}
}

func TestRefusedTokenExplainsItself(t *testing.T) {
	if (RefusedToken{Reason: "x"}).Error() == "" {
		t.Fatal("empty message")
	}
}
