package mailauth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

type testSigner struct {
	algorithm string
	canonical string
	domain    string
	selector  string
	headers   string
	extra     string
	length    int
	rsaKey    *rsa.PrivateKey
	edKey     ed25519.PrivateKey
}

func newRsaSigner(t *testing.T) testSigner {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return testSigner{algorithm: "rsa-sha256", canonical: "relaxed/relaxed", domain: "outside.test", selector: "s1", headers: "from:to:subject", length: -1, rsaKey: key}
}

func newEdSigner(t *testing.T) testSigner {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return testSigner{algorithm: "ed25519-sha256", canonical: "relaxed/relaxed", domain: "outside.test", selector: "e1", headers: "from:to:subject", length: -1, edKey: key}
}

func (signer testSigner) keyRecord(t *testing.T) string {
	if signer.edKey != nil {
		return "v=DKIM1; k=ed25519; p=" + base64.StdEncoding.EncodeToString(signer.edKey.Public().(ed25519.PublicKey))
	}
	der, err := x509.MarshalPKIXPublicKey(&signer.rsaKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(der)
}

func (signer testSigner) publish(t *testing.T, fake *FakeResolver) *FakeResolver {
	return fake.text(signer.selector+"._domainkey."+signer.domain, signer.keyRecord(t))
}

func (signer testSigner) sign(t *testing.T, message string) string {
	raw := newRawMessage([]byte(message))
	headerMethod, bodyMethod, _ := strings.Cut(signer.canonical, "/")
	body := dkimCanonical{}.body(raw.body, bodyMethod)
	lengthTag := ""
	if signer.length >= 0 {
		body = body[:signer.length]
		lengthTag = fmt.Sprintf("l=%d; ", signer.length)
	}
	bodyHash := sha256.Sum256([]byte(body))
	field := fmt.Sprintf("DKIM-Signature: v=1; a=%s; c=%s/%s; d=%s; s=%s;\r\n\th=%s; %s%sbh=%s;\r\n\tb=\r\n",
		signer.algorithm, headerMethod, bodyMethod, signer.domain, signer.selector, signer.headers, lengthTag, signer.extra,
		base64.StdEncoding.EncodeToString(bodyHash[:]))
	parsed, err := dkimSignature{}.parse(newMessageField(field))
	if err != nil {
		t.Fatalf("test signature does not parse: %v", err)
	}
	digest := sha256.Sum256(dkimVerifier{}.signedData(raw, parsed))
	value := base64.StdEncoding.EncodeToString(signer.signature(t, digest[:]))
	folded := value[:20] + "\r\n\t " + value[20:]
	return strings.Replace(field, "b=\r\n", "b="+folded+"\r\n", 1) + message
}

func (signer testSigner) signature(t *testing.T, digest []byte) []byte {
	if signer.edKey != nil {
		return ed25519.Sign(signer.edKey, digest)
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, signer.rsaKey, crypto.SHA256, digest)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}
