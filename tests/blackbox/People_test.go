//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

func resetPasswordReply(t *testing.T, stack Stack, token string, personId string, newPassword string) Reply {
	t.Helper()
	request := map[string]string{"newPassword": newPassword}
	return stack.Call(t, http.MethodPost, "/api/v1/directory/people/"+personId+"/password", token, request)
}

func TestAcId13AdminOfEveryDomainOfThePersonResetsThePassword(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	resetPasswordReply(t, stack, ana.AccessToken, dan.Id, dan.Password+"-Reset").Expect(t, http.StatusNoContent)
	if !signIn(t, stack, dan.Username, dan.Password+"-Reset").MustChangePassword {
		t.Fatal("want a forced password change after a reset")
	}
}

func TestAcId13AdminOfOnlySomeDomainsOfThePersonCannotReset(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	operator := operatorToken(t, stack)
	added(t, operatorAddReply(t, stack, operator, createDomain(t, stack, operator), dan.Username, false))
	resetPasswordReply(t, stack, ana.AccessToken, dan.Id, dan.Password+"-Reset").Expect(t, http.StatusForbidden)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id+"/totp", ana.AccessToken, nil).Expect(t, http.StatusForbidden)
	change := map[string]string{"state": "disabled"}
	stack.Call(t, http.MethodPatch, "/api/v1/directory/people/"+dan.Id, ana.AccessToken, change).Expect(t, http.StatusForbidden)
}

func TestAcId13AdminCannotResetAPersonOfAnotherDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	eve := fixtureIn(t, stack, "EVE", "example.org")
	resetPasswordReply(t, stack, ana.AccessToken, eve.Me.MemberId, "Blackbox-Never-Set-1").Expect(t, http.StatusForbidden)
}

func TestAcId13OperatorResetsAnyPerson(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	operator := operatorToken(t, stack)
	added(t, operatorAddReply(t, stack, operator, createDomain(t, stack, operator), dan.Username, false))
	resetPasswordReply(t, stack, operator, dan.Id, dan.Password+"-Reset").Expect(t, http.StatusNoContent)
}

func TestAcId4DisabledPersonLosesEveryDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	danSession := signIn(t, stack, dan.Username, dan.Password)
	change := map[string]string{"state": "disabled"}
	stack.Call(t, http.MethodPatch, "/api/v1/directory/people/"+dan.Id, operatorToken(t, stack), change).Expect(t, http.StatusOK)
	refreshReply(t, stack, danSession.SessionKey).Expect(t, http.StatusUnauthorized)
	signInReply(t, stack, dan.Username, dan.Password).Expect(t, http.StatusUnauthorized)
}

func TestAcId13OnlyAnOperatorDeletesAPerson(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id, ana.AccessToken, nil).Expect(t, http.StatusForbidden)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id, operatorToken(t, stack), nil).Expect(t, http.StatusNoContent)
	signInReply(t, stack, dan.Username, dan.Password).Expect(t, http.StatusUnauthorized)
}

func TestAcId10DeletedPersonsPrefixIsNeverReused(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	operator := operatorToken(t, stack)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id, operator, nil).Expect(t, http.StatusNoContent)
	addMemberReply(t, stack, ana.AccessToken, dan.Username, false).ExpectError(t, http.StatusConflict, "duplicate")
}

func TestPeopleLookupIsInstallWideServiceOnly(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	stack.Call(t, http.MethodGet, "/api/v1/directory/people/"+ana.Me.MemberId, ana.AccessToken, nil).Expect(t, http.StatusForbidden)
	stack.Call(t, http.MethodGet, "/api/v1/directory/people/"+ana.Me.MemberId, operatorToken(t, stack), nil).Expect(t, http.StatusForbidden)
}
