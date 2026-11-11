import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/oidc"
)

// @TestSetup
type KeycloakTestSetup struct{}

func (KeycloakTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup KeycloakTestSetup) Realm(name string) oidc.SigningRealm {
	folder, err := os.MkdirTemp("", "identity-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.Started(name)
	if err != nil {
		panic(err)
	}
	setup.useSettings(realm, oidc.SessionSettings{})
	if err := realm.Publish(time.Now()); err != nil {
		panic(err)
	}
	return realm
}

func (setup KeycloakTestSetup) Keycloak(realm oidc.SigningRealm) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		setup.answer(writer, request, realm)
	}))
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	realm.Settings.Keycloak = server.URL + "/realms/{realm}/protocol/openid-connect"
	realm.Settings.Admin = server.URL + "/admin/realms/{realm}"
	setup.useSettings(realm, oidc.SessionSettings{
		Client:  oidc.ClientSetting{Secret: "client-secret"},
		Session: oidc.KeySetting{Key: key},
		Account: oidc.ClientSetting{Secret: "account-secret"},
	})
}

func (setup KeycloakTestSetup) answer(writer http.ResponseWriter, request *http.Request, realm oidc.SigningRealm) {
	form := request.PostForm
	path := request.URL.Path
	switch {
	case strings.Contains(path, "/admin/"):
		setup.admin(writer, request)
	case strings.HasSuffix(path, "/logout"):
		writer.WriteHeader(http.StatusNoContent)
	case form.Get("client_id") == "mesail-account":
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "admin-token", "expires_in": 60})
	case form.Get("password") == "right" || form.Get("refresh_token") == "refresh-1":
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": setup.Token(realm, setup.Member()), "refresh_token": "refresh-1", "expires_in": 60})
	case form.Get("password") == "locked":
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Account temporarily disabled"}`))
	default:
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
	}
}

func (KeycloakTestSetup) admin(writer http.ResponseWriter, request *http.Request) {
	reset := oidc.PasswordReset{}
	_ = json.NewDecoder(request.Body).Decode(&reset)
	switch {
	case request.Header.Get("Authorization") != "Bearer admin-token":
		writer.WriteHeader(http.StatusUnauthorized)
	case strings.HasSuffix(request.URL.Path, "/reset-password") && len(reset.Value) < 12:
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalidPasswordMinLengthMessage","error_description":"Invalid password: minimum length 12."}`))
	case strings.HasSuffix(request.URL.Path, "/credentials"):
		_, _ = writer.Write([]byte(`[{"id":"password-1","type":"password"}]`))
	case strings.HasSuffix(request.URL.Path, "/sessions"):
		_, _ = writer.Write([]byte(`[{"id":"session-other"}]`))
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (KeycloakTestSetup) useSettings(realm oidc.SigningRealm, session oidc.SessionSettings) {
	document, err := json.Marshal(oidc.ApplicationSnapshot{Yon: oidc.ApplicationSettings{Oidc: realm.Settings, Identity: session}})
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(document))
}

func (KeycloakTestSetup) Member() oidc.TokenRequest {
	return oidc.TokenRequest{Subject: "member-1", Name: "Ana Example", Email: "ana@example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60}
}

func (KeycloakTestSetup) Token(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	token, err := realm.Token(request)
	if err != nil {
		panic(err)
	}
	return token
}

func (KeycloakTestSetup) Unconfigured() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
}
