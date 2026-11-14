package admission_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/packages/oidc"
)

type realtimeJoinToken struct {
	Token  string      `json:"token"`
	Keys   oidc.KeySet `json:"keys"`
	CallId string      `json:"callId"`
	Now    int64       `json:"now"`
}

func readRealtimeJoinToken(t *testing.T) realtimeJoinToken {
	t.Helper()
	raw, err := os.ReadFile("testdata/realtime-join-token.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture := realtimeJoinToken{}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestATokenSignedByRealtimeIsAdmitted(t *testing.T) {
	fixture := readRealtimeJoinToken(t)
	clock := &fixedClock{now: time.Unix(fixture.Now, 0)}
	check := admission.NewJoinTokenCheck(admission.NewKeyBook(&countingFetcher{set: fixture.Keys}, "http://realtime/keys", clock), clock)
	claims, err := check.Admit(fixture.Token, fixture.CallId)
	if err != nil {
		t.Fatal(err)
	}
	want := admission.JoinClaims{CallId: fixture.CallId, DeviceId: "device-1", DomainId: "example.com", Audience: "calls", IssuedAt: fixture.Now, ExpiresAt: fixture.Now + 60}
	if claims != want {
		t.Fatalf("claims %+v", claims)
	}
}

func TestATokenSignedByRealtimeEndsAfterSixtySeconds(t *testing.T) {
	fixture := readRealtimeJoinToken(t)
	clock := &fixedClock{now: time.Unix(fixture.Now+60, 0)}
	check := admission.NewJoinTokenCheck(admission.NewKeyBook(&countingFetcher{set: fixture.Keys}, "http://realtime/keys", clock), clock)
	if _, err := check.Admit(fixture.Token, fixture.CallId); err == nil {
		t.Fatal("an expired realtime token was admitted")
	}
}
