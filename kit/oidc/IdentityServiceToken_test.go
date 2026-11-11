package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type identityTokens struct {
	server  *httptest.Server
	fetches *atomic.Int64
}

func newIdentityTokens(t *testing.T) identityTokens {
	t.Helper()
	fetches := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked := ServiceTokenRequest{}
		_ = json.NewDecoder(request.Body).Decode(&asked)
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/identity/service-token" || request.Header.Get("Content-Type") != "application/json" || asked.ClientId != "mesail-service" || asked.ClientSecret != "service-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		count := fetches.Add(1)
		_ = json.NewEncoder(writer).Encode(ServiceTokenAnswer{AccessToken: asked.DomainId + "-token-" + strconv.FormatInt(count, 10), ExpiresIn: 60})
	}))
	t.Cleanup(server.Close)
	return identityTokens{server: server, fetches: fetches}
}

func identityTokensDocument(endpoint string, secret string) string {
	return `{"yon":{"oidc":{"issuer":"https://mail.example.net/api/v1/identity","jwks":"https://mail.example.net/api/v1/identity/jwks","audience":"mesail-api","serviceTokens":"` +
		endpoint + `"},"service":{"client":{"secret":"` + secret + `"}}}}`
}

func (identity identityTokens) tokens(t *testing.T, current *clock, secret string) ServiceTokens {
	t.Helper()
	tokens := ServiceTokensFactory{}.FromJson(identityTokensDocument(identity.server.URL+"/api/v1/identity/service-token", secret))
	if tokens.Problem != nil {
		t.Fatal(tokens.Problem)
	}
	tokens.Client = identity.server.Client()
	tokens.Cache = JsonFileCache{Folder: t.TempDir()}
	tokens.Now = current.Now
	return tokens
}

func TestAuthorizationAsksIdentityForADomainToken(t *testing.T) {
	identity := newIdentityTokens(t)
	header, err := identity.tokens(t, &clock{now: testNow()}, "service-secret").Authorization("example.com")
	if err != nil || header != "Bearer example.com-token-1" {
		t.Fatalf("got %q, %v", header, err)
	}
}

func TestAuthorizationAsksIdentityForAnInstallWideToken(t *testing.T) {
	identity := newIdentityTokens(t)
	header, err := identity.tokens(t, &clock{now: testNow()}, "service-secret").Authorization(InstallWide{}.DomainId())
	if err != nil || header != "Bearer *-token-1" {
		t.Fatalf("got %q, %v", header, err)
	}
}

func TestAuthorizationCachesEachDomainApart(t *testing.T) {
	identity := newIdentityTokens(t)
	tokens := identity.tokens(t, &clock{now: testNow()}, "service-secret")
	first, _ := tokens.Authorization("example.com")
	other, _ := tokens.Authorization("example.org")
	again, err := tokens.Authorization("example.com")
	if err != nil || first != again || other != "Bearer example.org-token-2" || identity.fetches.Load() != 2 {
		t.Fatalf("got %q %q %q, %d fetches, %v", first, other, again, identity.fetches.Load(), err)
	}
}

func TestAuthorizationAsksAgainTenSecondsBeforeExpiry(t *testing.T) {
	identity := newIdentityTokens(t)
	current := &clock{now: testNow()}
	tokens := identity.tokens(t, current, "service-secret")
	_, _ = tokens.Authorization("example.com")
	current.now = testNow().Add(51 * time.Second)
	header, err := tokens.Authorization("example.com")
	if err != nil || header != "Bearer example.com-token-2" {
		t.Fatalf("got %q, %v", header, err)
	}
}

func TestAuthorizationReportsARefusedServiceClient(t *testing.T) {
	identity := newIdentityTokens(t)
	_, err := identity.tokens(t, &clock{now: testNow()}, "wrong-secret").Authorization("example.com")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorizationReportsIdentityDown(t *testing.T) {
	identity := newIdentityTokens(t)
	tokens := identity.tokens(t, &clock{now: testNow()}, "service-secret")
	identity.server.Close()
	_, err := tokens.Authorization("example.com")
	unavailable := KeycloakUnavailable{}
	if !errors.As(err, &unavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorizationReportsAServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	tokens := ServiceTokensFactory{}.FromJson(identityTokensDocument(server.URL, "service-secret"))
	tokens.Client = server.Client()
	tokens.Cache = JsonFileCache{Folder: t.TempDir()}
	_, err := tokens.Authorization("example.com")
	unavailable := KeycloakUnavailable{}
	if !errors.As(err, &unavailable) || unavailable.Status != http.StatusServiceUnavailable {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorizationReportsAnAnswerThatIsNotJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("not json"))
	}))
	t.Cleanup(server.Close)
	tokens := ServiceTokensFactory{}.FromJson(identityTokensDocument(server.URL, "service-secret"))
	tokens.Client = server.Client()
	tokens.Cache = JsonFileCache{Folder: t.TempDir()}
	_, err := tokens.Authorization("example.com")
	if err == nil {
		t.Fatal("accepted an answer that is not JSON")
	}
}

func TestAuthorizationReportsABodyThatBreaksOff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", "100")
		_, _ = writer.Write([]byte("{"))
	}))
	t.Cleanup(server.Close)
	tokens := ServiceTokensFactory{}.FromJson(identityTokensDocument(server.URL, "service-secret"))
	tokens.Client = server.Client()
	tokens.Cache = JsonFileCache{Folder: t.TempDir()}
	_, err := tokens.Authorization("example.com")
	unavailable := KeycloakUnavailable{}
	if !errors.As(err, &unavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestServiceTokensNeedTheSecretInTheMesailMode(t *testing.T) {
	tokens := ServiceTokensFactory{}.FromJson(identityTokensDocument("https://x/service-token", ""))
	if !errors.Is(tokens.Problem, NotConfigured{}) {
		t.Fatalf("got %v", tokens.Problem)
	}
}

func TestIdentityServiceTokenKeysTheCacheByDomain(t *testing.T) {
	token := IdentityServiceToken{Endpoint: "https://x/service-token", DomainId: "example.com"}
	if token.CacheKey() != "https://x/service-token#example.com" {
		t.Fatalf("got %q", token.CacheKey())
	}
}
