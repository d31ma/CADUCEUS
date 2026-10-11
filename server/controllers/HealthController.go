import "net/http"

// HealthController answers /health, the liveness check the entry point waits
// for before it starts the mail services and that the container's
// HEALTHCHECK polls. It reads nothing and reveals nothing.
// @RequestRoute("/health")
// @Public
type HealthController struct{}

// GetHealth answers GET /health with {"ok":true} once the server answers.
// @GetRoute
func (*HealthController) GetHealth(request *http.Request) *http.Response {
	return Answer{}.Ok(map[string]any{"ok": true})
}
