package oidc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type keycloakFake struct {
	server *httptest.Server
	forms  []url.Values
	paths  []string
}

func newKeycloakFake(t *testing.T, status int, body string) *keycloakFake {
	t.Helper()
	fake := &keycloakFake{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		fake.forms = append(fake.forms, request.PostForm)
		fake.paths = append(fake.paths, request.URL.Path)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *keycloakFake) client() KeycloakClient {
	return KeycloakClient{Client: fake.server.Client(), Endpoint: fake.server.URL + "/realms/example.com/protocol/openid-connect", ClientId: "mesail-identity", ClientSecret: "client-secret"}
}

func TestSignInPostsAPasswordGrant(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, `{"access_token":"a","refresh_token":"r","expires_in":60,"refresh_expires_in":1800}`)
	grant, err := fake.client().SignIn("ana@example.com", "pw", "123456")
	form := fake.forms[0]
	if err != nil || grant.AccessToken != "a" || grant.RefreshToken != "r" || grant.ExpiresIn != 60 {
		t.Fatalf("got %+v, %v", grant, err)
	}
	if fake.paths[0] != "/realms/example.com/protocol/openid-connect/token" || form.Get("grant_type") != "password" || form.Get("username") != "ana@example.com" || form.Get("totp") != "123456" || form.Get("client_secret") != "client-secret" {
		t.Fatalf("got %s %v", fake.paths[0], form)
	}
}

func TestSignInReportsWrongCredentials(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusUnauthorized, `{"error":"invalid_grant"}`)
	_, err := fake.client().SignIn("ana@example.com", "wrong", "")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInReportsABadRequestAsARefusal(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
	_, err := fake.client().SignIn("ana@example.com", "pw", "")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInReportsAKeycloakFailure(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusBadGateway, "")
	_, err := fake.client().SignIn("ana@example.com", "pw", "")
	var unavailable KeycloakUnavailable
	if !errors.As(err, &unavailable) || unavailable.Status != http.StatusBadGateway || unavailable.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestSignInReportsAnUnreachableKeycloak(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, "")
	client := fake.client()
	fake.server.Close()
	_, err := client.SignIn("ana@example.com", "pw", "")
	var unavailable KeycloakUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestRefreshPostsARefreshGrant(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, `{"access_token":"a2","refresh_token":"r2"}`)
	grant, err := fake.client().Refresh("r1")
	if err != nil || grant.RefreshToken != "r2" || fake.forms[0].Get("grant_type") != "refresh_token" || fake.forms[0].Get("refresh_token") != "r1" {
		t.Fatalf("got %+v, %v, %v", grant, err, fake.forms[0])
	}
}

func TestSignOutPostsToTheLogoutEndpoint(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusNoContent, "")
	err := fake.client().SignOut("r1")
	form := fake.forms[0]
	if err != nil || fake.paths[0] != "/realms/example.com/protocol/openid-connect/logout" || form.Get("refresh_token") != "r1" || form.Has("grant_type") {
		t.Fatalf("got %v, %s, %v", err, fake.paths[0], form)
	}
}

func TestSignInCannotTellALockedAccountFromAWrongPassword(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid user credentials"}`)
	_, err := fake.client().SignIn("ana@example.com", "pw", "")
	if !errors.Is(err, SignInRefused{}) || (AccountLocked{}).Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestSignInReportsAnUnknownRealmAsARefusal(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusNotFound, `{"error":"Realm does not exist"}`)
	_, err := fake.client().SignIn("ana@nowhere.example", "pw", "")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignOutReportsAKeycloakFailure(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusServiceUnavailable, "")
	var unavailable KeycloakUnavailable
	if err := fake.client().SignOut("r1"); !errors.As(err, &unavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestSignOutReportsAnUnreachableKeycloak(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, "")
	client := fake.client()
	fake.server.Close()
	if err := client.SignOut("r1"); err == nil {
		t.Fatal("expected a failure")
	}
}

func TestSignInRefusedExplainsItself(t *testing.T) {
	if (SignInRefused{}).Error() == "" {
		t.Fatal("empty message")
	}
}
