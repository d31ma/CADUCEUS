package keycloak

// tachyon: example secrets

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type fakeAnswer struct {
	status   int
	body     string
	location string
}

type recordedRequest struct {
	method        string
	uri           string
	authorization string
	contentType   string
	body          string
}

type fakeKeycloak struct {
	mutex    sync.Mutex
	server   *httptest.Server
	token    fakeAnswer
	tokens   int
	limit    int
	answers  map[string]fakeAnswer
	requests []recordedRequest
}

func newFakeKeycloak(t *testing.T) *fakeKeycloak {
	fake := &fakeKeycloak{token: fakeAnswer{status: http.StatusOK, body: `{"access_token":"example-token"}`}, answers: map[string]fakeAnswer{}}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.answer))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fakeKeycloak) on(method string, uri string, answer fakeAnswer) *fakeKeycloak {
	fake.answers[method+" "+uri] = answer
	return fake
}

func (fake *fakeKeycloak) answer(writer http.ResponseWriter, request *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	body, _ := io.ReadAll(request.Body)
	fake.requests = append(fake.requests, recordedRequest{method: request.Method, uri: request.RequestURI, authorization: request.Header.Get("Authorization"), contentType: request.Header.Get("Content-Type"), body: string(body)})
	answer := fake.chosen(request)
	if answer.location != "" {
		writer.Header().Set("Location", answer.location)
	}
	writer.WriteHeader(answer.status)
	io.WriteString(writer, answer.body)
}

func (fake *fakeKeycloak) chosen(request *http.Request) fakeAnswer {
	if request.RequestURI == "/idp/realms/mesail/protocol/openid-connect/token" {
		return fake.issued()
	}
	answer, found := fake.answers[request.Method+" "+request.RequestURI]
	if !found {
		return fakeAnswer{status: http.StatusTeapot}
	}
	return answer
}

func (fake *fakeKeycloak) recorded() []recordedRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]recordedRequest{}, fake.requests...)
}

func (fake *fakeKeycloak) adminRequests() []recordedRequest {
	admin := []recordedRequest{}
	for _, request := range fake.recorded() {
		if request.uri != "/idp/realms/mesail/protocol/openid-connect/token" {
			admin = append(admin, request)
		}
	}
	return admin
}

func (fake *fakeKeycloak) settings() UserSettings {
	return UserSettings{BaseUrl: fake.server.URL + "/idp/", ClientId: "mesail-directory", ClientSecret: "example-secret"}
}

func (fake *fakeKeycloak) users() Users {
	return NewUsers(fake.settings(), fake.server.Client())
}

func (fake *fakeKeycloak) issued() fakeAnswer {
	fake.tokens++
	if fake.limit > 0 && fake.tokens > fake.limit {
		return fakeAnswer{status: http.StatusUnauthorized}
	}
	return fake.token
}
