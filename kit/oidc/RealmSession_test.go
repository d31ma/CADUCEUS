package oidc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"
)

type mesailRealm struct {
	t         *testing.T
	server    *httptest.Server
	signer    testSigner
	mutex     sync.Mutex
	changes   map[string]any
	password  string
	locked    bool
	otpSecret string
	usernames []string
	sessions  []string
	ended     []string
	deleted   []string
	signOuts  int
	user      map[string]any
	passkeys  []PasskeyRecord
	added     []PasskeyAddRequest
	grants    []url.Values
}

func newMesailRealm(t *testing.T) *mesailRealm {
	t.Helper()
	realm := &mesailRealm{t: t, signer: newTestSigner(t), changes: map[string]any{}, password: "right-password", sessions: []string{"session-a", "session-b"}, user: mesailUser()}
	admin := http.NewServeMux()
	admin.HandleFunc("GET /admin/realms/mesail/users", realm.lookup)
	admin.HandleFunc("GET /admin/realms/mesail/attack-detection/brute-force/users/person-1", realm.bruteForce)
	admin.HandleFunc("PUT /admin/realms/mesail/users/{id}/reset-password", realm.resetPassword)
	admin.HandleFunc("GET /admin/realms/mesail/users/person-1/sessions", realm.userSessions)
	admin.HandleFunc("DELETE /admin/realms/mesail/sessions/{id}", realm.endSession)
	admin.HandleFunc("GET /admin/realms/mesail/users/person-1/credentials", realm.credentials)
	admin.HandleFunc("DELETE /admin/realms/mesail/users/person-1/credentials/{id}", realm.deleteCredential)
	admin.HandleFunc("GET /admin/realms/mesail/users/person-1", realm.readUser)
	admin.HandleFunc("PUT /admin/realms/mesail/users/person-1", realm.writeUser)
	realm.handlePasskeys(admin)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/mesail/protocol/openid-connect/certs", realm.certs)
	mux.HandleFunc("POST /realms/mesail/protocol/openid-connect/token", realm.token)
	mux.HandleFunc("POST /realms/mesail/protocol/openid-connect/logout", realm.logout)
	mux.Handle("/admin/", realm.authorized(admin))
	realm.server = httptest.NewServer(realm.locking(mux))
	t.Cleanup(realm.server.Close)
	return realm
}

func mesailUser() map[string]any {
	return map[string]any{
		"id":         "person-1",
		"username":   "ana",
		"email":      "ana@example.com",
		"enabled":    true,
		"attributes": map[string]any{"mesail.mustChangePassword": []string{"true"}, "locale": []string{"en"}},
	}
}

func mesailDocument(keycloak string, admin string, key string, accountSecret string) string {
	return fmt.Sprintf(`{"yon":{"oidc":{"issuer":"https://id.mesail.test/realms/mesail","jwks":"https://id.mesail.test/realms/mesail/protocol/openid-connect/certs","audience":"mesail-api","keycloak":%q,"admin":%q},"identity":{"client":{"secret":"identity-secret"},"account":{"secret":%q},"session":{"key":%q},"passkeyorigins":"android:apk-key-hash:abc https://mail.mesail.test"}}}`, keycloak, admin, accountSecret, key)
}

func mesailSessionKey() string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
}

func (realm *mesailRealm) keycloak() string {
	return realm.server.URL + "/realms/mesail/protocol/openid-connect"
}

func (realm *mesailRealm) admin() string {
	return realm.server.URL + "/admin/realms/mesail"
}

func (realm *mesailRealm) document() string {
	return mesailDocument(realm.keycloak(), realm.admin(), mesailSessionKey(), "account-secret")
}

func (realm *mesailRealm) session(t *testing.T) RealmSession {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	session := RealmSessionFactory{}.FromJson(realm.document())
	session.Sessions.Client = realm.server.Client()
	return session
}

func (realm *mesailRealm) change(name string, value any) {
	realm.mutex.Lock()
	defer realm.mutex.Unlock()
	realm.changes[name] = value
}

func (realm *mesailRealm) locking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		realm.mutex.Lock()
		defer realm.mutex.Unlock()
		next.ServeHTTP(writer, request)
	})
}

func (realm *mesailRealm) authorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer admin-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (realm *mesailRealm) certs(writer http.ResponseWriter, request *http.Request) {
	_ = json.NewEncoder(writer).Encode(realm.signer.keySet())
}

func (realm *mesailRealm) accessToken() string {
	claims := map[string]any{
		"iss":                realm.server.URL + "/realms/mesail",
		"sub":                "person-1",
		"aud":                []string{"mesail-identity", "account"},
		"azp":                "mesail-identity",
		"exp":                time.Now().Unix() + 60,
		"iat":                time.Now().Unix(),
		"typ":                "Bearer",
		"sid":                "session-a",
		"preferred_username": "ana",
		"name":               "Ana Example",
		"realm_access":       map[string]any{"roles": []string{"mesail-member"}},
	}
	maps.Copy(claims, realm.changes)
	return realm.signer.rsaToken(realm.t, "rsa-1", claims)
}

func (realm *mesailRealm) token(writer http.ResponseWriter, request *http.Request) {
	_ = request.ParseForm()
	form := request.PostForm
	switch {
	case form.Get("grant_type") == "client_credentials" && form.Get("client_id") == "mesail-account" && form.Get("client_secret") == "account-secret":
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "admin-token", "expires_in": 60})
	case form.Get("client_id") != "mesail-identity" || form.Get("client_secret") != "identity-secret":
		writer.WriteHeader(http.StatusUnauthorized)
	case form.Get("grant_type") == "password":
		realm.passwordGrant(writer, form)
	case form.Get("grant_type") == "refresh_token" && form.Get("refresh_token") == "refresh-1":
		realm.granted(writer, "refresh-2")
	default:
		realm.refused(writer)
	}
}

func (realm *mesailRealm) passwordGrant(writer http.ResponseWriter, form url.Values) {
	realm.usernames = append(realm.usernames, form.Get("username"))
	codeOk := realm.otpSecret == "" || (Totp{}).Matches(realm.otpSecret, form.Get("totp"), time.Now())
	switch {
	case form.Has("passkey_assertion"):
		realm.passkeyGrant(writer, form)
	case form.Get("password") == "locked":
		realm.locked = true
		realm.refused(writer)
	case form.Get("username") == "ana" && form.Get("password") == realm.password && codeOk:
		realm.granted(writer, "refresh-1")
	default:
		realm.refused(writer)
	}
}

func (realm *mesailRealm) granted(writer http.ResponseWriter, refresh string) {
	_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": realm.accessToken(), "refresh_token": refresh, "expires_in": 60})
}

func (realm *mesailRealm) refused(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusBadRequest)
	_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
}

func (realm *mesailRealm) logout(writer http.ResponseWriter, request *http.Request) {
	realm.signOuts++
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) lookup(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if query.Get("exact") != "true" || query.Get("username") != "ana" {
		_, _ = writer.Write([]byte(`[]`))
		return
	}
	_, _ = writer.Write([]byte(`[{"id":"person-1"}]`))
}

func (realm *mesailRealm) bruteForce(writer http.ResponseWriter, request *http.Request) {
	_ = json.NewEncoder(writer).Encode(BruteForceStatus{Disabled: realm.locked})
}

func (realm *mesailRealm) resetPassword(writer http.ResponseWriter, request *http.Request) {
	reset := PasswordReset{}
	_ = json.NewDecoder(request.Body).Decode(&reset)
	if len(reset.Value) < 12 || reset.Temporary || reset.Type != "password" {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalidPasswordMinLengthMessage","error_description":"Invalid password: minimum length 12."}`))
		return
	}
	realm.password = reset.Value
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) userSessions(writer http.ResponseWriter, request *http.Request) {
	sessions := []UserSession{}
	for _, id := range realm.sessions {
		sessions = append(sessions, UserSession{Id: id})
	}
	_ = json.NewEncoder(writer).Encode(sessions)
}

func (realm *mesailRealm) endSession(writer http.ResponseWriter, request *http.Request) {
	realm.ended = append(realm.ended, request.PathValue("id"))
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) credentials(writer http.ResponseWriter, request *http.Request) {
	credentials := []UserCredential{{Id: "password-1", Type: "password"}}
	if realm.otpSecret != "" {
		credentials = append(credentials, UserCredential{Id: "otp-1", Type: "otp"})
	}
	_ = json.NewEncoder(writer).Encode(credentials)
}

func (realm *mesailRealm) deleteCredential(writer http.ResponseWriter, request *http.Request) {
	realm.deleted = append(realm.deleted, request.PathValue("id"))
	if request.PathValue("id") == "otp-1" {
		realm.otpSecret = ""
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) readUser(writer http.ResponseWriter, request *http.Request) {
	_ = json.NewEncoder(writer).Encode(realm.user)
}

func (realm *mesailRealm) writeUser(writer http.ResponseWriter, request *http.Request) {
	raw, _ := io.ReadAll(request.Body)
	body := map[string]any{}
	_ = json.Unmarshal(raw, &body)
	if _, hasCredentials := body["credentials"]; hasCredentials {
		realm.storeOtp(raw)
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	realm.user = body
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) storeOtp(raw []byte) {
	update := UserCredentials{}
	_ = json.Unmarshal(raw, &update)
	secret := OtpSecretData{}
	_ = json.Unmarshal([]byte(update.Credentials[0].SecretData), &secret)
	realm.otpSecret = secret.Value
}

func realmIs(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("got error %v, want %v", err, target)
	}
}

func realmEqual[T comparable](t *testing.T, name string, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %+v, want %+v", name, got, want)
	}
}

func realmList(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
}

func realmUnavailable(t *testing.T, session RealmSession) {
	t.Helper()
	_, signInErr := session.SignIn("ana", "right-password", "")
	_, refreshErr := session.Refresh("refresh-1")
	signOutErr := session.SignOut("refresh-1")
	_, sealErr := session.Seal(SessionContent{Domain: "mesail", RefreshToken: "refresh-1"})
	_, openErr := session.Open("key")
	for _, err := range []error{signInErr, refreshErr, signOutErr, sealErr, openErr} {
		realmIs(t, err, SessionUnavailable{})
	}
}

func TestRealmSessionSignsInByUsername(t *testing.T) {
	realm := newMesailRealm(t)
	opened, err := realm.session(t).SignIn("ana", "right-password", "")
	realmIs(t, err, nil)
	realmEqual(t, "session", opened, KeycloakSession{PersonId: "person-1", SessionId: "session-a", Username: "ana", DisplayName: "Ana Example", RefreshToken: "refresh-1"})
	realmList(t, "usernames", realm.usernames, []string{"ana"})
}

func TestRealmSessionReadsTheOperatorRoleAndThePasswordFlag(t *testing.T) {
	realm := newMesailRealm(t)
	realm.change("realm_access", map[string]any{"roles": []string{"mesail-member", "mesail-operator"}})
	realm.change("mesail_must_change_password", true)
	opened, err := realm.session(t).SignIn("ana", "right-password", "")
	realmIs(t, err, nil)
	realmEqual(t, "session", opened, KeycloakSession{PersonId: "person-1", SessionId: "session-a", Username: "ana", DisplayName: "Ana Example", Operator: true, MustChangePassword: true, RefreshToken: "refresh-1"})
}

func TestRealmSessionRefusesAWrongPassword(t *testing.T) {
	realm := newMesailRealm(t)
	opened, err := realm.session(t).SignIn("ana", "wrong", "")
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "session", opened, KeycloakSession{})
}

func TestRealmSessionReportsTheLockedAccount(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.session(t).SignIn("ana", "locked", "")
	realmIs(t, err, AccountLocked{})
}

func TestRealmSessionIsUnavailableWhenKeycloakIsDown(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	realm.server.Close()
	_, err := session.SignIn("ana", "right-password", "")
	realmIs(t, err, SessionUnavailable{})
}

func TestRealmSessionRefusesATokenForAnotherAudience(t *testing.T) {
	realm := newMesailRealm(t)
	realm.change("aud", []string{"mesail-api"})
	_, err := realm.session(t).SignIn("ana", "right-password", "")
	realmIs(t, err, SignInRefused{})
}

func TestRealmSessionRefusesATokenFromAnotherIssuer(t *testing.T) {
	realm := newMesailRealm(t)
	realm.change("iss", "https://id.mesail.test/realms/mesail")
	_, err := realm.session(t).SignIn("ana", "right-password", "")
	realmIs(t, err, SignInRefused{})
}

func TestRealmSessionRefreshes(t *testing.T) {
	realm := newMesailRealm(t)
	opened, err := realm.session(t).Refresh("refresh-1")
	realmIs(t, err, nil)
	realmEqual(t, "refresh token", opened.RefreshToken, "refresh-2")
	realmEqual(t, "person", opened.PersonId, "person-1")
}

func TestRealmSessionRefusesABadRefreshToken(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.session(t).Refresh("refresh-9")
	realmIs(t, err, SignInRefused{})
}

func TestRealmSessionSignsOut(t *testing.T) {
	realm := newMesailRealm(t)
	err := realm.session(t).SignOut("refresh-1")
	realmIs(t, err, nil)
	realmEqual(t, "sign-outs", realm.signOuts, 1)
}

func TestRealmSessionSealsAndOpensTheContent(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	content := SessionContent{Domain: "example.com", RefreshToken: "refresh-1"}
	sealed, sealErr := session.Seal(content)
	opened, openErr := session.Open(sealed)
	realmIs(t, sealErr, nil)
	realmIs(t, openErr, nil)
	realmEqual(t, "content", opened, content)
}

func TestRealmSessionRefusesATamperedKey(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	sealed, _ := session.Seal(SessionContent{Domain: "example.com", RefreshToken: "refresh-1"})
	_, err := session.Open(sealed[:len(sealed)-4] + "AAAA")
	realmIs(t, err, SignInRefused{})
}

func TestRealmSessionNeedsTheSessionKey(t *testing.T) {
	realm := newMesailRealm(t)
	t.Setenv("TMPDIR", t.TempDir())
	session := RealmSessionFactory{}.FromJson(mesailDocument(realm.keycloak(), realm.admin(), "", "account-secret"))
	realmUnavailable(t, session)
	realmList(t, "usernames", realm.usernames, nil)
}

func TestRealmSessionFactoryReadsTheEnvironment(t *testing.T) {
	realm := newMesailRealm(t)
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", realm.document())
	session := RealmSessionFactory{}.Configured()
	session.Sessions.Client = realm.server.Client()
	opened, err := session.SignIn("ana", "right-password", "")
	realmIs(t, err, nil)
	realmEqual(t, "person", opened.PersonId, "person-1")
}

func TestRealmSessionVerifierExpectsTheKeycloakIssuer(t *testing.T) {
	verifier := VerifierFactory{}.Keycloak("https://id.mesail.test/realms/mesail/protocol/openid-connect")
	realmEqual(t, "issuer", verifier.Issuer, "https://id.mesail.test/realms/mesail")
	realmEqual(t, "key set", verifier.Jwks, "https://id.mesail.test/realms/mesail/protocol/openid-connect/certs")
	realmEqual[ClaimChecker](t, "rules", verifier.Rules, ClaimRules{Audience: "mesail-identity"})
}
