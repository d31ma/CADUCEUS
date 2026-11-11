import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/oidc"
)

// @TestSetup
type GatewayTestSetup struct{}

func (GatewayTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup GatewayTestSetup) Secret() string {
	return strings.Repeat("g", 32)
}

func (setup GatewayTestSetup) Realm(gatewayUrl string) oidc.SigningRealm {
	folder, err := os.MkdirTemp("", "realtime-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.Started("example.com")
	if err != nil {
		panic(err)
	}
	setup.UseSettings(realm, gatewayUrl, setup.Secret())
	if err := realm.Publish(time.Now()); err != nil {
		panic(err)
	}
	return realm
}

func (setup GatewayTestSetup) UseSettings(realm oidc.SigningRealm, gatewayUrl string, secret string) {
	gateway := map[string]any{"url": gatewayUrl, "secret": secret}
	document, err := json.Marshal(map[string]any{"yon": map[string]any{"oidc": realm.Settings, "realtime": map[string]any{"gateway": gateway}}})
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(document))
}

func (setup GatewayTestSetup) Gateway(closes chan string) string {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case closes <- request.Method + " " + request.RequestURI + " " + request.Header.Get("X-Realtime-Gateway"):
		default:
		}
		writer.WriteHeader(http.StatusOK)
	}))
	return server.URL
}

func (setup GatewayTestSetup) Token(realm oidc.SigningRealm, roles []string) string {
	token, err := realm.Token(oidc.TokenRequest{Subject: "member-1", RealmRoles: roles, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 300})
	if err != nil {
		panic(err)
	}
	return token
}
