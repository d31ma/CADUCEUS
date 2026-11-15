package adapters

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mesail.dev/packages/oidc"
)

func TestScheduledCleanupCallsTheJobOnTheRelayHost(t *testing.T) {
	cleanup := ScheduledCleanup{}
	if got := cleanup.CleanupUrl("http://127.0.0.1:8104/api/v1/messaging/outbox/relay"); got != "http://127.0.0.1:8104/api/v1/messaging/jobs/changes-cleanup" {
		t.Fatalf("got %q", got)
	}
	if cleanup.CleanupUrl("") != "" || cleanup.CleanupUrl("http://bad host/relay") != "" {
		t.Fatal("want no url")
	}
}

func TestScheduledCleanupFetchesAnInstallWideTokenAndCallsTheJob(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	asked := oidc.ServiceTokenRequest{}
	identity := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&asked)
		_, _ = writer.Write([]byte(`{"accessToken":"install-wide-token","expiresIn":60}`))
	}))
	t.Cleanup(identity.Close)
	path := ""
	job := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_, _ = writer.Write([]byte(`{"dropped":0,"more":false}`))
	}))
	t.Cleanup(job.Close)
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"serviceTokens":"`+identity.URL+`/api/v1/identity/service-token"},"service":{"client":{"secret":"s"}},"messaging":{"relayurl":"`+job.URL+`/api/v1/messaging/outbox/relay"}}}`)
	if err := (ScheduledCleanup{}).Run(); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/messaging/jobs/changes-cleanup" || asked.DomainId != (oidc.InstallWide{}).DomainId() {
		t.Fatalf("got %q %+v", path, asked)
	}
}
