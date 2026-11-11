package mailin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func directoryServer(t *testing.T, status int, body string) (*httptest.Server, *[]string) {
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = append(seen, request.URL.Path+" "+request.Header.Get("Authorization"))
		writer.WriteHeader(status)
		writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func realmToken(realm string) (string, error) {
	return "Bearer token-for-" + realm, nil
}

func TestTheDirectoryReturnsAnActiveDomain(t *testing.T) {
	body, _ := json.Marshal(Domain{DomainId: "sender.test", Name: "sender.test", State: "active"})
	server, seen := directoryServer(t, http.StatusOK, string(body))
	domain, found, err := Directory{Client: server.Client(), BaseUrl: server.URL + "/", Authorize: realmToken}.Domain("Sender.Test")
	check(t, err == nil && found && domain.IsActive(), "want an active domain")
	check(t, (*seen)[0] == "/api/v1/directory/domains/sender.test Bearer token-for-mesail-operators", "wrong request")
}

func TestTheDirectoryReportsAnUnknownDomain(t *testing.T) {
	server, _ := directoryServer(t, http.StatusNotFound, `{"code":"not_found"}`)
	_, found, err := Directory{Client: server.Client(), BaseUrl: server.URL, Authorize: realmToken}.Domain("sender.test")
	check(t, err == nil && !found, "want not found")
}

func TestTheDirectoryFailsOnAnotherStatus(t *testing.T) {
	server, _ := directoryServer(t, http.StatusBadGateway, "")
	_, _, err := Directory{Client: server.Client(), BaseUrl: server.URL, Authorize: realmToken}.Domain("sender.test")
	check(t, err != nil, "want a failure")
}

func TestTheDirectoryFailsOnABadBody(t *testing.T) {
	server, _ := directoryServer(t, http.StatusOK, "{")
	_, found, err := Directory{Client: server.Client(), BaseUrl: server.URL, Authorize: realmToken}.Domain("sender.test")
	check(t, err != nil && !found, "want a failure")
}

func TestTheDirectoryFailsWithoutAToken(t *testing.T) {
	server, seen := directoryServer(t, http.StatusOK, "{}")
	refuse := func(realm string) (string, error) { return "", errors.New("no token") }
	_, _, err := Directory{Client: server.Client(), BaseUrl: server.URL, Authorize: refuse}.Domain("sender.test")
	check(t, err != nil && len(*seen) == 0, "want a failure before any request")
}

func TestTheDirectoryFailsOnABadAddress(t *testing.T) {
	_, _, err := Directory{Client: http.DefaultClient, BaseUrl: "http://bad host", Authorize: realmToken}.Domain("sender.test")
	check(t, err != nil, "want a failure")
}

func TestTheDirectoryFailsWhenNothingAnswers(t *testing.T) {
	server, _ := directoryServer(t, http.StatusOK, "{}")
	server.Close()
	_, _, err := Directory{Client: http.DefaultClient, BaseUrl: server.URL, Authorize: realmToken}.Domain("sender.test")
	check(t, err != nil, "want a failure")
}

func TestDomainTokensAskTheRealmOfTheDomain(t *testing.T) {
	token, err := DomainTokens{Authorize: realmToken}.TokenFor("example.com")
	check(t, err == nil && token == "token-for-example.com", "wrong token")
}
