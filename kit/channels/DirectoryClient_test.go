package channels

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorded struct {
	method        string
	path          string
	authorization string
	body          string
}

func answering(t *testing.T, status int, seen *recorded) DirectoryClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*seen = recorded{method: request.Method, path: request.URL.EscapedPath(), authorization: request.Header.Get("Authorization"), body: string(body)}
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return NewDirectoryClient(server.URL+"/", server.Client())
}

func TestMemberInDomainForwardsTheCallersToken(t *testing.T) {
	seen := recorded{}
	found, err := answering(t, http.StatusOK, &seen).MemberInDomain("Bearer ana", "bo id")
	if err != nil || !found {
		t.Fatalf("want found, got %v %v", found, err)
	}
	if seen.method != http.MethodGet || seen.path != "/api/v1/directory/members/bo%20id" || seen.authorization != "Bearer ana" {
		t.Fatalf("unexpected request %+v", seen)
	}
}

func TestMemberInDomainIsFalseOnNotFound(t *testing.T) {
	found, err := answering(t, http.StatusNotFound, &recorded{}).MemberInDomain("Bearer ana", "eve")
	if err != nil || found {
		t.Fatalf("want not found, got %v %v", found, err)
	}
}

func TestMemberInDomainFailsOnAnUnexpectedStatus(t *testing.T) {
	_, err := answering(t, http.StatusInternalServerError, &recorded{}).MemberInDomain("Bearer ana", "eve")
	var failed DirectoryFailed
	if !errors.As(err, &failed) || failed.Status != http.StatusInternalServerError {
		t.Fatalf("want DirectoryFailed, got %v", err)
	}
}

func TestMemberInDomainFailsWhenTheDirectoryIsDown(t *testing.T) {
	_, err := NewDirectoryClient("http://127.0.0.1:1", http.DefaultClient).MemberInDomain("Bearer ana", "eve")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestClaimChannelAddressPostsTheChannel(t *testing.T) {
	seen := recorded{}
	err := answering(t, http.StatusCreated, &seen).ClaimChannelAddress("Bearer ana", "releases@example.com", "ch_1")
	if err != nil {
		t.Fatal(err)
	}
	if seen.method != http.MethodPost || seen.path != "/api/v1/directory/channel-addresses" || !strings.Contains(seen.body, `"channelId":"ch_1"`) {
		t.Fatalf("unexpected request %+v", seen)
	}
}

func TestClaimChannelAddressReportsATakenAddress(t *testing.T) {
	err := answering(t, http.StatusConflict, &recorded{}).ClaimChannelAddress("Bearer ana", "bo@example.com", "ch_1")
	if !errors.As(err, &AddressTaken{}) {
		t.Fatalf("want AddressTaken, got %v", err)
	}
	if !strings.Contains(err.Error(), "bo@example.com") {
		t.Fatalf("message misses the address: %v", err)
	}
}

func TestClaimChannelAddressReportsARefusedAddress(t *testing.T) {
	err := answering(t, http.StatusBadRequest, &recorded{}).ClaimChannelAddress("Bearer ana", "x@example.org", "ch_1")
	if !errors.As(err, &AddressRefused{}) || err.Error() == "" {
		t.Fatalf("want AddressRefused, got %v", err)
	}
}

func TestClaimChannelAddressFailsOnAnUnexpectedStatus(t *testing.T) {
	err := answering(t, http.StatusBadGateway, &recorded{}).ClaimChannelAddress("Bearer ana", "x@example.com", "ch_1")
	if !errors.As(err, &DirectoryFailed{}) || !strings.Contains(err.Error(), "502") {
		t.Fatalf("want DirectoryFailed, got %v", err)
	}
}

func TestClaimChannelAddressFailsWhenTheDirectoryIsDown(t *testing.T) {
	if NewDirectoryClient("http://127.0.0.1:1", http.DefaultClient).ClaimChannelAddress("Bearer ana", "x@example.com", "ch_1") == nil {
		t.Fatal("want an error")
	}
}

func TestReleaseChannelAddressDeletesTheAddress(t *testing.T) {
	seen := recorded{}
	if err := answering(t, http.StatusNoContent, &seen).ReleaseChannelAddress("Bearer ana", "releases@example.com"); err != nil {
		t.Fatal(err)
	}
	if seen.method != http.MethodDelete || seen.path != "/api/v1/directory/channel-addresses/releases@example.com" {
		t.Fatalf("unexpected request %+v", seen)
	}
}

func TestReleaseChannelAddressAcceptsAnAddressThatIsGone(t *testing.T) {
	if err := answering(t, http.StatusNotFound, &recorded{}).ReleaseChannelAddress("Bearer ana", "x@example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseChannelAddressFailsOnAnUnexpectedStatus(t *testing.T) {
	err := answering(t, http.StatusForbidden, &recorded{}).ReleaseChannelAddress("Bearer ana", "x@example.com")
	if !errors.As(err, &DirectoryFailed{}) {
		t.Fatalf("want DirectoryFailed, got %v", err)
	}
}

func TestReleaseChannelAddressFailsWhenTheDirectoryIsDown(t *testing.T) {
	if NewDirectoryClient("http://127.0.0.1:1", http.DefaultClient).ReleaseChannelAddress("Bearer ana", "x@example.com") == nil {
		t.Fatal("want an error")
	}
}

func TestExchangeRefusesABadAddress(t *testing.T) {
	if _, err := NewDirectoryClient("http://bad host", http.DefaultClient).MemberInDomain("Bearer ana", "bo"); err == nil {
		t.Fatal("want an error")
	}
}
