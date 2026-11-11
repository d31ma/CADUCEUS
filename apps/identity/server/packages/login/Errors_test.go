package login

import "testing"

func TestAccessRevokedHasAMessage(t *testing.T) {
	same(t, "message", AccessRevoked{}.Error(), "the person's access was revoked")
}

func TestInvalidDomainHasAMessage(t *testing.T) {
	same(t, "message", InvalidDomain{}.Error(), "the domain is neither a domain name nor *")
}

func TestNoDomainHasAMessage(t *testing.T) {
	same(t, "message", NoDomain{}.Error(), "the person has no domain")
}

func TestNotAMemberHasAMessage(t *testing.T) {
	same(t, "message", NotAMember{}.Error(), "the person is not a member of this domain")
}

func TestNotFoundHasAMessage(t *testing.T) {
	same(t, "message", NotFound{}.Error(), "the person is not a member of that domain")
}
