//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
	"time"
)

func membersMatching(t *testing.T, stack Stack, adminToken string, username string) int {
	t.Helper()
	var page struct {
		Items []memberSummary `json:"items"`
	}
	stack.Call(t, http.MethodGet, "/api/v1/directory/members?query="+username, adminToken, nil).Expect(t, http.StatusOK).Decode(t, &page)
	return len(page.Items)
}

func TestD2RemovingADomainAnswers202AndFinishesLater(t *testing.T) {
	stack := NewStack(t)
	operator := operatorToken(t, stack)
	domainId := createDomain(t, stack, operator)
	added(t, operatorAddReply(t, stack, operator, domainId, freshPrefix(), true))
	var removing struct {
		DomainId string `json:"domainId"`
		State    string `json:"state"`
	}
	stack.Call(t, http.MethodDelete, "/api/v1/directory/domains/"+domainId, operator, nil).Expect(t, http.StatusAccepted).Decode(t, &removing)
	if removing.DomainId != domainId || removing.State != "removing" {
		t.Fatalf("want %s removing, got %+v", domainId, removing)
	}
	stack.Eventually(t, 150*time.Second, "the removed domain is gone", func() bool {
		return stack.Call(t, http.MethodGet, "/api/v1/directory/domains/"+domainId, operator, nil).Status == http.StatusNotFound
	})
}

func TestD2DeletingAPersonAnswers202AndRemovesTheMembershipLater(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := createPerson(t, stack, ana.AccessToken)
	stack.Call(t, http.MethodDelete, "/api/v1/directory/people/"+dan.Id, operatorToken(t, stack), nil).Expect(t, http.StatusAccepted)
	signInReply(t, stack, dan.Username, dan.Password).Expect(t, http.StatusUnauthorized)
	stack.Eventually(t, 150*time.Second, "the deleted person leaves the member list", func() bool {
		return membersMatching(t, stack, ana.AccessToken, dan.Username) == 0
	})
}
