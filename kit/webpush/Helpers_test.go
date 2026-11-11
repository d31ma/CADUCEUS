package webpush

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

type testSubscriber struct {
	privateKey   *ecdh.PrivateKey
	authSecret   []byte
	subscription Subscription
}

func newTestSubscriber(t *testing.T, endpoint string) testSubscriber {
	t.Helper()
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	_, err = rand.Read(authSecret)
	if err != nil {
		t.Fatal(err)
	}
	subscription := Subscription{Endpoint: endpoint, P256dh: Base64Text{}.Encode(privateKey.PublicKey().Bytes()), Auth: Base64Text{}.Encode(authSecret)}
	return testSubscriber{privateKey: privateKey, authSecret: authSecret, subscription: subscription}
}

func mustDecode(t *testing.T, value string) []byte {
	t.Helper()
	raw, err := Base64Text{}.Decode(value)
	if err != nil {
		t.Fatalf("decode %q: %v", value, err)
	}
	return raw
}

func decryptForTest(t *testing.T, body []byte, privateKey *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("body too short: %d", len(body))
	}
	salt := body[:16]
	recordSize := binary.BigEndian.Uint32(body[16:20])
	if recordSize != 4096 {
		t.Fatalf("record size %d", recordSize)
	}
	keyLength := int(body[20])
	serverPublic := body[21 : 21+keyLength]
	record := body[21+keyLength:]
	serverKey, err := ecdh.P256().NewPublicKey(serverPublic)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := privateKey.ECDH(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	info := "WebPush: info\x00" + string(privateKey.PublicKey().Bytes()) + string(serverPublic)
	inputKey, err := hkdf.Key(sha256.New, secret, authSecret, info, 32)
	if err != nil {
		t.Fatal(err)
	}
	key, err := hkdf.Key(sha256.New, inputKey, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := hkdf.Key(sha256.New, inputKey, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	padded, err := aead.Open(nil, nonce, record, nil)
	if err != nil {
		t.Fatalf("open record: %v", err)
	}
	if len(padded) == 0 || padded[len(padded)-1] != 2 {
		t.Fatalf("missing last record delimiter")
	}
	return bytes.TrimSuffix(padded, []byte{2})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errFailingReader
}

type readerError struct{}

func (readerError) Error() string {
	return "reader failed"
}

var errFailingReader = readerError{}
