package scheduling

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const relayPath = "/api/v1/directory/outbox/relay"

type countingRoute struct {
	calls atomic.Int32
	path  atomic.Value
}

func (route *countingRoute) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	route.calls.Add(1)
	route.path.Store(request.URL.Path)
}

func callsFor(t *testing.T, applicationJson string) *countingRoute {
	t.Helper()
	route := &countingRoute{}
	server := httptest.NewServer(route)
	t.Cleanup(server.Close)
	WorkerSwitch{ApplicationJson: applicationJson}.Run(func() {
		response, err := http.Post(server.URL+relayPath, "application/json", nil)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		response.Body.Close()
	})
	return route
}

func wantOneCall(t *testing.T, route *countingRoute) {
	t.Helper()
	if got, path := route.calls.Load(), route.path.Load(); got != 1 || path != relayPath {
		t.Fatalf("got %d calls to %v, want 1 to %s", got, path, relayPath)
	}
}

func TestLambdaProfileMakesNoCall(t *testing.T) {
	route := callsFor(t, `{"yon":{"server":{"host":"127.0.0.1","port":8102},"workers":{"enabled":false}}}`)
	if got := route.calls.Load(); got != 0 {
		t.Fatalf("got %d calls, want 0", got)
	}
}

func TestLambdaProfileNeverStartsTheWork(t *testing.T) {
	started := false
	WorkerSwitch{ApplicationJson: `{"yon":{"workers":{"enabled":false}}}`}.Run(func() { started = true })
	if started {
		t.Fatal("the work started in the Lambda profile")
	}
}

func TestNativeProfileCallsTheRouteOnce(t *testing.T) {
	route := callsFor(t, `{"yon":{"server":{"host":"127.0.0.1","port":8102}}}`)
	wantOneCall(t, route)
}

func TestMissingSettingsCallTheRouteOnce(t *testing.T) {
	route := callsFor(t, "")
	wantOneCall(t, route)
}

func TestContainerProfileCallsTheRouteOnce(t *testing.T) {
	route := callsFor(t, `{"yon":{"server":{"hosts":["realtime"],"port":8110}}}`)
	wantOneCall(t, route)
}

func TestWorkersTurnedOnCallTheRouteOnce(t *testing.T) {
	route := callsFor(t, `{"yon":{"workers":{"enabled":true}}}`)
	wantOneCall(t, route)
}

func TestUnreadableSettingsCallTheRouteOnce(t *testing.T) {
	route := callsFor(t, `not json`)
	wantOneCall(t, route)
}
