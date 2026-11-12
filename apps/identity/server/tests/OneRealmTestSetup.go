import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"mesail.dev/packages/oidc"
)

// @TestSetup
type OneRealmTestSetup struct{}

func (OneRealmTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup OneRealmTestSetup) Start() oidc.SigningKey {
	folder, err := os.MkdirTemp("", "identity-one-realm-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	keycloakKey, _ := oidc.SigningKey{}.Generated()
	signingKey, _ := oidc.SigningKey{}.Generated()
	signingText, _ := signingKey.Text()
	locked := &atomic.Bool{}
	keycloak := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		setup.keycloak(writer, request, keycloakKey, locked)
	}))
	directory := httptest.NewServer(http.HandlerFunc(setup.directory))
	sessionKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32)))
	document := `{"yon":{"oidc":{"issuer":"` + setup.Issuer() + `","jwks":"` + setup.Issuer() + `/jwks","audience":"mesail-api",` +
		`"keycloak":"` + keycloak.URL + `/realms/mesail/protocol/openid-connect","admin":"` + keycloak.URL + `/admin/realms/mesail"},` +
		`"identity":{"issuer":"` + setup.Issuer() + `","client":{"secret":"client-secret"},"account":{"secret":"account-secret"},"session":{"key":"` + sessionKey + `"},"signing":{"key":"` + signingText + `"}},` +
		`"directory":{"url":"` + directory.URL + `"},"service":{"client":{"secret":"service-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", document)
	return signingKey
}

func (OneRealmTestSetup) Issuer() string {
	return "https://mail.mesail.test/api/v1/identity"
}

func (setup OneRealmTestSetup) Member() oidc.ClaimsDocument {
	return oidc.ClaimsDocument{
		Issuer:          setup.Issuer(),
		Subject:         "person-ana",
		Audience:        oidc.AudienceList{"mesail-api"},
		AuthorizedParty: "mesail-web",
		ExpiresAt:       time.Now().Unix() + 60,
		Name:            "Ana Example",
		Username:        "ana",
		Email:           "ana@example.com",
		Domain:          "example.com",
		RealmAccess:     oidc.RealmAccess{Roles: []string{"mesail-member"}},
	}
}

func (OneRealmTestSetup) Token(key oidc.SigningKey, document oidc.ClaimsDocument) string {
	keyId, _ := key.KeyId()
	token, err := oidc.JwtWriter{}.Signed(key, keyId, document)
	if err != nil {
		panic(err)
	}
	return token
}

func (setup OneRealmTestSetup) keycloak(writer http.ResponseWriter, request *http.Request, key oidc.SigningKey, locked *atomic.Bool) {
	_ = request.ParseForm()
	form := request.PostForm
	switch {
	case strings.HasSuffix(request.URL.Path, "/certs"):
		public, _ := key.PublicKey()
		_ = json.NewEncoder(writer).Encode(oidc.KeySet{Keys: []oidc.JsonWebKey{public}})
	case strings.HasSuffix(request.URL.Path, "/logout"):
		writer.WriteHeader(http.StatusNoContent)
	case form.Get("client_id") == "mesail-account":
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "admin-token", "expires_in": 60})
	case strings.Contains(request.URL.Path, "/admin/"):
		setup.admin(writer, request, locked)
	case form.Get("username") == "ana" && form.Get("password") == "right" || form.Get("refresh_token") == "refresh-ana":
		setup.grant(writer, request, key)
	case form.Get("password") == "locked":
		locked.Store(true)
		setup.refused(writer)
	default:
		setup.refused(writer)
	}
}

func (OneRealmTestSetup) refused(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusBadRequest)
	_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid user credentials"}`))
}

func (OneRealmTestSetup) admin(writer http.ResponseWriter, request *http.Request, locked *atomic.Bool) {
	reset := oidc.PasswordReset{}
	_ = json.NewDecoder(request.Body).Decode(&reset)
	path := request.URL.Path
	switch {
	case request.Header.Get("Authorization") != "Bearer admin-token":
		writer.WriteHeader(http.StatusUnauthorized)
	case strings.HasSuffix(path, "/reset-password") && len(reset.Value) < 12:
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalidPasswordMinLengthMessage","error_description":"Invalid password: minimum length 12."}`))
	case strings.HasSuffix(path, "/credentials"):
		_, _ = writer.Write([]byte(`[{"id":"password-1","type":"password"}]`))
	case strings.HasSuffix(path, "/users") && request.URL.Query().Get("username") == "ana":
		_, _ = writer.Write([]byte(`[{"id":"person-ana"}]`))
	case strings.Contains(path, "/attack-detection/"):
		_ = json.NewEncoder(writer).Encode(oidc.BruteForceStatus{Disabled: locked.Load()})
	case strings.HasSuffix(path, "/sessions"):
		_, _ = writer.Write([]byte(`[{"id":"session-other"}]`))
	case request.Method == http.MethodGet && strings.HasSuffix(path, "/users/person-ana"):
		_, _ = writer.Write([]byte(`{"id":"person-ana","username":"ana","attributes":{"mesail.mustChangePassword":["true"]}}`))
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (OneRealmTestSetup) grant(writer http.ResponseWriter, request *http.Request, key oidc.SigningKey) {
	issuer := "http://" + request.Host + "/realms/mesail"
	keyId, _ := key.KeyId()
	document := oidc.ClaimsDocument{Issuer: issuer, Subject: "person-ana", Audience: oidc.AudienceList{"mesail-identity"}, AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60, TokenType: "Bearer", SessionId: "session-ana", Name: "Ana Example", Username: "ana"}
	token, _ := oidc.JwtWriter{}.Signed(key, keyId, document)
	_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": token, "refresh_token": "refresh-ana", "expires_in": 60})
}

func (OneRealmTestSetup) directory(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/api/v1/directory/people/person-ana" || !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	_, _ = writer.Write([]byte(`{"personId":"person-ana","username":"ana","displayName":"Ana Example","domainIds":["example.com","example.org"],"adminDomainIds":["example.com"],"defaultDomainId":"example.com","state":"active"}`))
}

func (OneRealmTestSetup) Unconfigured() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
}
