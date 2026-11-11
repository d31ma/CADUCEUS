package channels

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type keycloak struct {
	realms []string
	status int
}

func (fake *keycloak) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		parts := strings.Split(request.URL.Path, "/")
		fake.realms = append(fake.realms, parts[2])
		writer.WriteHeader(fake.status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "token-" + parts[2], "expires_in": 60})
	}))
	t.Cleanup(server.Close)
	return server
}

func tokens(t *testing.T, keycloakUrl string) oidc.ServiceTokens {
	t.Helper()
	return oidc.ServiceTokens{
		Settings: oidc.Settings{Keycloak: keycloakUrl + "/realms/{realm}/protocol/openid-connect"},
		Secret:   "secret",
		Client:   &http.Client{},
		Cache:    oidc.JsonFileCache{Folder: t.TempDir()},
		Now:      time.Now,
	}
}

func receiver(t *testing.T, status int, seen *string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*seen = request.Method + " " + request.URL.Path + " " + request.Header.Get("Authorization") + " " + string(body)
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestRealmPusherUsesATokenOfTheEventsDomain(t *testing.T) {
	fake := &keycloak{status: http.StatusOK}
	seen := ""
	url := receiver(t, http.StatusOK, &seen)
	pusher := RealmPusher{Tokens: tokens(t, fake.serve(t).URL), Timeout: time.Second}
	err := pusher.Push(outbox.Subscriber{Name: "search", URL: url + "/events"}, events.Envelope{EventID: "evt_1", DomainId: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.realms) != 1 || fake.realms[0] != "example.com" {
		t.Fatalf("want a token of realm example.com, asked %v", fake.realms)
	}
	if !strings.HasPrefix(seen, "POST /events Bearer token-example.com ") {
		t.Fatalf("unexpected push %q", seen)
	}
}

func TestRealmPusherFailsWithoutAToken(t *testing.T) {
	fake := &keycloak{status: http.StatusUnauthorized}
	pusher := RealmPusher{Tokens: tokens(t, fake.serve(t).URL), Timeout: time.Second}
	if pusher.Push(outbox.Subscriber{Name: "search", URL: "http://127.0.0.1:1/events"}, events.Envelope{DomainId: "example.com"}) == nil {
		t.Fatal("want an error")
	}
}

func TestScheduledSweepPostsToTheRelayRouteWithAnInstallToken(t *testing.T) {
	fake := &keycloak{status: http.StatusOK}
	seen := ""
	url := receiver(t, http.StatusOK, &seen)
	sweep := ScheduledSweep{Tokens: tokens(t, fake.serve(t).URL), Client: &http.Client{}}
	err := sweep.RunWith(`{"yon":{"channels":{"relayurl":"` + url + `/api/v1/channels/outbox/relay"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if fake.realms[0] != "mesail-operators" || seen != "POST /api/v1/channels/outbox/relay Bearer token-mesail-operators {}" {
		t.Fatalf("unexpected sweep %v %q", fake.realms, seen)
	}
}

func TestScheduledSweepReportsARefusal(t *testing.T) {
	fake := &keycloak{status: http.StatusOK}
	seen := ""
	url := receiver(t, http.StatusForbidden, &seen)
	err := ScheduledSweep{Tokens: tokens(t, fake.serve(t).URL), Client: &http.Client{}}.RunWith(`{"yon":{"channels":{"relayurl":"` + url + `/relay"}}}`)
	if !errors.As(err, &SweepRefused{}) || err.Error() != "the outbox relay route answered 403" {
		t.Fatalf("want SweepRefused, got %v", err)
	}
}

func TestScheduledSweepFailsWithoutSettings(t *testing.T) {
	if NewScheduledSweep().Run() == nil {
		t.Fatal("want an error without oidc settings")
	}
}

func TestScheduledSweepFailsOnABadUrlOrADownService(t *testing.T) {
	fake := &keycloak{status: http.StatusOK}
	sweep := ScheduledSweep{Tokens: tokens(t, fake.serve(t).URL), Client: &http.Client{}}
	if sweep.RunWith(`{"yon":{"channels":{"relayurl":"http://bad host/relay"}}}`) == nil {
		t.Fatal("want an error for a bad url")
	}
	if sweep.RunWith(`{"yon":{"channels":{"relayurl":"http://127.0.0.1:1/relay"}}}`) == nil {
		t.Fatal("want an error for a down service")
	}
}
