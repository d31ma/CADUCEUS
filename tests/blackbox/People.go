//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

type memberSummary struct {
	MemberId       string `json:"memberId"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	PrimaryAddress string `json:"primaryAddress"`
	Admin          bool   `json:"admin"`
	AccountManaged bool   `json:"accountManaged"`
}

type addedMember struct {
	Member         memberSummary `json:"member"`
	ExistingPerson bool          `json:"existingPerson"`
}

type newPerson struct {
	Id       string
	Username string
	Password string
}

func addMemberRequest(username string, admin bool) map[string]any {
	return map[string]any{
		"username":      username,
		"displayName":   "Blackbox " + username,
		"firstPassword": firstPassword(username),
		"admin":         admin,
	}
}

func addMemberReply(t *testing.T, stack Stack, adminToken string, username string, admin bool) Reply {
	t.Helper()
	return stack.Call(t, http.MethodPost, "/api/v1/directory/members", adminToken, addMemberRequest(username, admin))
}

func operatorAddReply(t *testing.T, stack Stack, operatorToken string, domainId string, username string, admin bool) Reply {
	t.Helper()
	path := "/api/v1/directory/domains/" + domainId + "/members"
	return stack.Call(t, http.MethodPost, path, operatorToken, addMemberRequest(username, admin))
}

func added(t *testing.T, stack Stack, reply Reply) addedMember {
	t.Helper()
	var result addedMember
	reply.Expect(t, http.StatusCreated).Decode(t, &result)
	removeNewPersonAfterTest(t, stack, result)
	return result
}

func removeNewPersonAfterTest(t *testing.T, stack Stack, result addedMember) {
	t.Helper()
	if result.ExistingPerson {
		return
	}
	removeAfterTest(t, stack, "/api/v1/directory/people/"+result.Member.MemberId)
}

func createPerson(t *testing.T, stack Stack, adminToken string) newPerson {
	t.Helper()
	username := freshPrefix()
	result := added(t, stack, addMemberReply(t, stack, adminToken, username, false))
	return newPerson{Id: result.Member.MemberId, Username: username, Password: firstPassword(username)}
}

func operatorToken(t *testing.T, stack Stack) string {
	t.Helper()
	return fixture(t, stack, "OPERATOR").AccessToken
}

func createDomain(t *testing.T, stack Stack, operator string) string {
	t.Helper()
	domainId := freshPrefix() + ".test"
	request := map[string]string{"name": domainId}
	stack.Call(t, http.MethodPost, "/api/v1/directory/domains", operator, request).Expect(t, http.StatusCreated)
	removeAfterTest(t, stack, "/api/v1/directory/domains/"+domainId)
	return domainId
}

func removeMemberReply(t *testing.T, stack Stack, adminToken string, memberId string) Reply {
	t.Helper()
	return stack.Call(t, http.MethodDelete, "/api/v1/directory/members/"+memberId, adminToken, nil)
}

func createAliasReply(t *testing.T, stack Stack, adminToken string, address string, memberId string) Reply {
	t.Helper()
	request := map[string]string{"address": address, "memberId": memberId}
	return stack.Call(t, http.MethodPost, "/api/v1/directory/aliases", adminToken, request)
}

func addedInBulk(t *testing.T, reply Reply) addedMember {
	t.Helper()
	var result addedMember
	reply.Expect(t, http.StatusCreated).Decode(t, &result)
	return result
}
