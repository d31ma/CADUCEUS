package webpush

import (
	"errors"
	"strings"
	"testing"
)

func vapidDocument(public string, private string, subject string) string {
	return `{"yon":{"notify":{"vapid":{"public":{"key":"` + public + `"},"private":{"key":"` + private + `"},"subject":"` + subject + `"}}}}`
}

const testPublicKey = "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8"
const testPrivateKey = "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"

func TestReaderReadsTheVapidKeys(t *testing.T) {
	keys, err := VapidSettingsReader{}.FromApplicationJSON(vapidDocument(testPublicKey, testPrivateKey, "https://mesail.dev"))
	if err != nil {
		t.Fatal(err)
	}
	expected := VapidKeys{PublicKey: testPublicKey, PrivateKey: testPrivateKey, Subject: "https://mesail.dev"}
	if keys != expected {
		t.Fatalf("keys %+v", keys)
	}
}

func TestReaderRefusesMissingOrInvalidSettings(t *testing.T) {
	otherPublic := "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	cases := map[string]string{
		"not json":          "{",
		"empty":             "",
		"no notify section": `{"yon":{}}`,
		"no subject":        vapidDocument(testPublicKey, testPrivateKey, ""),
		"no public key":     vapidDocument("", testPrivateKey, "mailto:a@example.com"),
		"no private key":    vapidDocument(testPublicKey, "", "mailto:a@example.com"),
		"bad subject":       vapidDocument(testPublicKey, testPrivateKey, "http://example.com"),
		"keys do not match": vapidDocument(otherPublic, testPrivateKey, "mailto:a@example.com"),
		"private not text":  vapidDocument(testPublicKey, "%%%", "mailto:a@example.com"),
		"private is zero":   vapidDocument(testPublicKey, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "mailto:a@example.com"),
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			keys, err := VapidSettingsReader{}.FromApplicationJSON(document)
			if !errors.Is(err, NotConfigured{}) {
				t.Fatalf("error %v", err)
			}
			if keys != (VapidKeys{}) {
				t.Fatalf("keys leaked %+v", keys)
			}
			if !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("message %q", err.Error())
			}
		})
	}
}
