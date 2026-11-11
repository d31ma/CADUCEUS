package recipients

import "testing"

func classify(destination Destination) Kind {
	return Classifier{}.Kind(destination)
}

func TestMemberOfTheCallersDomainIsAMember(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.com", Hosted: true, LocalKind: Kinds{}.Member()})
	if got != (Kinds{}).Member() {
		t.Fatalf("got %q", got)
	}
}

func TestChannelOfTheCallersDomainIsAChannel(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.com", Hosted: true, LocalKind: Kinds{}.Channel()})
	if got != (Kinds{}).Channel() {
		t.Fatalf("got %q", got)
	}
}

func TestUnknownAddressOfTheCallersDomainIsExternal(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.com", Hosted: true})
	if got != (Kinds{}).External() {
		t.Fatalf("got %q", got)
	}
}

func TestMemberOfAnotherHostedDomainIsOtherDomain(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.org", Hosted: true, LocalKind: Kinds{}.Member()})
	if got != (Kinds{}).OtherDomain() {
		t.Fatalf("got %q", got)
	}
}

func TestMemberOfAnotherHostedDomainIsNeverOfferedChat(t *testing.T) {
	kind := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.org", Hosted: true, LocalKind: Kinds{}.Member()})
	if err := (Policy{}).Check(kind, Deliveries{}.Chat()); err == nil {
		t.Fatal("chat must never cross a domain")
	}
}

func TestChannelOfAnotherHostedDomainIsOtherDomain(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.org", Hosted: true, LocalKind: Kinds{}.Channel()})
	if got != (Kinds{}).OtherDomain() {
		t.Fatalf("got %q", got)
	}
}

func TestAddressThatIsNotHostedIsExternal(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "partner.test"})
	if got != (Kinds{}).External() {
		t.Fatalf("got %q", got)
	}
}

func TestOtherDomainKindIsNeverPassedThroughFromTheLookup(t *testing.T) {
	got := classify(Destination{CallerDomainId: "example.com", AddressDomainId: "example.com", Hosted: true, LocalKind: Kinds{}.OtherDomain()})
	if got != (Kinds{}).External() {
		t.Fatalf("got %q", got)
	}
}
