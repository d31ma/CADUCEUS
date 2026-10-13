package googleoauth

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

func TestClientUsesOfflineMailCalendarAndContactScopesAndRotatesTokens(t *testing.T) {
	var form url.Values
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		form = request.PostForm
		_ = json.NewEncoder(response).Encode(map[string]any{"access_token": "access", "refresh_token": "refresh", "token_type": "Bearer", "expires_in": 3600})
	}))
	defer server.Close()
	now := time.Date(2026, 8, 22, 16, 0, 0, 0, time.UTC)
	client, err := New(Config{ClientID: "client-id", ClientSecret: "client-secret-value", RedirectURI: "https://mail.example.com/settings", AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := client.AuthorizationURL("state", "challenge")
	if err != nil {
		t.Fatal(err)
	}
	query := mustURL(t, authorization).Query()
	for _, scope := range []string{gmailScope, calendarEventsScope, calendarListScope, contactsScope} {
		if !strings.Contains(query.Get("scope"), scope) {
			t.Fatalf("authorization scope missing %q: %q", scope, query.Get("scope"))
		}
	}
	if query.Get("access_type") != "offline" || query.Get("prompt") != "consent" || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization query = %#v", query)
	}
	grant, err := client.Exchange(context.Background(), "code", "verifier")
	if err != nil || grant.AccessToken != "access" || grant.RefreshToken != "refresh" || form.Get("code_verifier") != "verifier" || !grant.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("grant=%#v form=%#v error=%v", grant, form, err)
	}
}

func TestClientRedactsGoogleTokenErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"error_description":"secret provider detail"}`))
	}))
	defer server.Close()
	client, err := New(Config{ClientID: "client-id", ClientSecret: "client-secret-value", RedirectURI: "https://mail.example.com/settings", AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Refresh(context.Background(), "refresh"); err == nil || strings.Contains(err.Error(), "provider detail") {
		t.Fatalf("Refresh() error = %v", err)
	}
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
