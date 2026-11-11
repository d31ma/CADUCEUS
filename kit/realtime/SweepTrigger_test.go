package realtime

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type presenceTokens struct {
	realm string
	err   error
}

func (tokens *presenceTokens) Authorization(realm string) (string, error) {
	tokens.realm = realm
	return "Bearer operators", tokens.err
}

func sweepTrigger(t *testing.T, status int, tokens *presenceTokens, seen *http.Request) SweepTrigger {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*seen = *request
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	address, _ := url.Parse(server.URL)
	settings := `{"yon":{"server":{"port":` + address.Port() + `}}}`
	return SweepTrigger{Tokens: tokens, Environment: func(string) string { return settings }, Client: server.Client()}
}

func TestFirePostsToTheSweepRouteWithAnOperatorsToken(t *testing.T) {
	tokens := &presenceTokens{}
	seen := http.Request{}
	err := sweepTrigger(t, http.StatusOK, tokens, &seen).Fire()
	if err != nil || seen.Method != http.MethodPost || seen.URL.Path != "/api/v1/realtime/jobs/sweep" || seen.Header.Get("Authorization") != "Bearer operators" || tokens.realm != "mesail-operators" {
		t.Fatalf("got %v %s %s %q", err, seen.Method, seen.URL, tokens.realm)
	}
}

func TestFireReportsARefusedSweep(t *testing.T) {
	seen := http.Request{}
	err := sweepTrigger(t, http.StatusNotFound, &presenceTokens{}, &seen).Fire()
	if !errors.As(err, &TriggerFailed{}) {
		t.Fatalf("got %v", err)
	}
}

func TestFireStopsWithoutAToken(t *testing.T) {
	seen := http.Request{}
	failure := errors.New("no token")
	err := sweepTrigger(t, http.StatusOK, &presenceTokens{err: failure}, &seen).Fire()
	if !errors.Is(err, failure) || seen.Method != "" {
		t.Fatalf("got %v", err)
	}
}

func TestFireNeedsTheServerPort(t *testing.T) {
	trigger := SweepTrigger{Tokens: &presenceTokens{}, Environment: func(string) string { return "{}" }, Client: http.DefaultClient}
	if err := trigger.Fire(); !errors.As(err, &TriggerFailed{}) || err.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestConfiguredReadsTheApplicationSettings(t *testing.T) {
	trigger := SweepTrigger{}.Configured(func(string) string { return `{"yon":{"server":{"port":8106}}}` })
	address, err := trigger.address()
	if err != nil || address != "http://127.0.0.1:8106/api/v1/realtime/jobs/sweep" || trigger.Client.Timeout.Seconds() != 30 || trigger.Tokens == nil {
		t.Fatalf("got %q %v", address, err)
	}
}

func TestFireFailsWhenTheServerIsUnreachable(t *testing.T) {
	trigger := SweepTrigger{Tokens: &presenceTokens{}, Environment: func(string) string { return `{"yon":{"server":{"port":1}}}` }, Client: http.DefaultClient}
	if err := trigger.Fire(); err == nil {
		t.Fatal("an unreachable server was not reported")
	}
}
