package webpush

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

func rfcSubscription() Subscription {
	return Subscription{
		Endpoint: "https://push.example.net/push/JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV",
		P256dh:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
	}
}

func rfcRandom(t *testing.T) io.Reader {
	salt := mustDecode(t, "DGv6ra1nlYgDCS1FRnbzlw")
	serverPrivate := mustDecode(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw")
	return bytes.NewReader(append(salt, serverPrivate...))
}

func TestEncryptMatchesRfc8291AppendixA(t *testing.T) {
	plaintext := mustDecode(t, "V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24")
	body, err := NewEncryptor(rfcRandom(t)).Encrypt(rfcSubscription(), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	expected := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml" +
		"mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT" +
		"pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if (Base64Text{}).Encode(body) != expected {
		t.Fatalf("encrypted body\n got %s\nwant %s", Base64Text{}.Encode(body), expected)
	}
}

func TestRfc8291VectorDecryptsWithUserAgentKey(t *testing.T) {
	plaintext := mustDecode(t, "V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24")
	body, err := NewEncryptor(rfcRandom(t)).Encrypt(rfcSubscription(), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	userAgent, err := ecdh.P256().NewPrivateKey(mustDecode(t, "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"))
	if err != nil {
		t.Fatal(err)
	}
	decrypted := decryptForTest(t, body, userAgent, mustDecode(t, "BTBZMqHH6r4Tts7J_aSIgg"))
	if string(decrypted) != "When I grow up, I want to be a watermelon" {
		t.Fatalf("decrypted %q", decrypted)
	}
}

func TestEncryptRoundTripWithRandomKeys(t *testing.T) {
	subscriber := newTestSubscriber(t, "https://fcm.googleapis.com/fcm/send/x")
	plaintext := []byte(`{"title":"hello"}`)
	body, err := NewEncryptor(rand.Reader).Encrypt(subscriber.subscription, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decryptForTest(t, body, subscriber.privateKey, subscriber.authSecret), plaintext) {
		t.Fatal("round trip failed")
	}
}

func TestEncryptAcceptsTheLargestPlaintextInOneMessage(t *testing.T) {
	subscriber := newTestSubscriber(t, "https://fcm.googleapis.com/fcm/send/x")
	body, err := NewEncryptor(rand.Reader).Encrypt(subscriber.subscription, bytes.Repeat([]byte("a"), 3993))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 4096 {
		t.Fatalf("body length %d", len(body))
	}
}

func TestEncryptRefusesPlaintextLargerThanOneRecord(t *testing.T) {
	subscriber := newTestSubscriber(t, "https://fcm.googleapis.com/fcm/send/x")
	_, err := NewEncryptor(rand.Reader).Encrypt(subscriber.subscription, bytes.Repeat([]byte("a"), 3994))
	tooLarge := PayloadTooLarge{}
	if !errors.As(err, &tooLarge) || tooLarge.Size != 3994 || tooLarge.Limit != 3993 {
		t.Fatalf("error %v", err)
	}
	if !strings.Contains(err.Error(), "3994") {
		t.Fatalf("message %q", err.Error())
	}
}

func TestEncryptRefusesInvalidSubscriptionKeys(t *testing.T) {
	valid := rfcSubscription()
	cases := map[string]struct {
		subscription Subscription
		field        string
	}{
		"p256dh not base64":  {Subscription{P256dh: "%%%", Auth: valid.Auth}, "p256dh"},
		"p256dh not a point": {Subscription{P256dh: Base64Text{}.Encode(bytes.Repeat([]byte{4}, 65)), Auth: valid.Auth}, "p256dh"},
		"auth not base64":    {Subscription{P256dh: valid.P256dh, Auth: "%%%"}, "auth"},
		"auth wrong length":  {Subscription{P256dh: valid.P256dh, Auth: "AAAA"}, "auth"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewEncryptor(rand.Reader).Encrypt(testCase.subscription, []byte("x"))
			invalid := InvalidSubscription{}
			if !errors.As(err, &invalid) || invalid.Field != testCase.field {
				t.Fatalf("error %v", err)
			}
			if !strings.Contains(err.Error(), testCase.field) {
				t.Fatalf("message %q", err.Error())
			}
		})
	}
}

func TestEncryptAcceptsPaddedBase64Keys(t *testing.T) {
	subscription := rfcSubscription()
	subscription.Auth += "=="
	_, err := NewEncryptor(rfcRandom(t)).Encrypt(subscription, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncryptReportsRandomFailures(t *testing.T) {
	cases := map[string]io.Reader{
		"no salt":         failingReader{},
		"short salt":      bytes.NewReader(make([]byte, 8)),
		"no server key":   bytes.NewReader(make([]byte, 16)),
		"zero server key": bytes.NewReader(make([]byte, 48)),
	}
	for name, random := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewEncryptor(random).Encrypt(rfcSubscription(), []byte("x"))
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
