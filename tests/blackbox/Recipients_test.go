//go:build blackbox

package blackbox

import (
	"net/http"
	"slices"
	"testing"
)

type resolvedRecipient struct {
	Address           string   `json:"address"`
	Kind              string   `json:"kind"`
	DefaultDelivery   string   `json:"defaultDelivery"`
	AllowedDeliveries []string `json:"allowedDeliveries"`
}

func resolve(t *testing.T, stack Stack, token string, addresses ...string) map[string]resolvedRecipient {
	t.Helper()
	var response struct {
		Recipients []resolvedRecipient `json:"recipients"`
	}
	body := map[string]any{"addresses": addresses}
	stack.Call(t, http.MethodPost, "/api/v1/directory/recipients/resolve", token, body).Expect(t, http.StatusOK).Decode(t, &response)
	byAddress := map[string]resolvedRecipient{}
	for _, recipient := range response.Recipients {
		byAddress[recipient.Address] = recipient
	}
	return byAddress
}

func expectEmailOnly(t *testing.T, recipient resolvedRecipient, kind string) {
	t.Helper()
	if recipient.Kind != kind || recipient.DefaultDelivery != "email" || !slices.Equal(recipient.AllowedDeliveries, []string{"email"}) {
		t.Fatalf("want kind %s with email only, got %+v", kind, recipient)
	}
}

func TestAcCp1InternalMemberResolvesToChat(t *testing.T) {
	stack := NewStack(t)
	bo := resolve(t, stack, stack.Token(t, "ANA"), "bo@example.com")["bo@example.com"]
	if bo.Kind != "member" || bo.DefaultDelivery != "chat" {
		t.Fatalf("got %+v", bo)
	}
}

func TestAcCp1ExternalAddressResolvesToEmailOnly(t *testing.T) {
	stack := NewStack(t)
	carol := resolve(t, stack, stack.Token(t, "ANA"), "carol@partner.test")["carol@partner.test"]
	expectEmailOnly(t, carol, "external")
}

func TestAcDm5MemberOfAnotherHostedDomainResolvesToEmailOnly(t *testing.T) {
	stack := NewStack(t)
	eve := resolve(t, stack, stack.Token(t, "ANA"), "eve@example.org")["eve@example.org"]
	expectEmailOnly(t, eve, "otherDomain")
}

func TestAcId2MemberOfAnotherDomainIsOtherDomain(t *testing.T) {
	stack := NewStack(t)
	bo := resolve(t, stack, stack.Token(t, "EVE"), "bo@example.com")["bo@example.com"]
	expectEmailOnly(t, bo, "otherDomain")
}

func TestAddressesAreMatchedWithoutCase(t *testing.T) {
	stack := NewStack(t)
	bo := resolve(t, stack, stack.Token(t, "ANA"), "Bo@Example.COM")["bo@example.com"]
	if bo.Kind != "member" {
		t.Fatalf("got %+v", bo)
	}
}
