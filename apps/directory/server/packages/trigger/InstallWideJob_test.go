package trigger

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"mesail.dev/packages/oidc"
)

func identityFor(t *testing.T, seen *oidc.ServiceTokenRequest) string {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewDecoder(request.Body).Decode(seen)
		writer.Write([]byte(`{"accessToken":"example-token","expiresIn":60}`))
	}))
	t.Cleanup(server.Close)
	return server.URL + "/api/v1/identity/service-token"
}

func TestRunWithoutServiceTokenSettingsIsNotConfigured(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	err := InstallWideJob{Path: "/api/v1/directory/jobs/removals"}.Run(`{"yon":{}}`)
	if !errors.As(err, &oidc.NotConfigured{}) {
		t.Fatalf("got %v, want NotConfigured", err)
	}
}

func TestRunAsksIdentityForAnInstallWideToken(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	seen := oidc.ServiceTokenRequest{}
	settings := fmt.Sprintf(`{"yon":{"oidc":{"servicetokens":%q},"service":{"client":{"secret":"example-secret"}}}}`, identityFor(t, &seen))
	err := InstallWideJob{Path: "/api/v1/directory/outbox/relay"}.Run(settings)
	if err == nil || seen.DomainId != (oidc.InstallWide{}).DomainId() || seen.ClientId != "mesail-service" {
		t.Fatalf("got %v, %+v", err, seen)
	}
}
