package realtime

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type fixedServiceTokens struct {
	err error
}

func (tokens fixedServiceTokens) Authorization(domainId string) (string, error) {
	return "Bearer service-" + domainId, tokens.err
}

func serviceServer(t *testing.T, status int, answer string, seen *[]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*seen = append(*seen, request.Method+" "+request.URL.Path+" "+request.Header.Get("Authorization")+" "+string(body))
		writer.WriteHeader(status)
		io.WriteString(writer, answer)
	}))
	t.Cleanup(server.Close)
	return server
}

func serviceCallTo(server *httptest.Server, tokens ServiceTokenSource) ServiceCall {
	return ServiceCall{BaseUrl: server.URL + "/", Tokens: tokens, Client: server.Client()}
}

func TestTheDirectoryClientAsksForOwnersWithAServiceToken(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusOK, `{"owners":{"a1":"ana"}}`, &seen)
	owners, err := DirectoryClient{Call: serviceCallTo(server, fixedServiceTokens{})}.Owners("example.com", []string{"a1", "x"})
	want := []string{`POST /api/v1/directory/devices/owners Bearer service-example.com {"domainId":"example.com","deviceIds":["a1","x"]}`}
	if err != nil || !reflect.DeepEqual(owners, map[string]string{"a1": "ana"}) || !reflect.DeepEqual(seen, want) {
		t.Fatalf("got %+v, %v, %v", owners, err, seen)
	}
}

func TestTheDirectoryClientGivesAnEmptyMapForNoOwners(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusOK, `{}`, &seen)
	owners, err := DirectoryClient{Call: serviceCallTo(server, fixedServiceTokens{})}.Owners("example.com", []string{"a1"})
	if err != nil || owners == nil || len(owners) != 0 {
		t.Fatalf("got %+v, %v", owners, err)
	}
}

func TestAServiceCallFailsOnAnErrorStatus(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusNotFound, `{}`, &seen)
	_, err := DirectoryClient{Call: serviceCallTo(server, fixedServiceTokens{})}.Owners("example.com", []string{"a1"})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAServiceCallFailsOnAnUnreadableAnswer(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusOK, `not json`, &seen)
	_, err := DirectoryClient{Call: serviceCallTo(server, fixedServiceTokens{})}.Owners("example.com", []string{"a1"})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAServiceCallFailsWithoutAToken(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusOK, `{}`, &seen)
	err := NotifyClient{Call: serviceCallTo(server, fixedServiceTokens{err: errors.New("no secret")})}.Send("example.com", CallPush{ToPersonIds: []string{"cara"}, Kind: "missed"})
	if !errors.As(err, &Unavailable{}) || len(seen) != 0 {
		t.Fatalf("got %v, %v", err, seen)
	}
}

func TestAServiceCallFailsWithNoTokenSource(t *testing.T) {
	err := ServiceCall{BaseUrl: "http://127.0.0.1:1", Client: http.DefaultClient}.Post("example.com", "/x", map[string]string{}, nil)
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAServiceCallFailsWhenTheServerIsDown(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusOK, `{}`, &seen)
	call := serviceCallTo(server, fixedServiceTokens{})
	server.Close()
	if err := call.Post("example.com", "/x", map[string]string{}, nil); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestTheNotifyClientPostsTheCallPush(t *testing.T) {
	seen := []string{}
	server := serviceServer(t, http.StatusNoContent, ``, &seen)
	err := NotifyClient{Call: serviceCallTo(server, fixedServiceTokens{})}.Send("example.com", CallPush{ToPersonIds: []string{"cara"}, Kind: "incoming"})
	want := []string{`POST /api/v1/notify/call-pushes Bearer service-example.com {"toPersonIds":["cara"],"kind":"incoming"}`}
	if err != nil || !reflect.DeepEqual(seen, want) {
		t.Fatalf("got %v, %v", err, seen)
	}
}
