package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRefusesATokenWithTwoParts(t *testing.T) {
	_, err := TokenParser{}.Parse("a.b")
	refusedBy(t, err)
}

func TestParseRefusesAnOversizedToken(t *testing.T) {
	_, err := TokenParser{}.Parse(strings.Repeat("a", 16385) + ".b.c")
	refusedBy(t, err)
}

func TestParseRefusesAHeaderThatIsNotBase64(t *testing.T) {
	_, err := TokenParser{}.Parse("***.e30.c2ln")
	refusedBy(t, err)
}

func TestParseRefusesAHeaderThatIsNotJson(t *testing.T) {
	_, err := TokenParser{}.Parse(encode([]byte("nope")) + ".e30.c2ln")
	refusedBy(t, err)
}

func TestParseRefusesClaimsThatAreNotJson(t *testing.T) {
	header := encode([]byte(`{"alg":"RS256","kid":"k"}`))
	_, err := TokenParser{}.Parse(header + "." + encode([]byte("nope")) + ".c2ln")
	refusedBy(t, err)
}

func TestParseRefusesASignatureThatIsNotBase64(t *testing.T) {
	header := encode([]byte(`{"alg":"RS256","kid":"k"}`))
	_, err := TokenParser{}.Parse(header + ".e30.***")
	refusedBy(t, err)
}

func TestParseRefusesANumericAudience(t *testing.T) {
	header := encode([]byte(`{"alg":"RS256","kid":"k"}`))
	_, err := TokenParser{}.Parse(header + "." + encode([]byte(`{"aud":7}`)) + ".c2ln")
	refusedBy(t, err)
}

func TestParseKeepsTheHeaderAndSigningInput(t *testing.T) {
	header := encode([]byte(`{"alg":"ES256","kid":"k","typ":"JWT"}`))
	token, err := TokenParser{}.Parse(header + ".e30.c2ln")
	if err != nil || token.Header.KeyId != "k" || token.Header.Type != "JWT" || token.SigningInput != header+".e30" || string(token.Signature) != "sig" {
		t.Fatalf("got %+v, %v", token, err)
	}
}

func TestRsaKeyRefusesAnotherKeyType(t *testing.T) {
	_, err := RsaKey{}.From(JsonWebKey{KeyType: "EC"})
	refusedBy(t, err)
}

func TestRsaKeyRefusesAShortKey(t *testing.T) {
	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RsaKey{}.From(rsaJwk(&weak.PublicKey, "weak", "sig"))
	refusedBy(t, err)
}

func TestRsaKeyRefusesABadModulus(t *testing.T) {
	_, err := RsaKey{}.From(JsonWebKey{KeyType: "RSA", Modulus: "***", Exponent: "AQAB"})
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestRsaKeyRefusesABadExponent(t *testing.T) {
	_, err := RsaKey{}.From(JsonWebKey{KeyType: "RSA", Modulus: encode(big.NewInt(7).Bytes()), Exponent: "***"})
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestEcdsaKeyRefusesAnotherCurve(t *testing.T) {
	_, err := EcdsaKey{}.From(JsonWebKey{KeyType: "EC", Curve: "P-384"})
	refusedBy(t, err)
}

func TestEcdsaKeyRefusesABadX(t *testing.T) {
	_, err := EcdsaKey{}.From(JsonWebKey{KeyType: "EC", Curve: "P-256", X: "***"})
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestEcdsaKeyRefusesABadY(t *testing.T) {
	_, err := EcdsaKey{}.From(JsonWebKey{KeyType: "EC", Curve: "P-256", X: encode(make([]byte, 32)), Y: "***"})
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestEcdsaKeyRefusesAPointOffTheCurve(t *testing.T) {
	_, err := EcdsaKey{}.From(JsonWebKey{KeyType: "EC", Curve: "P-256", X: encode(make([]byte, 32)), Y: encode(make([]byte, 32))})
	if err == nil {
		t.Fatal("expected a failure")
	}
}

func TestSignatureCheckRefusesAShortEs256Signature(t *testing.T) {
	signer := newTestSigner(t)
	token := SignedToken{Header: TokenHeader{Algorithm: "ES256"}, SigningInput: "a.b", Signature: []byte("short")}
	refusedBy(t, SignatureCheck{}.Verify(token, ecdsaJwk(&signer.ecdsaKey.PublicKey, "ec-1")))
}

func TestSignatureCheckRefusesAnUnknownAlgorithm(t *testing.T) {
	refusedBy(t, SignatureCheck{}.Verify(SignedToken{Header: TokenHeader{Algorithm: "PS256"}}, JsonWebKey{}))
}

func TestDomainNameRefusesAnOverlongName(t *testing.T) {
	if (DomainName{}).Valid(strings.Repeat("a.", 127) + "a") {
		t.Fatal("an overlong name was accepted")
	}
}

func TestDomainNameAcceptsALowercaseDomain(t *testing.T) {
	if !(DomainName{}).Valid("mail.example-1.org") {
		t.Fatal("a valid name was refused")
	}
}

func TestDomainNameRefusesAPath(t *testing.T) {
	if (DomainName{}).Valid("../admin") {
		t.Fatal("a path was accepted")
	}
}

func TestAddressDomainRefusesAnAddressWithoutADomain(t *testing.T) {
	_, err := AddressDomain{}.Of("ana")
	refusedBy(t, err)
}

func TestAddressDomainReadsTheLowercaseDomain(t *testing.T) {
	domain, err := AddressDomain{}.Of(" Ana@Example.COM ")
	if err != nil || domain != "example.com" {
		t.Fatalf("got %q, %v", domain, err)
	}
}

func TestSettingsReaderReadsTheOidcSection(t *testing.T) {
	settings, err := SettingsReader{}.FromJson(`{"yon":{"server":{"port":8101},"oidc":{"issuer":"https://id.test/api/v1/identity","jwks":"http://id/api/v1/identity/jwks","audience":"mesail-api"}}}`)
	want := Settings{Issuer: "https://id.test/api/v1/identity", Jwks: "http://id/api/v1/identity/jwks", Audience: "mesail-api"}
	if err != nil || settings != want {
		t.Fatalf("got %+v, %v", settings, err)
	}
}

func TestSettingsReaderRefusesAMissingAudience(t *testing.T) {
	_, err := SettingsReader{}.FromJson(`{"yon":{"oidc":{"issuer":"https://id.test/api/v1/identity","jwks":"http://id/api/v1/identity/jwks"}}}`)
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSettingsReaderRefusesAnEmptyDocument(t *testing.T) {
	_, err := SettingsReader{}.FromJson("")
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSettingsReaderReadsTheHandlerEnvironment(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"issuer":"https://id.test/api/v1/identity","jwks":"http://id/api/v1/identity/jwks","audience":"mesail-api"}}}`)
	settings, err := SettingsReader{}.FromEnvironment()
	if err != nil || settings.Audience != "mesail-api" {
		t.Fatalf("got %+v, %v", settings, err)
	}
}

func TestVerifierFactoryRefusesWithoutSettings(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	_, refusing := VerifierFactory{}.FromEnvironment().(RefusingVerifier)
	if !refusing {
		t.Fatal("want the refusing verifier")
	}
}

func TestVerifierFactoryBuildsTheMesailVerifier(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"issuer":"https://id.test/api/v1/identity","jwks":"http://id/api/v1/identity/jwks","audience":"mesail-api"}}}`)
	verifier, isMesail := VerifierFactory{}.FromEnvironment().(ExactIssuerVerifier)
	if !isMesail || verifier.Issuer != "https://id.test/api/v1/identity" {
		t.Fatalf("got %+v", verifier)
	}
}

func TestConfiguredGateChecksTokensWhenConfigured(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"issuer":"https://id.test/api/v1/identity","jwks":"http://id/api/v1/identity/jwks","audience":"mesail-api"}}}`)
	_, err := Configured().Identify("Bearer eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.")
	refusedBy(t, err)
}

func TestJsonFileCacheRoundTrips(t *testing.T) {
	cache := JsonFileCache{Folder: filepath.Join(t.TempDir(), "keys")}
	stored := CachedKeySet{FetchedAt: 42, Keys: KeySet{Keys: []JsonWebKey{{KeyType: "RSA", KeyId: "k"}}}}
	err := cache.Store("https://kc/certs", stored)
	loaded := CachedKeySet{}
	cache.Load("https://kc/certs", &loaded)
	if err != nil || loaded.FetchedAt != 42 || !loaded.Keys.Has("k") {
		t.Fatalf("got %+v, %v", loaded, err)
	}
}

func TestJsonFileCacheLoadsNothingForAnUnknownUrl(t *testing.T) {
	loaded := CachedKeySet{}
	JsonFileCache{Folder: t.TempDir()}.Load("https://kc/certs", &loaded)
	if loaded.FetchedAt != 0 {
		t.Fatalf("got %+v", loaded)
	}
}

func TestJsonFileCacheReportsAFolderItCannotCreate(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (JsonFileCache{Folder: filepath.Join(blocker, "keys")}).Store("u", CachedKeySet{}); err == nil {
		t.Fatal("expected a failure")
	}
}

func TestJsonFileCacheReportsAFolderItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only folder")
	}
	folder := t.TempDir()
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
	if err := (JsonFileCache{Folder: folder}).Store("u", CachedKeySet{}); err == nil {
		t.Fatal("expected a failure")
	}
}

func TestJsonFileCacheStoresJson(t *testing.T) {
	cache := JsonFileCache{Folder: t.TempDir()}
	if err := cache.Store("u", CachedKeySet{FetchedAt: 1}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cache.path("u"))
	if err != nil || !json.Valid(raw) {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestAudienceListWritesOneAudienceAsAString(t *testing.T) {
	raw, err := json.Marshal(AudienceList{"mesail-api"})
	if err != nil || string(raw) != `"mesail-api"` {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestAudienceListWritesSeveralAudiencesAsAList(t *testing.T) {
	raw, err := json.Marshal(AudienceList{"mesail-api", "account"})
	if err != nil || string(raw) != `["mesail-api","account"]` {
		t.Fatalf("got %s, %v", raw, err)
	}
}
