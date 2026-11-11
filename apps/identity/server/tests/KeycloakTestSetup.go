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
		setup.answer(writer, request.URL.Path, request.PostForm.Get("password"), request.PostForm.Get("refresh_token"), realm)
	}))
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	setup.useSettings(realm, oidc.SessionSettings{
		Keycloak: server.URL + "/realms/{realm}/protocol/openid-connect",
		Client:   oidc.ClientSetting{Secret: "client-secret"},
		Session:  oidc.KeySetting{Key: key},
	})
}

func (setup KeycloakTestSetup) answer(writer http.ResponseWriter, path string, password string, refreshToken string, realm oidc.SigningRealm) {
	switch {
	case strings.HasSuffix(path, "/logout"):
		writer.WriteHeader(http.StatusNoContent)
	case password == "right" || refreshToken == "refresh-1":
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": setup.Token(realm, setup.Member()), "refresh_token": "refresh-1", "expires_in": 60})
	case password == "locked":
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Account temporarily disabled"}`))
	default:
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
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
