package oidc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type keycloakRealm struct {
	server   *httptest.Server
	signOuts *atomic.Int64
}

func newKeycloakRealm(t *testing.T) keycloakRealm {
	t.Helper()
	signer := newTestSigner(t)
	signOuts := &atomic.Int64{}
	locked := &atomic.Bool{}
	claims := claimsWith("exp", time.Now().Unix()+60)
	answer := func(writer http.ResponseWriter, refresh string) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": signer.rsaToken(t, "rsa-1", claims), "refresh_token": refresh, "expires_in": 60})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/example.com/certs", func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(signer.keySet())
	})
	mux.HandleFunc("POST /realms/example.com/protocol/openid-connect/token", func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		form := request.PostForm
		switch {
		case form.Get("client_id") == "mesail-account" && form.Get("client_secret") == "account-secret":
			_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "admin-token", "expires_in": 60})
		case form.Get("client_id") != "mesail-identity" || form.Get("client_secret") != "client-secret":
			writer.WriteHeader(http.StatusUnauthorized)
		case form.Get("grant_type") == "password" && form.Get("password") == "right" && form.Get("totp") == "123456":
			answer(writer, "refresh-1")
		case form.Get("grant_type") == "password" && form.Get("password") == "locked":
			locked.Store(true)
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
		case form.Get("grant_type") == "refresh_token" && form.Get("refresh_token") == "refresh-1":
			answer(writer, "refresh-2")
		default:
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
		}
	})
	mux.HandleFunc("GET /admin/realms/example.com/users", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer admin-token" || request.URL.Query().Get("username") != "ana@example.com" || request.URL.Query().Get("exact") != "true" {
			_, _ = writer.Write([]byte(`[]`))
			return
		}
		_, _ = writer.Write([]byte(`[{"id":"member-1"}]`))
	})
	mux.HandleFunc("GET /admin/realms/example.com/attack-detection/brute-force/users/member-1", func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(BruteForceStatus{Disabled: locked.Load()})
	})
	mux.HandleFunc("POST /realms/example.com/protocol/openid-connect/logout", func(writer http.ResponseWriter, request *http.Request) {
		signOuts.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return keycloakRealm{server: server, signOuts: signOuts}
}

func (realm keycloakRealm) document() string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	return `{"yon":{"oidc":{"issuer":"https://id.mesail.test/realms/{realm}","jwks":"` + realm.server.URL + `/realms/{realm}/certs","audience":"mesail-api","keycloak":"` + realm.server.URL + `/realms/{realm}/protocol/openid-connect","admin":"` + realm.server.URL + `/admin/realms/{realm}"},` +
		`"identity":{"client":{"secret":"client-secret"},"account":{"secret":"account-secret"},"session":{"key":"` + key + `"}}}}`
}

func (realm keycloakRealm) issuer(t *testing.T) SessionIssuer {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	issuer := SessionIssuerFactory{}.FromJson(realm.document())
	if issuer.Problem != nil {
		t.Fatal(issuer.Problem)
	}
	issuer.Client = realm.server.Client()
	return issuer
}

func TestSignInGrantsASession(t *testing.T) {
	realm := newKeycloakRealm(t)
	grant, err := realm.issuer(t).SignIn("Ana@Example.com", "right", "123456")
	if err != nil || grant.Caller.DomainId != "example.com" || grant.Caller.MemberId != "member-1" || grant.AccessToken == "" || grant.ExpiresIn != 60 || grant.SessionKey == "" {
		t.Fatalf("got %+v, %v", grant, err)
	}
}

func TestSessionKeyHidesTheRefreshToken(t *testing.T) {
	realm := newKeycloakRealm(t)
	grant, _ := realm.issuer(t).SignIn("ana@example.com", "right", "123456")
	raw, _ := base64.RawURLEncoding.DecodeString(grant.SessionKey)
	if bytes.Contains(raw, []byte("refresh-1")) {
		t.Fatal("the session key shows the refresh token")
	}
}

func TestSignInRefusesAWrongPassword(t *testing.T) {
	realm := newKeycloakRealm(t)
	_, err := realm.issuer(t).SignIn("ana@example.com", "wrong", "123456")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInRefusesAMissingCode(t *testing.T) {
	realm := newKeycloakRealm(t)
	_, err := realm.issuer(t).SignIn("ana@example.com", "right", "")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInReportsTheLockedAccount(t *testing.T) {
	realm := newKeycloakRealm(t)
	_, err := realm.issuer(t).SignIn("ana@example.com", "locked", "")
	if !errors.Is(err, AccountLocked{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInTreatsAnUnknownAddressAsARefusal(t *testing.T) {
	realm := newKeycloakRealm(t)
	_, err := realm.issuer(t).SignIn("bo@example.com", "wrong", "")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInRefusesAnAddressWithoutADomain(t *testing.T) {
	realm := newKeycloakRealm(t)
	_, err := realm.issuer(t).SignIn("ana", "right", "123456")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignInIsUnavailableWhenKeycloakIsDown(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	realm.server.Close()
	_, err := issuer.SignIn("ana@example.com", "right", "123456")
	if !errors.Is(err, SessionUnavailable{}) || (SessionUnavailable{}).Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestRefreshRotatesTheSession(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	first, _ := issuer.SignIn("ana@example.com", "right", "123456")
	second, err := issuer.Refresh(first.SessionKey)
	if err != nil || second.SessionKey == first.SessionKey || second.Caller.MemberId != "member-1" {
		t.Fatalf("got %+v, %v", second, err)
	}
	_, reused := issuer.Refresh(second.SessionKey)
	if !errors.Is(reused, SignInRefused{}) {
		t.Fatalf("a rotated refresh token should end with Keycloak's refusal, got %v", reused)
	}
}

func TestRefreshRefusesATamperedSessionKey(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	grant, _ := issuer.SignIn("ana@example.com", "right", "123456")
	_, err := issuer.Refresh(grant.SessionKey[:len(grant.SessionKey)-2] + "AA")
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRefreshRefusesASessionKeyWithAnotherDomain(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	content, _ := json.Marshal(SessionContent{Domain: "../admin", RefreshToken: "refresh-1"})
	sessionKey, _ := issuer.Sealer.Seal(string(content), "mesail-session")
	_, err := issuer.Refresh(sessionKey)
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRefreshRefusesASessionKeyThatIsNotJson(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	sessionKey, _ := issuer.Sealer.Seal("not json", "mesail-session")
	_, err := issuer.Refresh(sessionKey)
	if !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignOutEndsTheKeycloakSession(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	grant, _ := issuer.SignIn("ana@example.com", "right", "123456")
	err := issuer.SignOut(grant.SessionKey)
	if err != nil || realm.signOuts.Load() != 1 {
		t.Fatalf("got %v after %d sign-outs", err, realm.signOuts.Load())
	}
}

func TestSignOutRefusesATamperedSessionKey(t *testing.T) {
	realm := newKeycloakRealm(t)
	if err := realm.issuer(t).SignOut("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSignOutRefusesASessionKeyWithAnotherDomain(t *testing.T) {
	realm := newKeycloakRealm(t)
	issuer := realm.issuer(t)
	content, _ := json.Marshal(SessionContent{Domain: "../admin", RefreshToken: "refresh-1"})
	sessionKey, _ := issuer.Sealer.Seal(string(content), "mesail-session")
	if err := issuer.SignOut(sessionKey); !errors.Is(err, SignInRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSessionIssuerFactoryNeedsTheSessionKey(t *testing.T) {
	err := SessionIssuerFactory{}.FromJson(`{"yon":{"oidc":{"issuer":"https://x/realms/{realm}","jwks":"https://x/realms/{realm}/certs","audience":"mesail-api","keycloak":"https://x/realms/{realm}"},"identity":{"client":{"secret":"s"}}}}`).Problem
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnUnconfiguredSessionIssuerIsUnavailable(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	issuer := SessionIssuerFactory{}.Configured()
	_, signInErr := issuer.SignIn("ana@example.com", "right", "123456")
	_, refreshErr := issuer.Refresh("key")
	signOutErr := issuer.SignOut("key")
	if !errors.Is(signInErr, SessionUnavailable{}) || !errors.Is(refreshErr, SessionUnavailable{}) || !errors.Is(signOutErr, SessionUnavailable{}) {
		t.Fatalf("got %v, %v, %v", signInErr, refreshErr, signOutErr)
	}
}

func TestSessionErrorsTreatsAMissingConfigurationAsUnavailable(t *testing.T) {
	if err := (SessionErrors{}).Normalized(NotConfigured{}); !errors.Is(err, SessionUnavailable{}) {
		t.Fatalf("got %v", err)
	}
}
