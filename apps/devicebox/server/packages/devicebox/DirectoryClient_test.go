package devicebox

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fixedTokens struct {
	failure error
}

func (tokens fixedTokens) Authorization(domainId string) (string, error) {
	return "Bearer service-for-" + domainId, tokens.failure
}

func TestTheDirectoryClientAsksForTheOwnersOfTheDevicesWithAServiceToken(t *testing.T) {
	seen := deviceOwnersQuery{}
	authorization := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Method + " " + request.URL.Path + " " + request.Header.Get("Authorization")
		json.NewDecoder(request.Body).Decode(&seen)
		writer.Write([]byte(`{"owners":{"b1":"bo"}}`))
	}))
	defer server.Close()
	owners, err := NewDirectoryClient(server.URL+"/", fixedTokens{}, server.Client()).Owners("example.com", []string{"b1", "x9"})
	if err != nil || owners["b1"] != "bo" || len(owners) != 1 {
		t.Fatalf("got %v %v", owners, err)
	}
	if authorization != "POST /api/v1/directory/devices/owners Bearer service-for-example.com" || seen.DomainId != "example.com" || len(seen.DeviceIds) != 2 {
		t.Fatalf("got %q %+v", authorization, seen)
	}
}

func TestTheDirectoryClientFailsClosed(t *testing.T) {
	answers := map[string]func(http.ResponseWriter){
		"refused": func(writer http.ResponseWriter) { writer.WriteHeader(http.StatusNotFound) },
		"garbage": func(writer http.ResponseWriter) { writer.Write([]byte(`not json`)) },
		"empty":   func(writer http.ResponseWriter) { writer.Write([]byte(`{}`)) },
	}
	numbers := map[string]int{"refused": 6019, "garbage": 6031, "empty": 6031}
	for name, answer := range answers {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { answer(writer) }))
		_, err := NewDirectoryClient(server.URL, fixedTokens{}, server.Client()).Owners("example.com", []string{"b1"})
		server.Close()
		if numberOf(err) != numbers[name] {
			t.Fatalf("%s got %v", name, err)
		}
	}
	if _, err := NewDirectoryClient("http://127.0.0.1:1", fixedTokens{failure: errors.New("no token")}, http.DefaultClient).Owners("example.com", nil); numberOf(err) != 6019 {
		t.Fatalf("got %v", err)
	}
	if _, err := NewDirectoryClient("http://127.0.0.1:1", fixedTokens{}, http.DefaultClient).Owners("example.com", nil); numberOf(err) != 6019 {
		t.Fatalf("got %v", err)
	}
	if _, err := NewDirectoryClient("::not a url", fixedTokens{}, http.DefaultClient).Owners("example.com", nil); numberOf(err) != 6019 {
		t.Fatalf("got %v", err)
	}
}
