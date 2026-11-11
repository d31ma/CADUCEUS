package realtime

import (
	"errors"
	"testing"
	"time"
)

func requestFailure(t *testing.T, request PresenceRequest) string {
	t.Helper()
	invalid := Invalid{}
	err := PresenceRequestCheck{Now: presenceNow()}.Check(request)
	errors.As(err, &invalid)
	return invalid.Field
}

func TestCheckRefusesAnUnknownState(t *testing.T) {
	if field := requestFailure(t, PresenceRequest{State: "offline"}); field != "state" {
		t.Fatalf("got %q", field)
	}
}

func TestCheckRefusesDoNotDisturbWithoutAnEnd(t *testing.T) {
	if field := requestFailure(t, PresenceRequest{State: "dnd"}); field != "doNotDisturbUntil" {
		t.Fatalf("got %q", field)
	}
}

func TestCheckRefusesDoNotDisturbEndingInThePast(t *testing.T) {
	if field := requestFailure(t, PresenceRequest{State: "dnd", DoNotDisturbUntil: presenceText(-time.Minute)}); field != "doNotDisturbUntil" {
		t.Fatalf("got %q", field)
	}
}

func TestCheckAcceptsDoNotDisturbEndingLater(t *testing.T) {
	if err := (PresenceRequestCheck{Now: presenceNow()}).Check(PresenceRequest{State: "dnd", DoNotDisturbUntil: presenceText(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckAcceptsIdleWithoutAnEnd(t *testing.T) {
	if err := (PresenceRequestCheck{Now: presenceNow()}).Check(PresenceRequest{State: "idle"}); err != nil {
		t.Fatal(err)
	}
}
