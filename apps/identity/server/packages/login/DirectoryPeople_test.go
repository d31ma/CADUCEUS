package login

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
