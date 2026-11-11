package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func pkcs8Text(t *testing.T, private any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestSigningKeyTextRoundTrips(t *testing.T) {
	text := generatedKeyText(t)
	parsed, err := SigningKey{}.Parsed(" " + text + "\n")
	if err != nil {
		t.Fatal(err)
	}
	again, err := parsed.Text()
	if err != nil || again != text {
		t.Fatalf("got %q, %v", again, err)
	}
}

func TestSigningKeyReadsPem(t *testing.T) {
	der, err := base64.StdEncoding.DecodeString(generatedKeyText(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	key, err := SigningKey{}.Parsed(text)
	if err != nil || key.Private.Curve != elliptic.P256() {
		t.Fatalf("got %v", err)
	}
}

func TestSigningKeyRefusesTextThatIsNotBase64(t *testing.T) {
	_, err := SigningKey{}.Parsed("not base64 !")
	if err == nil {
		t.Fatal("parsed a broken key")
	}
}

func TestSigningKeyRefusesBytesThatAreNotPkcs8(t *testing.T) {
	_, err := SigningKey{}.Parsed(base64.StdEncoding.EncodeToString([]byte("not a key")))
	if err == nil {
		t.Fatal("parsed a broken key")
	}
}

func TestSigningKeyRefusesAP384Key(t *testing.T) {
	private, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SigningKey{}.Parsed(pkcs8Text(t, private))
	refusedBy(t, err)
}

func TestSigningKeyRefusesAnRsaKey(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SigningKey{}.Parsed(pkcs8Text(t, private))
	refusedBy(t, err)
}
