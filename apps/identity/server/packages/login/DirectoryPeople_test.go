package login

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

type directoryServer struct {
	server        *httptest.Server
	authorization string
	path          string
}

func newDirectoryServer(t *testing.T, status int, body string) *directoryServer {
	t.Helper()
	directory := &directoryServer{}
	directory.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		directory.authorization = request.Header.Get("Authorization")
		directory.path = request.URL.Path
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(directory.server.Close)
	return directory
}

func (directory *directoryServer) people(t *testing.T) DirectoryPeople {
	t.Helper()
	return DirectoryPeople{Client: directory.server.Client(), BaseUrl: directory.server.URL, Signer: testSigner(t)}
}

func TestDirectoryPeopleDecodesThePerson(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{"personId":"person-1","username":"ana","displayName":"Ana Example","domainIds":["example.com"],"adminDomainIds":["example.com"],"defaultDomainId":"example.com","state":"active"}`)
	person, err := directory.people(t).Person("person-1")
	failOn(t, err)
	same(t, "person", person, Person{PersonId: "person-1", Username: "ana", DisplayName: "Ana Example", DomainIds: []string{"example.com"}, AdminDomainIds: []string{"example.com"}, DefaultDomainId: "example.com", State: "active"})
	same(t, "path", directory.path, "/api/v1/directory/people/person-1")
}

func TestDirectoryPeopleSendsAnInstallWideServiceToken(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{"personId":"person-1"}`)
	_, err := directory.people(t).Person("person-1")
	failOn(t, err)
	token, bearer := strings.CutPrefix(directory.authorization, "Bearer ")
	claims := signedClaims(t, token)
	same(t, "bearer", bearer, true)
	same(t, "domain", claims.Domain, "*")
	same(t, "party", claims.AuthorizedParty, "mesail-service")
}

func TestDirectoryPeopleGivesAnUnknownPersonNoDomains(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusNotFound, `{"error":"not found"}`)
	person, err := directory.people(t).Person("person-9")
	failOn(t, err)
	same(t, "person", person, Person{PersonId: "person-9"})
}

func TestDirectoryPeopleReportsAServerFailure(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusInternalServerError, `{"personId":"person-1"}`)
	_, err := directory.people(t).Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestDirectoryPeopleReportsABrokenBody(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{"personId":`)
	_, err := directory.people(t).Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestDirectoryPeopleReportsAClosedServer(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{}`)
	people := directory.people(t)
	directory.server.Close()
	_, err := people.Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestDirectoryPeopleReportsASignerFailure(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{}`)
	people := directory.people(t)
	people.Signer = brokenSigner()
	_, err := people.Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "path", directory.path, "")
}

func TestDirectoryPeopleReportsABadBaseUrl(t *testing.T) {
	people := DirectoryPeople{Client: http.DefaultClient, BaseUrl: "http://bad\x7fhost", Signer: testSigner(t)}
	_, err := people.Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

type flakyDirectory struct {
	server  *httptest.Server
	answers []int
	calls   atomic.Int32
	times   []time.Time
	mutex   sync.Mutex
}

func newFlakyDirectory(t *testing.T, answers ...int) *flakyDirectory {
	t.Helper()
	directory := &flakyDirectory{answers: answers}
	directory.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call := int(directory.calls.Add(1)) - 1
		directory.mutex.Lock()
		directory.times = append(directory.times, time.Now())
		directory.mutex.Unlock()
		status := directory.answers[min(call, len(directory.answers)-1)]
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(`{"personId":"person-1","state":"active"}`))
	}))
	t.Cleanup(directory.server.Close)
	return directory
}

func (directory *flakyDirectory) people(t *testing.T) DirectoryPeople {
	t.Helper()
	return DirectoryPeople{Client: directory.server.Client(), BaseUrl: directory.server.URL, Signer: testSigner(t)}
}

func TestDirectoryPeopleTriesAgainOnceAfterABadGateway(t *testing.T) {
	directory := newFlakyDirectory(t, http.StatusBadGateway, http.StatusOK)
	person, err := directory.people(t).Person("person-1")
	failOn(t, err)
	same(t, "person", person.PersonId, "person-1")
	same(t, "calls", directory.calls.Load(), int32(2))
	if pause := directory.times[1].Sub(directory.times[0]); pause < 200*time.Millisecond {
		t.Fatalf("the second try came after %s, want at least 200ms", pause)
	}
}

func TestDirectoryPeopleTriesAgainOnceAfterAnUnavailableDirectory(t *testing.T) {
	directory := newFlakyDirectory(t, http.StatusServiceUnavailable, http.StatusOK)
	_, err := directory.people(t).Person("person-1")
	failOn(t, err)
	same(t, "calls", directory.calls.Load(), int32(2))
}

func TestDirectoryPeopleGivesUpAfterTheSecondTry(t *testing.T) {
	directory := newFlakyDirectory(t, http.StatusBadGateway, http.StatusBadGateway, http.StatusOK)
	_, err := directory.people(t).Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "calls", directory.calls.Load(), int32(2))
}

func TestDirectoryPeopleDoesNotTryAgainAfterAServerError(t *testing.T) {
	directory := newFlakyDirectory(t, http.StatusInternalServerError, http.StatusOK)
	_, err := directory.people(t).Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "calls", directory.calls.Load(), int32(1))
}

func TestDirectoryPeopleTriesAgainAfterATimeout(t *testing.T) {
	directory := newFlakyDirectory(t, http.StatusOK)
	var calls atomic.Int32
	slow := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			time.Sleep(300 * time.Millisecond)
		}
		directory.server.Config.Handler.ServeHTTP(writer, request)
	})
	server := httptest.NewServer(slow)
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 100 * time.Millisecond
	person, err := DirectoryPeople{Client: client, BaseUrl: server.URL, Signer: testSigner(t)}.Person("person-1")
	failOn(t, err)
	same(t, "person", person.PersonId, "person-1")
	same(t, "calls", calls.Load(), int32(2))
}

type deadlineRecorder struct {
	deadlines []time.Time
}

func (recorder *deadlineRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	deadline, _ := request.Context().Deadline()
	recorder.deadlines = append(recorder.deadlines, deadline)
	return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
}

func TestDirectoryPeopleKeepsBothTriesUnderTwelveSeconds(t *testing.T) {
	recorder := &deadlineRecorder{}
	started := time.Now()
	_, err := DirectoryPeople{Client: &http.Client{Transport: recorder}, BaseUrl: "http://directory.test", Signer: testSigner(t)}.Person("person-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "tries", len(recorder.deadlines), 2)
	if recorder.deadlines[0].IsZero() || recorder.deadlines[1] != recorder.deadlines[0] || recorder.deadlines[1].Sub(started) >= 12*time.Second {
		t.Fatalf("deadlines %v, started %v", recorder.deadlines, started)
	}
}
