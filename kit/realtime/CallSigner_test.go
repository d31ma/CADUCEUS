package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

func keyText(t *testing.T, key oidc.SigningKey) string {
	t.Helper()
	text, err := key.Text()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestTheSignerReadsItsKeysFromSettings(t *testing.T) {
	current, previous := signalTestKey(), signalTestKey()
	signer := CallSignerFactory{}.FromSettings(CallKeySettings{Key: keyText(t, current), Previous: keyText(t, previous)}, connectionTestTime)
	set, err := signer.KeySet()
	currentId, _ := current.KeyId()
	previousId, _ := previous.KeyId()
	if err != nil || len(set.Keys) != 2 || set.Keys[0].KeyId != currentId || set.Keys[1].KeyId != previousId || set.Keys[0].KeyType != "EC" || set.Keys[0].Curve != "P-256" {
		t.Fatalf("got %+v, %v", set, err)
	}
}

func TestTheTokenHeaderNamesTheKeyAndES256(t *testing.T) {
	key := signalTestKey()
	token, err := CallSigner{Current: key, Now: connectionTestTime}.Token("call_1", "a1", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	header := oidc.TokenHeader{}
	json.Unmarshal(raw, &header)
	keyId, _ := key.KeyId()
	if header != (oidc.TokenHeader{Algorithm: "ES256", KeyId: keyId, Type: "JWT"}) {
		t.Fatalf("got %+v", header)
	}
}

func TestASignerWithoutAKeyRefusesToSign(t *testing.T) {
	signer := CallSignerFactory{}.FromSettings(CallKeySettings{Key: "not a key"}, time.Now)
	_, tokenErr := signer.Token("call_1", "a1", "example.com")
	_, keysErr := signer.KeySet()
	refusal := SignalRefusal{}
	if !errors.As(tokenErr, &refusal) || refusal.Number() != 5042 || !errors.As(keysErr, &refusal) {
		t.Fatalf("got %v, %v", tokenErr, keysErr)
	}
}

func TestABadPreviousKeyTurnsSigningOff(t *testing.T) {
	signer := CallSignerFactory{}.FromSettings(CallKeySettings{Key: keyText(t, signalTestKey()), Previous: "broken"}, time.Now)
	if signer.Ready() == nil {
		t.Fatal("expected signing to be off")
	}
}

func TestWriteTheJoinTokenFixtureForEdgeCalls(t *testing.T) {
	path := os.Getenv("MESAIL_JOIN_TOKEN_FIXTURE")
	if path == "" {
		t.Skip("set MESAIL_JOIN_TOKEN_FIXTURE to write edge/calls/admission/testdata/realtime-join-token.json")
	}
	signer := CallSigner{Current: signalTestKey(), Now: connectionTestTime}
	token, tokenErr := signer.Token("call_FIXTURE", "device-1", "example.com")
	set, setErr := signer.KeySet()
	document, jsonErr := json.MarshalIndent(map[string]any{"token": token, "keys": set, "callId": "call_FIXTURE", "now": connectionTestTime().Unix()}, "", "  ")
	if err := errors.Join(tokenErr, setErr, jsonErr, os.WriteFile(path, append(document, '\n'), 0o644)); err != nil {
		t.Fatal(err)
	}
}
