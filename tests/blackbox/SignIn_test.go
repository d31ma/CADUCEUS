//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

func expectSameAccount(t *testing.T, first session, second session) {
	t.Helper()
	if first.Me.MemberId == "" || first.Me.MemberId != second.Me.MemberId {
		t.Fatalf("want one account, got %+v and %+v", first.Me, second.Me)
	}
}

func TestAcId9PrefixAndFullAddressSignInToTheSameAccount(t *testing.T) {
	stack := NewStack(t)
	password := stack.Password(t, "ANA")
	byPrefix := signIn(t, stack, "ana", password)
	expectSameAccount(t, byPrefix, signIn(t, stack, " ANA ", password))
	expectSameAccount(t, byPrefix, signIn(t, stack, "ana@example.com", password))
	expectSameAccount(t, byPrefix, signIn(t, stack, "Ana@Example.COM", password))
	expectSameAccount(t, byPrefix, signIn(t, stack, "ana@example.org", password))
}

func TestAcId9PrefixOnlyOpensTheDefaultDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	if ana.Me.DomainId != "example.com" || ana.Me.PrimaryAddress != "ana@example.com" || ana.Me.Username != "ana" {
		t.Fatalf("want ana in her default domain example.com, got %+v", ana.Me)
	}
}

func TestAcId9FullAddressOpensThatDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixtureIn(t, stack, "ANA", "example.org")
	if ana.Me.DomainId != "example.org" || ana.Me.PrimaryAddress != "ana@example.org" {
		t.Fatalf("want ana in example.org, got %+v", ana.Me)
	}
}

func TestAcId9SessionListsEveryDomainOfThePerson(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	want := []domainChoice{{DomainId: "example.com", Admin: true, Default: true}, {DomainId: "example.org", Admin: false, Default: false}}
	if len(ana.Domains) != len(want) || ana.Domains[0] != want[0] || ana.Domains[1] != want[1] {
		t.Fatalf("want %+v, got %+v", want, ana.Domains)
	}
}

func TestAcId9WrongPasswordAndUnknownPrefixGetTheSameAnswer(t *testing.T) {
	stack := NewStack(t)
	dan := createPerson(t, stack, fixture(t, stack, "ANA").AccessToken)
	wrongPassword := signInReply(t, stack, dan.Username, "not-the-password").Expect(t, http.StatusUnauthorized).Error(t)
	unknownPrefix := signInReply(t, stack, freshPrefix(), "not-the-password").Expect(t, http.StatusUnauthorized).Error(t)
	if wrongPassword != unknownPrefix {
		t.Fatalf("the answers differ: %+v and %+v", wrongPassword, unknownPrefix)
	}
}

func TestAcId9InvalidLoginIsRefusedLikeAWrongPassword(t *testing.T) {
	stack := NewStack(t)
	password := stack.Password(t, "ANA")
	signInReply(t, stack, "ana+news", password).Expect(t, http.StatusUnauthorized)
	signInReply(t, stack, "aná", password).Expect(t, http.StatusUnauthorized)
	signInReply(t, stack, "", password).Expect(t, http.StatusUnauthorized)
	signInReply(t, stack, "ana@", password).Expect(t, http.StatusUnauthorized)
}

func TestAcId9AddressInADomainThePersonIsNotInIsNotAMember(t *testing.T) {
	stack := NewStack(t)
	signInReply(t, stack, "carol@example.com", stack.Password(t, "CAROL")).ExpectError(t, http.StatusForbidden, "not_a_member")
}

func TestAcId9WrongPasswordWithAForeignDomainStaysUnauthenticated(t *testing.T) {
	stack := NewStack(t)
	dan := createPerson(t, stack, fixture(t, stack, "ANA").AccessToken)
	signInReply(t, stack, dan.Username+"@example.org", "not-the-password").Expect(t, http.StatusUnauthorized)
}

func TestAcId9PersonInNoDomainIsRefusedAfterTheRightPassword(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	removeMemberReply(t, stack, ana.AccessToken, dan.Id).Expect(t, http.StatusNoContent)
	signInReply(t, stack, dan.Username, dan.Password).ExpectError(t, http.StatusForbidden, "no_domain")
}

func TestAcId14OperatorWithNoDomainGetsAnInstallWideSession(t *testing.T) {
	stack := NewStack(t)
	operator := fixture(t, stack, "OPERATOR")
	if operator.Me.DomainId != "*" || operator.Me.PrimaryAddress != "" || len(operator.Domains) != 0 {
		t.Fatalf("want an install-wide session, got %+v with domains %+v", operator.Me, operator.Domains)
	}
	stack.Call(t, http.MethodGet, "/api/v1/directory/domains", operator.AccessToken, nil).Expect(t, http.StatusOK)
}

func TestAcId14InstallWideSessionReadsNoDomainData(t *testing.T) {
	stack := NewStack(t)
	operator := fixture(t, stack, "OPERATOR")
	reply := stack.Call(t, http.MethodGet, "/api/v1/messaging/conversations", operator.AccessToken, nil)
	if reply.Status != http.StatusForbidden && reply.Status != http.StatusNotFound {
		t.Fatalf("want 403 or 404 for domain data with domain *, got %d: %s", reply.Status, reply.Body)
	}
}

func TestAcId12NewPersonMustChangeTheFirstPassword(t *testing.T) {
	stack := NewStack(t)
	dan := createPerson(t, stack, fixture(t, stack, "ANA").AccessToken)
	first := signIn(t, stack, dan.Username, dan.Password)
	if !first.MustChangePassword || first.Me.PrimaryAddress != dan.Username+"@example.com" {
		t.Fatalf("want a forced password change for %s@example.com, got %+v", dan.Username, first)
	}
	change := map[string]string{"currentPassword": dan.Password, "newPassword": dan.Password + "-Changed"}
	stack.Call(t, http.MethodPost, "/api/v1/identity/password", first.AccessToken, change).Expect(t, http.StatusOK)
	if signIn(t, stack, dan.Username, dan.Password+"-Changed").MustChangePassword {
		t.Fatal("the forced password change is still on after the change")
	}
}

func TestAcId9DefaultDomainIsTheOneThePersonChose(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	added(t, stack, operatorAddReply(t, stack, operatorToken(t, stack), "example.org", dan.Username, false))
	danSession := signIn(t, stack, dan.Username, dan.Password)
	choice := map[string]string{"domainId": "example.org"}
	stack.Call(t, http.MethodPut, "/api/v1/directory/me/default-domain", danSession.AccessToken, choice).Expect(t, http.StatusNoContent)
	again := signIn(t, stack, dan.Username, dan.Password)
	want := []domainChoice{{DomainId: "example.com", Default: false}, {DomainId: "example.org", Default: true}}
	if again.Me.DomainId != "example.org" || len(again.Domains) != 2 || again.Domains[0] != want[0] || again.Domains[1] != want[1] {
		t.Fatalf("want the chosen default example.org, got %+v with domains %+v", again.Me, again.Domains)
	}
}

func TestAcId9DefaultDomainMustBeOneOfThePersonsDomains(t *testing.T) {
	stack := NewStack(t)
	bo := fixture(t, stack, "BO")
	choice := map[string]string{"domainId": "example.org"}
	stack.Call(t, http.MethodPut, "/api/v1/directory/me/default-domain", bo.AccessToken, choice).Expect(t, http.StatusNotFound)
}
