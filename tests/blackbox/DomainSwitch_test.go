//go:build blackbox

package blackbox

import (
	"net/http"
	"slices"
	"testing"
	"time"
)

func TestAcId6SwitchingDomainNeedsNoPassword(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	inOrg := switchDomain(t, stack, ana.SessionKey, "example.org")
	if inOrg.Me.DomainId != "example.org" || inOrg.Me.MemberId != ana.Me.MemberId {
		t.Fatalf("want the same person in example.org, got %+v", inOrg.Me)
	}
	if me(t, stack, inOrg.AccessToken).DomainId != "example.org" {
		t.Fatal("the new token does not name example.org")
	}
	back := switchDomain(t, stack, inOrg.SessionKey, "example.com")
	if back.Me.DomainId != "example.com" {
		t.Fatalf("want example.com again, got %+v", back.Me)
	}
}

func TestAcId6SwitchToADomainThePersonIsNotInIsNotFound(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	switchReply(t, stack, ana.SessionKey, "partner.test").Expect(t, http.StatusNotFound)
}

func TestAcId6EachSessionKeepsItsOwnDomain(t *testing.T) {
	stack := NewStack(t)
	firstTab := fixture(t, stack, "ANA")
	secondTab := switchDomain(t, stack, fixture(t, stack, "ANA").SessionKey, "example.org")
	var refreshed session
	refreshReply(t, stack, firstTab.SessionKey).Expect(t, http.StatusOK).Decode(t, &refreshed)
	if refreshed.Me.DomainId != "example.com" || secondTab.Me.DomainId != "example.org" {
		t.Fatalf("want one tab per domain, got %s and %s", refreshed.Me.DomainId, secondTab.Me.DomainId)
	}
}

func TestAcId3RolesFollowTheActiveDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	inOrg := switchDomain(t, stack, ana.SessionKey, "example.org")
	if !slices.Contains(ana.Me.Roles, "DomainAdmin") || slices.Contains(inOrg.Me.Roles, "DomainAdmin") {
		t.Fatalf("want DomainAdmin only in example.com, got %v and %v", ana.Me.Roles, inOrg.Me.Roles)
	}
	stack.Call(t, http.MethodPost, "/api/v1/directory/members", inOrg.AccessToken, addMemberRequest(freshPrefix(), false)).Expect(t, http.StatusForbidden)
}

func TestAcId2TokenOfOneDomainNeverReadsTheOtherDomainOfTheSamePerson(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	channelId := createPublicChannel(t, stack, ana.AccessToken, "one-login-"+freshPrefix())
	inOrg := switchDomain(t, stack, ana.SessionKey, "example.org")
	if slices.Contains(visibleChannelIds(t, stack, inOrg.AccessToken), channelId) {
		t.Fatal("an example.com channel is visible with the example.org token")
	}
	stack.Call(t, http.MethodPost, "/api/v1/channels/"+channelId+"/join", inOrg.AccessToken, nil).Expect(t, http.StatusNotFound)
}

func TestAcId4And12RemovedMemberLosesOnlyThatDomain(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	added(t, operatorAddReply(t, stack, operatorToken(t, stack), "example.org", dan.Username, false))
	inCom := signIn(t, stack, dan.Username+"@example.com", dan.Password)
	inOrg := signIn(t, stack, dan.Username+"@example.org", dan.Password)
	removeMemberReply(t, stack, ana.AccessToken, dan.Id).Expect(t, http.StatusNoContent)
	stack.Eventually(t, 70*time.Second, "the example.com session to lose its domain", func() bool {
		return refreshReply(t, stack, inCom.SessionKey).Status == http.StatusForbidden
	})
	refreshReply(t, stack, inCom.SessionKey).ExpectError(t, http.StatusForbidden, "not_a_member")
	var moved session
	switchReply(t, stack, inCom.SessionKey, "").Expect(t, http.StatusOK).Decode(t, &moved)
	if moved.Me.DomainId != "example.org" {
		t.Fatalf("want the same session to open example.org, got %+v", moved.Me)
	}
	refreshReply(t, stack, inOrg.SessionKey).Expect(t, http.StatusOK)
	switchReply(t, stack, inOrg.SessionKey, "example.com").Expect(t, http.StatusNotFound)
}

func TestAcId12AddedDomainAppearsWithinAMinute(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	danSession := signIn(t, stack, dan.Username, dan.Password)
	added(t, operatorAddReply(t, stack, operatorToken(t, stack), "example.org", dan.Username, false))
	stack.Eventually(t, 70*time.Second, "example.org in the domain list", func() bool {
		var refreshed session
		refreshReply(t, stack, danSession.SessionKey).Expect(t, http.StatusOK).Decode(t, &refreshed)
		return slices.Contains(domainIds(refreshed.Domains), "example.org")
	})
}

func TestAcId12RemovedAddressIsUnknownToSenders(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	removeMemberReply(t, stack, ana.AccessToken, dan.Id).Expect(t, http.StatusNoContent)
	expectRcptRefused(t, dan.Username+"@example.com")
}
