package trigger

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func serverFor(t *testing.T, status int, seen *http.Request) (LocalRoute, string) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*seen = *request
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	address, _ := url.Parse(server.URL)
	dial := func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address.Host)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: dial}}
	return LocalRoute{Client: client}, fmt.Sprintf(`{"yon":{"server":{"port":%s}}}`, address.Port())
}

func TestPostCallsTheRouteWithTheToken(t *testing.T) {
	seen := http.Request{}
	route, settings := serverFor(t, http.StatusOK, &seen)
	err := route.Post(settings, "/api/v1/directory/jobs/removals", "Bearer example", nil)
	if err != nil || seen.Method != http.MethodPost || seen.URL.Path != "/api/v1/directory/jobs/removals" || seen.Header.Get("Authorization") != "Bearer example" || seen.Host[:len("directory.localhost:")] != "directory.localhost:" {
		t.Fatalf("got %v, %s %s %s %q", err, seen.Method, seen.Host, seen.URL.Path, seen.Header.Get("Authorization"))
	}
}

func TestPostReportsARefusedRoute(t *testing.T) {
	seen := http.Request{}
	route, settings := serverFor(t, http.StatusForbidden, &seen)
	err := route.Post(settings, "/jobs", "Bearer example", nil)
	refused := RouteRefused{}
	if !errors.As(err, &refused) || refused.Status != http.StatusForbidden || refused.Error() != "the scheduled route answered 403" {
		t.Fatalf("got %v", err)
	}
}

func TestPostSendsNothingWithoutAToken(t *testing.T) {
	missing := errors.New("no token")
	err := LocalRoute{}.Post(`{}`, "/jobs", "", missing)
	if !errors.Is(err, missing) {
		t.Fatalf("got %v", err)
	}
}

func TestPostFailsOnSettingsThatAreNotJson(t *testing.T) {
	err := LocalRoute{}.Post(`{`, "/jobs", "Bearer example", nil)
	if err == nil {
		t.Fatalf("want a failure")
	}
}

func TestPostFailsWithoutAPort(t *testing.T) {
	err := LocalRoute{}.Post(`{"yon":{}}`, "/jobs", "Bearer example", nil)
	if err == nil || err.Error() != "the application settings name no server port" {
		t.Fatalf("got %v", err)
	}
}

func TestPostFailsOnAPathThatIsNotAUrl(t *testing.T) {
	err := LocalRoute{}.Post(`{"yon":{"server":{"port":1}}}`, "/\x7f", "Bearer example", nil)
	if err == nil {
		t.Fatalf("want a failure")
	}
}

func TestPostFailsWhenNothingAnswers(t *testing.T) {
	seen := http.Request{}
	route, settings := serverFor(t, http.StatusOK, &seen)
	route.Client = &http.Client{Transport: failingTransport{}}
	err := route.Post(settings, "/jobs", "Bearer example", nil)
	if err == nil {
		t.Fatalf("want a failure")
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}
