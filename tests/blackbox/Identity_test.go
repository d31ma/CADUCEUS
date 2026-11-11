//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

type member struct {
	MemberId       string   `json:"memberId"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"displayName"`
	PrimaryAddress string   `json:"primaryAddress"`
	DomainId       string   `json:"domainId"`
	Roles          []string `json:"roles"`
}

func me(t *testing.T, stack Stack, token string) member {
	t.Helper()
	var signedIn member
	stack.Call(t, http.MethodGet, "/api/v1/identity/me", token, nil).Expect(t, http.StatusOK).Decode(t, &signedIn)
	return signedIn
}

func TestAcId1RoutesRefuseAMissingToken(t *testing.T) {
	stack := NewStack(t)
	stack.Call(t, http.MethodGet, "/api/v1/identity/me", "", nil).Expect(t, http.StatusUnauthorized)
	stack.Call(t, http.MethodGet, "/api/v1/messaging/conversations", "", nil).Expect(t, http.StatusUnauthorized)
}

func TestAcId1RoutesRefuseAForgedToken(t *testing.T) {
	stack := NewStack(t)
	forged := "eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ."
	stack.Call(t, http.MethodGet, "/api/v1/identity/me", forged, nil).Expect(t, http.StatusUnauthorized)
}

func TestMeDescribesTheSignedInMember(t *testing.T) {
	stack := NewStack(t)
	ana := me(t, stack, stack.Token(t, "ANA"))
	if ana.MemberId == "" || ana.Username != "ana" || ana.PrimaryAddress != "ana@example.com" {
		t.Fatalf("got %+v", ana)
	}
}

func TestMeNamesTheDomain(t *testing.T) {
	stack := NewStack(t)
	ana := me(t, stack, stack.Token(t, "ANA"))
	if ana.DomainId != "example.com" {
		t.Fatalf("want domainId example.com, got %+v", ana)
	}
}

func TestAcId3NeitherAMemberNorADomainAdminCanAddADomain(t *testing.T) {
	stack := NewStack(t)
	request := map[string]string{"name": "evil.example"}
	stack.Call(t, http.MethodPost, "/api/v1/directory/domains", stack.Token(t, "BO"), request).Expect(t, http.StatusForbidden)
	stack.Call(t, http.MethodPost, "/api/v1/directory/domains", stack.Token(t, "ANA"), request).Expect(t, http.StatusForbidden)
}
