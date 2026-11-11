package dkim

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var testRSAOnce sync.Once
var testRSAKey *rsa.PrivateKey

func testRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testRSAOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		testRSAKey = key
	})
	return testRSAKey
}

func testEd25519(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return private
}

func testKeys(t *testing.T) map[string]Key {
	return map[string]Key{
		"rsa":     {Domain: "example.com", Selector: "s1", Signer: testRSA(t)},
		"ed25519": {Domain: "example.com", Selector: "ed1", Signer: testEd25519(t)},
	}
}

func testMessage(headers string, body string) []byte {
	return []byte(headers + "\r\n" + body)
}

func testSignAndVerify(t *testing.T, key Key, message []byte) testSignature {
	t.Helper()
	signed, err := Signer{}.Sign(SignRequest{Message: message, Key: key, Time: time.Unix(1700000000, 0)})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !strings.HasSuffix(string(signed), string(message)) {
		t.Fatal("signed output does not end with the original message")
	}
	if !strings.HasPrefix(string(signed), "DKIM-Signature: ") {
		t.Fatal("signed output does not start with DKIM-Signature")
	}
	signature, err := testVerify(signed, key.Signer.Public())
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, signed)
	}
	return signature
}

func TestSignVerifiesForEveryMessageShape(t *testing.T) {
	cases := map[string][]byte{
		"plain": testMessage("From: Alice <alice@example.com>\r\nTo: bob@other.test\r\nSubject: Hello\r\n",
			"Hello Bob\r\n"),
		"folded headers": testMessage("From: Alice\r\n <alice@example.com>\r\nTo: bob@other.test,\r\n\tcarol@other.test\r\nSubject: A long\r\n  folded   subject\r\n",
			"Body\r\n"),
		"whitespace runs": testMessage("From:   alice@example.com  \r\nSubject:\t\tTabs  and   spaces \t\r\n",
			"Line  with   runs \t \r\n\t indented\t\r\nend   \r\n"),
		"trailing empty lines": testMessage("From: alice@example.com\r\nSubject: x\r\n",
			"Text\r\n\r\n\r\n  \r\n\t\r\n"),
		"empty body":          testMessage("From: alice@example.com\r\nSubject: empty\r\n", ""),
		"body without crlf":   testMessage("From: alice@example.com\r\n", "no line end"),
		"subdomain from":      testMessage("From: alice@mail.example.com\r\n", "x\r\n"),
		"upper case domain":   testMessage("From: alice@Mail.EXAMPLE.Com\r\n", "x\r\n"),
		"repeated references": testMessage("From: alice@example.com\r\nReferences: <a@x>\r\nReferences: <b@x>\r\nTo: b@other.test\r\n", "x\r\n"),
		"empty value":         testMessage("From: alice@example.com\r\nSubject:\r\n", "x\r\n"),
		"mime message": testMessage("From: =?utf-8?q?Al=C3=AFce?= <alice@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n",
			"caf=C3=A9\r\n"),
	}
	for keyName, key := range testKeys(t) {
		for caseName, message := range cases {
			t.Run(keyName+"/"+caseName, func(t *testing.T) {
				testSignAndVerify(t, key, message)
			})
		}
	}
}

func TestSignWritesTheExpectedTags(t *testing.T) {
	key := testKeys(t)["rsa"]
	key.Domain = "Example.COM"
	key.Selector = "S1"
	message := testMessage("From: alice@example.com\r\nTo: bob@other.test\r\nX-Other: no\r\nSubject: hi\r\nDate: Mon, 1 Jan 2024 00:00:00 +0000\r\nMessage-ID: <1@example.com>\r\n", "x\r\n")
	signature := testSignAndVerify(t, key, message)
	want := map[string]string{"v": "1", "a": "rsa-sha256", "c": "relaxed/relaxed", "d": "example.com", "s": "s1", "t": "1700000000", "h": "from:to:subject:date:message-id:from"}
	for tag, value := range want {
		if signature.tags[tag] != value {
			t.Errorf("tag %s = %q, want %q", tag, signature.tags[tag], value)
		}
	}
}

func TestSignUsesRequestedHeaders(t *testing.T) {
	key := testKeys(t)["ed25519"]
	message := testMessage("From: alice@example.com\r\nX-Custom: one\r\nSubject: hi\r\n", "x\r\n")
	signed, err := Signer{}.Sign(SignRequest{Message: message, Key: key, Headers: []string{"X-Custom", "Subject"}})
	if err != nil {
		t.Fatal(err)
	}
	signature, err := testVerify(signed, key.Signer.Public())
	if err != nil {
		t.Fatal(err)
	}
	if signature.tags["h"] != "x-custom:subject:from" {
		t.Errorf("h = %q", signature.tags["h"])
	}
}

func TestSignUsesNowWhenTimeIsZero(t *testing.T) {
	key := testKeys(t)["ed25519"]
	before := time.Now().Unix()
	signed, err := Signer{}.Sign(SignRequest{Message: testMessage("From: a@example.com\r\n", ""), Key: key})
	if err != nil {
		t.Fatal(err)
	}
	signature, err := testVerify(signed, key.Signer.Public())
	if err != nil {
		t.Fatal(err)
	}
	stamp, _ := strconv.ParseInt(signature.tags["t"], 10, 64)
	if stamp < before || stamp > time.Now().Unix() {
		t.Errorf("t = %d, not now", stamp)
	}
}

func TestSignEmptyBodyHash(t *testing.T) {
	signature := testSignAndVerify(t, testKeys(t)["ed25519"], testMessage("From: a@example.com\r\n", "\r\n\r\n"))
	empty := sha256.Sum256(nil)
	if signature.tags["bh"] != base64.StdEncoding.EncodeToString(empty[:]) {
		t.Errorf("bh = %q", signature.tags["bh"])
	}
}

func TestSignDetectsTampering(t *testing.T) {
	key := testKeys(t)["rsa"]
	signed, err := Signer{}.Sign(SignRequest{Message: testMessage("From: a@example.com\r\nSubject: one\r\n", "body\r\n"), Key: key})
	if err != nil {
		t.Fatal(err)
	}
	changedHeader := []byte(strings.Replace(string(signed), "Subject: one", "Subject: two", 1))
	if _, err := testVerify(changedHeader, key.Signer.Public()); err == nil {
		t.Error("changed subject still verifies")
	}
	changedBody := []byte(strings.Replace(string(signed), "body", "Body", 1))
	if _, err := testVerify(changedBody, key.Signer.Public()); err == nil {
		t.Error("changed body still verifies")
	}
	addedFrom := []byte(strings.Replace(string(signed), "From: a@example.com", "From: a@example.com\r\nFrom: evil@example.com", 1))
	if _, err := testVerify(addedFrom, key.Signer.Public()); err == nil {
		t.Error("added From still verifies")
	}
}

func TestSignFoldsLongLines(t *testing.T) {
	key := testKeys(t)["rsa"]
	signed, err := Signer{}.Sign(SignRequest{Message: testMessage("From: a@example.com\r\n", "x\r\n"), Key: key})
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(string(signed), "\r\nFrom:")
	for _, line := range strings.Split(header, "\r\n") {
		if len(line) > 200 {
			t.Errorf("line too long (%d): %q", len(line), line)
		}
	}
}

func TestSignRefusesAnotherDomain(t *testing.T) {
	cases := map[string]string{
		"other domain":    "From: alice@other.test\r\n",
		"suffix trick":    "From: alice@badexample.com\r\n",
		"second address":  "From: alice@example.com, eve@other.test\r\n",
		"second header":   "From: alice@example.com\r\nFrom: eve@other.test\r\n",
		"missing from":    "Subject: none\r\n",
		"unparsable from": "From: not an address\r\n",
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Signer{}.Sign(SignRequest{Message: testMessage(headers, "x\r\n"), Key: testKeys(t)["ed25519"]})
			var mismatch DomainMismatch
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want DomainMismatch", err)
			}
			if mismatch.Key != "example.com" {
				t.Errorf("Key = %q", mismatch.Key)
			}
		})
	}
}

func TestSignRefusesMalformedMessages(t *testing.T) {
	cases := map[string]string{
		"no header end":     "From: a@example.com\r\nbody",
		"line without name": "From: a@example.com\r\nnot a header\r\n\r\nbody",
		"leading fold":      " folded\r\nFrom: a@example.com\r\n\r\nbody",
		"bare line feed":    "From: a@example.com\nSubject: x\r\n\r\nbody",
	}
	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Signer{}.Sign(SignRequest{Message: []byte(message), Key: testKeys(t)["ed25519"]})
			var malformed MalformedMessage
			if !errors.As(err, &malformed) {
				t.Fatalf("err = %v, want MalformedMessage", err)
			}
			if malformed.Error() == "" {
				t.Error("empty message")
			}
		})
	}
}

type testFakeSigner struct {
	public crypto.PublicKey
}

func (fake testFakeSigner) Public() crypto.PublicKey { return fake.public }

func (fake testFakeSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("hardware secret 0xdeadbeef failed")
}

func TestSignRefusesBadKeys(t *testing.T) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	small := &rsa.PublicKey{N: new(big.Int).Lsh(big.NewInt(1), 511), E: 65537}
	message := testMessage("From: a@example.com\r\n", "x\r\n")
	cases := map[string]struct {
		key  Key
		want any
	}{
		"nil signer":      {Key{Domain: "example.com", Selector: "s1"}, &UnsupportedKey{}},
		"ecdsa":           {Key{Domain: "example.com", Selector: "s1", Signer: ecdsaKey}, &UnsupportedKey{}},
		"small rsa":       {Key{Domain: "example.com", Selector: "s1", Signer: testFakeSigner{public: small}}, &UnsupportedKey{}},
		"signer fails":    {Key{Domain: "example.com", Selector: "s1", Signer: testFakeSigner{public: testRSA(t).Public()}}, &UnsupportedKey{}},
		"selector inject": {Key{Domain: "example.com", Selector: "s1; d=evil.test", Signer: testRSA(t)}, &InvalidKeyName{}},
		"empty selector":  {Key{Domain: "example.com", Signer: testRSA(t)}, &InvalidKeyName{}},
		"domain path":     {Key{Domain: "example.com/x", Selector: "s1", Signer: testRSA(t)}, &InvalidKeyName{}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Signer{}.Sign(SignRequest{Message: message, Key: test.key})
			if err == nil {
				t.Fatal("no error")
			}
			if !errors.As(err, test.want) {
				t.Fatalf("err = %T %v", err, err)
			}
			if strings.Contains(err.Error(), "deadbeef") {
				t.Error("error leaks signer detail")
			}
		})
	}
}

func TestSmallRSAErrorNamesBits(t *testing.T) {
	small := &rsa.PublicKey{N: new(big.Int).Lsh(big.NewInt(1), 511), E: 65537}
	err := rsaMethod{}.check(small)
	if err == nil || !strings.Contains(err.Error(), "512 bits") {
		t.Fatalf("err = %v", err)
	}
}
