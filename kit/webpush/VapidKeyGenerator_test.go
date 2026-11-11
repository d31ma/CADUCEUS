package webpush

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestGenerateDerivesThePublicKeyFromTheRandomScalar(t *testing.T) {
	scalar := mustDecode(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw")
	keys, err := VapidKeyGenerator{}.Generate(bytes.NewReader(scalar))
	if err != nil {
		t.Fatal(err)
	}
	if keys.PrivateKey != "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw" {
		t.Fatalf("private %s", keys.PrivateKey)
	}
	if keys.PublicKey != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("public %s", keys.PublicKey)
	}
}

func TestGeneratedKeysAreAcceptedByTheSettingsReader(t *testing.T) {
	keys, err := VapidKeyGenerator{}.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	document := `{"yon":{"notify":{"vapid":{"public":{"key":"` + keys.PublicKey + `"},"private":{"key":"` + keys.PrivateKey + `"},"subject":"mailto:ops@example.com"}}}}`
	read, err := VapidSettingsReader{}.FromApplicationJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if read.PublicKey != keys.PublicKey || read.Subject != "mailto:ops@example.com" {
		t.Fatalf("read %+v", read)
	}
}

func TestGenerateReportsRandomFailures(t *testing.T) {
	_, err := VapidKeyGenerator{}.Generate(failingReader{})
	if err == nil {
		t.Fatal("expected an error for a failing reader")
	}
	_, err = VapidKeyGenerator{}.Generate(bytes.NewReader(make([]byte, 32)))
	if err == nil {
		t.Fatal("expected an error for a zero scalar")
	}
}
