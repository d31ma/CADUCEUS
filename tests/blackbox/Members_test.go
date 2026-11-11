//go:build blackbox

package blackbox

import (
	"net/http"
	"strings"
	"testing"
)

func expectInvalidUsername(t *testing.T, stack Stack, adminToken string, username string) {
	t.Helper()
	failure := addMemberReply(t, stack, adminToken, username, false).Expect(t, http.StatusBadRequest).Error(t)
	if failure.Code != "invalid_request" || failure.Field != "username" {
		t.Fatalf("want invalid_request on username for %q, got %+v", username, failure)
	}
}

func TestAcId12AdminAddsANewPerson(t *testing.T) {
	stack := NewStack(t)
	username := freshPrefix()
	result := added(t, addMemberReply(t, stack, fixture(t, stack, "ANA").AccessToken, username, false))
	if result.ExistingPerson || result.Member.MemberId == "" || result.Member.PrimaryAddress != username+"@example.com" {
		t.Fatalf("want a new person %s@example.com, got %+v", username, result)
	}
}

func TestAcId12AdminAddsAnExistingPersonWithoutAPassword(t *testing.T) {
	stack := NewStack(t)
	dan := createPerson(t, stack, fixture(t, stack, "ANA").AccessToken)
	request := map[string]any{"username": dan.Username, "admin": false}
	var result addedMember
	stack.Call(t, http.MethodPost, "/api/v1/directory/domains/example.org/members", operatorToken(t, stack), request).Expect(t, http.StatusCreated).Decode(t, &result)
	if !result.ExistingPerson || result.Member.MemberId != dan.Id || result.Member.PrimaryAddress != dan.Username+"@example.org" {
		t.Fatalf("want the existing person %s in example.org, got %+v", dan.Id, result)
	}
}

func TestAcId12NewPersonNeedsANameAndAFirstPassword(t *testing.T) {
	stack := NewStack(t)
	request := map[string]any{"username": freshPrefix(), "admin": false}
	stack.Call(t, http.MethodPost, "/api/v1/directory/members", fixture(t, stack, "ANA").AccessToken, request).ExpectError(t, http.StatusBadRequest, "invalid_request")
}

func TestAcId12AddingTheSameMemberTwiceIsADuplicate(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	addMemberReply(t, stack, ana.AccessToken, "bo", false).ExpectError(t, http.StatusConflict, "duplicate")
}

func TestAcId12OnlyAnAdminOfTheDomainAddsMembers(t *testing.T) {
	stack := NewStack(t)
	addMemberReply(t, stack, fixture(t, stack, "BO").AccessToken, freshPrefix(), false).Expect(t, http.StatusForbidden)
}

func TestAcId12DomainKeepsOneAdmin(t *testing.T) {
	stack := NewStack(t)
	operator := operatorToken(t, stack)
	domainId := createDomain(t, stack, operator)
	username := freshPrefix()
	firstAdmin := added(t, operatorAddReply(t, stack, operator, domainId, username, true))
	admin := signIn(t, stack, username, firstPassword(username))
	demote := map[string]bool{"admin": false}
	stack.Call(t, http.MethodPatch, "/api/v1/directory/members/"+firstAdmin.Member.MemberId, admin.AccessToken, demote).ExpectError(t, http.StatusConflict, "last_admin")
	removeMemberReply(t, stack, admin.AccessToken, firstAdmin.Member.MemberId).ExpectError(t, http.StatusConflict, "last_admin")
}

func TestAcId12AdminCanMakeAMemberAnAdmin(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	var changed memberSummary
	promote := map[string]bool{"admin": true}
	stack.Call(t, http.MethodPatch, "/api/v1/directory/members/"+dan.Id, ana.AccessToken, promote).Expect(t, http.StatusOK).Decode(t, &changed)
	danSession := signIn(t, stack, dan.Username, dan.Password)
	if !changed.Admin || !danSession.Domains[0].Admin {
		t.Fatalf("want dan to be an admin of example.com, got %+v and %+v", changed, danSession.Domains)
	}
}

func TestAcId10PrefixOfAPersonCannotBeAnAlias(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	createAliasReply(t, stack, ana.AccessToken, "eve@example.com", ana.Me.MemberId).ExpectError(t, http.StatusConflict, "duplicate")
}

func TestAcId10AliasLocalPartCannotBecomeAPrefix(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	localPart := freshPrefix()
	createAliasReply(t, stack, ana.AccessToken, localPart+"@example.com", ana.Me.MemberId).Expect(t, http.StatusCreated)
	operator := operatorToken(t, stack)
	operatorAddReply(t, stack, operator, "example.org", localPart, false).ExpectError(t, http.StatusConflict, "duplicate")
}

func TestAcId10PrefixOfAPersonCannotBeAChannelAddress(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	channelId := createPublicChannel(t, stack, ana.AccessToken, "prefix-"+freshPrefix())
	request := map[string]string{"address": "eve@example.com", "channelId": channelId}
	stack.Call(t, http.MethodPost, "/api/v1/directory/channel-addresses", ana.AccessToken, request).ExpectError(t, http.StatusConflict, "duplicate")
}

func TestAcId11PrefixIsStoredInLowercase(t *testing.T) {
	stack := NewStack(t)
	username := freshPrefix()
	result := added(t, addMemberReply(t, stack, fixture(t, stack, "ANA").AccessToken, strings.ToUpper(username), false))
	if result.Member.Username != username || result.Member.PrimaryAddress != username+"@example.com" {
		t.Fatalf("want %s in lowercase, got %+v", username, result.Member)
	}
}

func TestAcId11PrefixRulesAreChecked(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	expectInvalidUsername(t, stack, ana.AccessToken, "a..b"+freshPrefix())
	expectInvalidUsername(t, stack, ana.AccessToken, "-"+freshPrefix())
	expectInvalidUsername(t, stack, ana.AccessToken, freshPrefix()+".")
	expectInvalidUsername(t, stack, ana.AccessToken, freshPrefix()+"+news")
	expectInvalidUsername(t, stack, ana.AccessToken, freshPrefix()+"á")
	expectInvalidUsername(t, stack, ana.AccessToken, strings.Repeat("a", 65))
	expectInvalidUsername(t, stack, ana.AccessToken, "postmaster")
	expectInvalidUsername(t, stack, ana.AccessToken, "mailer-daemon")
}

func TestAcId11HostedAddressIgnoresLetterCase(t *testing.T) {
	stack := NewStack(t)
	var response struct {
		Recipients []resolvedRecipient `json:"recipients"`
	}
	body := map[string]any{"addresses": []string{"ANA@Example.COM"}}
	stack.Call(t, http.MethodPost, "/api/v1/directory/recipients/resolve", fixture(t, stack, "BO").AccessToken, body).Expect(t, http.StatusOK).Decode(t, &response)
	if len(response.Recipients) != 1 || response.Recipients[0].Kind != "member" {
		t.Fatalf("want ANA@Example.COM to be the member ana, got %+v", response.Recipients)
	}
}

func TestAcId11AliasWithAPlusIsRefused(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	createAliasReply(t, stack, ana.AccessToken, freshPrefix()+"+news@example.com", ana.Me.MemberId).Expect(t, http.StatusBadRequest)
}
