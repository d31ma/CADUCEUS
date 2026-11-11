package webpush

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
}

func testKeys() VapidKeys {
	return VapidKeys{PublicKey: testPublicKey, PrivateKey: testPrivateKey, Subject: "mailto:ops@mesail.dev"}
}

func TestAuthorizationIsASignedVapidToken(t *testing.T) {
	header, err := NewVapidSigner(testKeys(), fixedNow).Authorization("https://fcm.googleapis.com/fcm/send/abc?x=1")
	if err != nil {
		t.Fatal(err)
	}
	prefix := "vapid t="
	suffix := ", k=" + testPublicKey
	if !strings.HasPrefix(header, prefix) || !strings.HasSuffix(header, suffix) {
		t.Fatalf("header %q", header)
	}
	token := strings.TrimSuffix(strings.TrimPrefix(header, prefix), suffix)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token %q", token)
	}
	if string(mustDecode(t, parts[0])) != `{"typ":"JWT","alg":"ES256"}` {
		t.Fatalf("jwt header %s", mustDecode(t, parts[0]))
	}
	claims := VapidClaims{}
	err = json.Unmarshal(mustDecode(t, parts[1]), &claims)
	if err != nil {
		t.Fatal(err)
	}
	expected := VapidClaims{Audience: "https://fcm.googleapis.com", Expires: fixedNow().Add(12 * time.Hour).Unix(), Subject: "mailto:ops@mesail.dev"}
	if claims != expected {
		t.Fatalf("claims %+v", claims)
	}
	signature := mustDecode(t, parts[2])
	if len(signature) != 64 {
		t.Fatalf("signature length %d", len(signature))
	}
	publicKey, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), mustDecode(t, testPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	first := new(big.Int).SetBytes(signature[:32])
	second := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(publicKey, digest[:], first, second) {
		t.Fatal("signature does not verify")
	}
}

func TestAuthorizationKeepsANonDefaultPortInTheAudience(t *testing.T) {
	header, err := NewVapidSigner(testKeys(), fixedNow).Authorization("https://push.example.com:8443/x")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Split(strings.TrimPrefix(header, "vapid t="), ".")
	claims := VapidClaims{}
	_ = json.Unmarshal(mustDecode(t, token[1]), &claims)
	if claims.Audience != "https://push.example.com:8443" {
		t.Fatalf("audience %q", claims.Audience)
	}
}

func TestAuthorizationRefusesARelativeOrInvalidEndpoint(t *testing.T) {
	for _, endpoint := range []string{"/push/abc", "https://exa mple.com/%zz"} {
		_, err := NewVapidSigner(testKeys(), fixedNow).Authorization(endpoint)
		refused := RefusedEndpoint{}
		if !errors.As(err, &refused) {
			t.Fatalf("%s: error %v", endpoint, err)
		}
	}
}

func TestAuthorizationRefusesABrokenPrivateKey(t *testing.T) {
	keys := testKeys()
	keys.PrivateKey = "%%%"
	_, err := NewVapidSigner(keys, fixedNow).Authorization("https://fcm.googleapis.com/x")
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("error %v", err)
	}
}
