package recipients

import (
	"errors"
	"testing"
)

func TestMemberDefaultsToChat(t *testing.T) {
	if got := (Policy{}).Default(Kinds{}.Member()); got != (Deliveries{}).Chat() {
		t.Fatalf("got %q", got)
	}
}

func TestExternalDefaultsToEmail(t *testing.T) {
	if got := (Policy{}).Default(Kinds{}.External()); got != (Deliveries{}).Email() {
		t.Fatalf("got %q", got)
	}
}

func TestChannelDefaultsToChat(t *testing.T) {
	if got := (Policy{}).Default(Kinds{}.Channel()); got != (Deliveries{}).Chat() {
		t.Fatalf("got %q", got)
	}
}

func TestMemberMayBeSentEmail(t *testing.T) {
	if err := (Policy{}).Check(Kinds{}.Member(), Deliveries{}.Email()); err != nil {
		t.Fatal(err)
	}
}

func TestMemberMayBeSentBoth(t *testing.T) {
	if err := (Policy{}).Check(Kinds{}.Member(), Deliveries{}.Both()); err != nil {
		t.Fatal(err)
	}
}

func TestExternalMayNotBeSentChat(t *testing.T) {
	err := (Policy{}).Check(Kinds{}.External(), Deliveries{}.Chat())
	var refused RefusedDelivery
	if !errors.As(err, &refused) {
		t.Fatalf("got %v", err)
	}
}

func TestChannelMayNotBeSentEmail(t *testing.T) {
	if err := (Policy{}).Check(Kinds{}.Channel(), Deliveries{}.Email()); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestUnknownKindAllowsNothing(t *testing.T) {
	if got := (Policy{}).Allowed(Kind("robot")); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestRefusalNamesTheDelivery(t *testing.T) {
	message := RefusedDelivery{Kind: Kinds{}.External(), Wanted: Deliveries{}.Chat()}.Error()
	if message != `delivery "chat" is not allowed for a external recipient` {
		t.Fatalf("got %q", message)
	}
}

func TestOtherHostedDomainDefaultsToEmail(t *testing.T) {
	if got := (Policy{}).Default(Kinds{}.OtherDomain()); got != (Deliveries{}).Email() {
		t.Fatalf("got %q", got)
	}
}

func TestOtherHostedDomainMayNotBeSentChat(t *testing.T) {
	if err := (Policy{}).Check(Kinds{}.OtherDomain(), Deliveries{}.Chat()); err == nil {
		t.Fatal("chat must never cross a domain")
	}
}
