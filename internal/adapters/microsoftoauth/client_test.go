package microsoftoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientBuildsConfidentialPKCEAuthorizationAndExchangesCode(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.Body == nil {
			t.Fatalf("token request = %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		form = request.PostForm
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"access_token": "access-token", "refresh_token": "refresh-token", "expires_in": 3600, "token_type": "Bearer",
		})
	}))
	defer server.Close()

	client, err := New(Config{
		ClientID: "client-id", ClientSecret: "client-secret-value", Tenant: "organizations",
		RedirectURI: "https://mail.example.com/settings", AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token",
		HTTPClient: server.Client(), Clock: func() time.Time { return time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	authorization, err := client.AuthorizationURL("state-value", "challenge-value")
	if err != nil {
		t.Fatalf("AuthorizationURL() error = %v", err)
	}
	parsed, _ := url.Parse(authorization)
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("response_mode") != "query" || query.Get("code_challenge_method") != "S256" || query.Get("state") != "state-value" {
		t.Fatalf("authorization query = %#v", query)
	}
	for _, scope := range []string{"openid", "offline_access", "https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com/SMTP.Send", "Calendars.ReadWrite", "Calendars.ReadWrite.Shared", "Contacts.ReadWrite", "Contacts.ReadWrite.Shared"} {
		if !strings.Contains(query.Get("scope"), scope) {
			t.Fatalf("authorization scope missing %q: %q", scope, query.Get("scope"))
		}
	}

	grant, err := client.Exchange(context.Background(), "authorization-code", "pkce-verifier")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if grant.AccessToken != "access-token" || grant.RefreshToken != "refresh-token" || !grant.ExpiresAt.Equal(time.Date(2026, 8, 22, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("grant = %#v", grant)
	}
	if form.Get("client_secret") != "client-secret-value" || form.Get("code_verifier") != "pkce-verifier" || form.Get("redirect_uri") != "https://mail.example.com/settings" {
		t.Fatalf("token form = %#v", form)
	}
}

func TestClientRefreshRotatesRefreshTokenAndRedactsProviderErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if requests == 1 {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh", "expires_in": 1800, "token_type": "Bearer"})
			return
		}
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"error_description":"leaked-provider-detail"}`))
	}))
	defer server.Close()
	client, err := New(Config{ClientID: "client-id", ClientSecret: "client-secret-value", Tenant: "organizations", RedirectURI: "https://mail.example.com/settings", AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil || grant.AccessToken != "new-access" || grant.RefreshToken != "new-refresh" {
		t.Fatalf("Refresh() = %#v, %v", grant, err)
	}
	if _, err := client.Refresh(context.Background(), "old-refresh"); err == nil || strings.Contains(err.Error(), "leaked-provider-detail") {
		t.Fatalf("provider error was not redacted: %v", err)
	}
}
