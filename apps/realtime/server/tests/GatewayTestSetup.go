import (
	"encoding/json"
	"io"
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
	started, err := oidc.SigningRealm{}.StartedMesail()
	if err != nil {
		panic(err)
	}
	realm := started.ForDomain("example.com")
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
	return setup.signed(realm, oidc.TokenRequest{Subject: "member-1", Username: "member-1", RealmRoles: roles, AuthorizedParty: "mesail-web"})
}

func (setup GatewayTestSetup) ServiceToken(realm oidc.SigningRealm) string {
	return setup.signed(realm, oidc.TokenRequest{Subject: "service", RealmRoles: []string{"mesail-service"}, AuthorizedParty: "mesail-service"})
}

func (setup GatewayTestSetup) signed(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	request.Audience = "mesail-api"
	request.ExpiresAt = time.Now().Unix() + 300
	token, err := realm.Token(request)
	if err != nil {
		panic(err)
	}
	return token
}

func (setup GatewayTestSetup) Install(frames chan string) (oidc.SigningRealm, string) {
	server := httptest.NewServer(setup.routes(frames))
	realm := setup.Realm(server.URL)
	key := setup.CallKey()
	setup.UseSignallingSettings(realm, server.URL, key)
	return realm, key
}

func (setup GatewayTestSetup) CallKey() string {
	key, err := oidc.SigningKey{}.Generated()
	if err != nil {
		panic(err)
	}
	text, err := key.Text()
	if err != nil {
		panic(err)
	}
	return text
}

func (setup GatewayTestSetup) UseSignallingSettings(realm oidc.SigningRealm, installUrl string, callKey string) {
	settings := realm.Settings
	settings.ServiceTokens = installUrl + "/service-token"
	realtime := map[string]any{
		"gateway":      map[string]any{"url": installUrl, "secret": setup.Secret()},
		"directoryurl": installUrl,
		"notifyurl":    installUrl,
		"calls":        map[string]any{"key": callKey},
	}
	yon := map[string]any{"oidc": settings, "service": map[string]any{"client": map[string]any{"secret": "service-secret"}}, "realtime": realtime}
	document, err := json.Marshal(map[string]any{"yon": yon})
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(document))
}

func (setup GatewayTestSetup) MemberToken(realm oidc.SigningRealm, subject string) string {
	return setup.signed(realm, oidc.TokenRequest{Subject: subject, Username: subject, RealmRoles: []string{"mesail-member"}, AuthorizedParty: "mesail-web"})
}

func (setup GatewayTestSetup) Next(frames chan string) string {
	select {
	case frame := <-frames:
		return frame
	case <-time.After(10 * time.Second):
		return "no frame within 10 seconds"
	}
}

func (setup GatewayTestSetup) routes(frames chan string) *http.ServeMux {
	routes := http.NewServeMux()
	routes.HandleFunc("POST /@connections/{connectionId}", func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		setup.keepSignalling(frames, request.PathValue("connectionId"), string(body))
	})
	routes.HandleFunc("DELETE /@connections/{connectionId}", func(writer http.ResponseWriter, request *http.Request) {
		frames <- request.PathValue("connectionId") + " closed " + request.URL.Query().Get("code")
	})
	routes.HandleFunc("POST /service-token", func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, `{"accessToken":"service-token","expiresIn":300}`)
	})
	routes.HandleFunc("POST /api/v1/directory/devices/owners", func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, `{"owners":{"device-1":"member-1","device-2":"member-2"}}`)
	})
	return routes
}

func (setup GatewayTestSetup) keepSignalling(frames chan string, connectionId string, body string) {
	if strings.HasPrefix(body, `{"type":"presence.changed"`) || strings.HasPrefix(body, `{"type":"auth.accepted"`) {
		return
	}
	frames <- connectionId + " " + body
}
