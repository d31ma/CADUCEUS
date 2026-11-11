package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type serviceKeycloak struct {
	server  *httptest.Server
	fetches *atomic.Int64
}

func newServiceKeycloak(t *testing.T) serviceKeycloak {
	t.Helper()
	fetches := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		form := request.PostForm
		if request.URL.Path != "/realms/example.com/protocol/openid-connect/token" || form.Get("grant_type") != "client_credentials" || form.Get("client_id") != "mesail-service" || form.Get("client_secret") != "service-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		count := fetches.Add(1)
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "service-token-" + string(rune('0'+count)), "expires_in": 60})
	}))
	t.Cleanup(server.Close)
	return serviceKeycloak{server: server, fetches: fetches}
}

func (keycloak serviceKeycloak) tokens(t *testing.T, current *clock) ServiceTokens {
	t.Helper()
	document := `{"yon":{"oidc":{"issuer":"https://x/realms/{realm}","jwks":"https://x/realms/{realm}/certs","audience":"mesail-api","keycloak":"` +
		keycloak.server.URL + `/realms/{realm}/protocol/openid-connect"},"service":{"client":{"secret":"service-secret"}}}}`
	tokens := ServiceTokensFactory{}.FromJson(document)
	if tokens.Problem != nil {
		t.Fatal(tokens.Problem)
	}
	tokens.Client = keycloak.server.Client()
	tokens.Cache = JsonFileCache{Folder: t.TempDir()}
	tokens.Now = current.Now
	return tokens
}

func TestAuthorizationAsksKeycloakForAServiceToken(t *testing.T) {
	keycloak := newServiceKeycloak(t)
	header, err := keycloak.tokens(t, &clock{now: testNow()}).Authorization("example.com")
	if err != nil || header != "Bearer service-token-1" {
		t.Fatalf("got %q, %v", header, err)
	}
}

func TestAuthorizationReusesTheCachedToken(t *testing.T) {
	keycloak := newServiceKeycloak(t)
	current := &clock{now: testNow()}
	tokens := keycloak.tokens(t, current)
	_, _ = tokens.Authorization("example.com")
	current.now = current.now.Add(49 * time.Second)
	header, err := tokens.Authorization("example.com")
	if err != nil || header != "Bearer service-token-1" || keycloak.fetches.Load() != 1 {
		t.Fatalf("got %q, %v after %d fetches", header, err, keycloak.fetches.Load())
	}
}

func TestAuthorizationFetchesANewTokenNearExpiry(t *testing.T) {
	keycloak := newServiceKeycloak(t)
	current := &clock{now: testNow()}
	tokens := keycloak.tokens(t, current)
	_, _ = tokens.Authorization("example.com")
	current.now = current.now.Add(51 * time.Second)
	header, err := tokens.Authorization("example.com")
	if err != nil || header != "Bearer service-token-2" {
		t.Fatalf("got %q, %v", header, err)
	}
}

func TestAuthorizationReportsAKeycloakRefusal(t *testing.T) {
	keycloak := newServiceKeycloak(t)
	_, err := keycloak.tokens(t, &clock{now: testNow()}).Authorization("example.org")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorizationRefusesAnInvalidRealm(t *testing.T) {
	keycloak := newServiceKeycloak(t)
	_, err := keycloak.tokens(t, &clock{now: testNow()}).Authorization("../master")
	refusedBy(t, err)
}

func TestAuthorizationNeedsItsSettings(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := ServiceTokensFactory{}.Configured().Authorization("example.com")
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestJsonFileCacheReportsAValueItCannotWrite(t *testing.T) {
	if err := (JsonFileCache{Folder: t.TempDir()}).Store("k", make(chan int)); err == nil {
		t.Fatal("expected a failure")
	}
}
