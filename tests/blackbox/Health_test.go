//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

func TestEveryServiceAnswersHealthWithoutAToken(t *testing.T) {
	stack := NewStack(t)
	services := []string{"identity", "directory", "channels", "messaging", "attachments", "realtime", "notify", "search", "mail-in", "mail-out"}
	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			var health struct {
				Service string `json:"service"`
				Status  string `json:"status"`
			}
			stack.Call(t, http.MethodGet, "/api/v1/"+service+"/health", "", nil).Expect(t, http.StatusOK).Decode(t, &health)
			if health.Service != service || health.Status != "ok" {
				t.Fatalf("got %+v", health)
			}
		})
	}
}
