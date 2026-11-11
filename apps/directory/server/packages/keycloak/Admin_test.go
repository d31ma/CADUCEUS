package keycloak

// tachyon: example secrets

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type recordedRequest struct {
	method        string
	uri           string
	authorization string
	contentType   string
	body          string
}

type fakeKeycloak struct {
	mutex       sync.Mutex
	server      *httptest.Server
	tokenStatus int
	tokenBody   string
	adminStatus int
	adminBody   string
	discovery   int
	token       recordedRequest
	admin       recordedRequest
	found       recordedRequest
	requests    int
}

func newFakeKeycloak(t *testing.T, tokenStatus int, adminStatus int, adminBody string) *fakeKeycloak {
	fake := &fakeKeycloak{tokenStatus: tokenStatus, tokenBody: `{"access_token":"example-token"}`, adminStatus: adminStatus, adminBody: adminBody, discovery: http.StatusNotFound}
	mux := http.NewServeMux()
	mux.HandleFunc("/idp/realms/master/protocol/openid-connect/token", fake.answerToken)
	mux.HandleFunc("/idp/realms/", fake.answerDiscovery)
	mux.HandleFunc("/idp/admin/", fake.answerAdmin)
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fakeKeycloak) answerToken(writer http.ResponseWriter, request *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.requests++
	fake.token = record(request)
	writer.WriteHeader(fake.tokenStatus)
	io.WriteString(writer, fake.tokenBody)
}

func (fake *fakeKeycloak) answerAdmin(writer http.ResponseWriter, request *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.requests++
	fake.admin = record(request)
	writer.WriteHeader(fake.adminStatus)
	io.WriteString(writer, fake.adminBody)
}

func (fake *fakeKeycloak) answerDiscovery(writer http.ResponseWriter, request *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.requests++
	fake.found = record(request)
	writer.WriteHeader(fake.discovery)
	io.WriteString(writer, "{}")
}

func (fake *fakeKeycloak) discoveryRequest() recordedRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.found
}

func (fake *fakeKeycloak) snapshot() (recordedRequest, recordedRequest, int) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.token, fake.admin, fake.requests
}

func (fake *fakeKeycloak) settings() AdminSettings {
	return AdminSettings{BaseUrl: fake.server.URL + "/idp", ClientId: "mesail-directory", ClientSecret: "example-secret", Secrets: RealmSecrets{Identity: "example-identity", Service: "example-service", Account: "example-account"}}
}

func (fake *fakeKeycloak) client() Admin {
	return NewAdmin(fake.settings(), fake.server.Client())
}

func record(request *http.Request) recordedRequest {
	body, _ := io.ReadAll(request.Body)
	return recordedRequest{method: request.Method, uri: request.RequestURI, authorization: request.Header.Get("Authorization"), contentType: request.Header.Get("Content-Type"), body: string(body)}
}

func TestCreateRealmPostsTheFilledTemplate(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusCreated, "")
	err := fake.client().CreateRealm("example.com")
	_, admin, _ := fake.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if admin.method != http.MethodPost || admin.uri != "/idp/admin/realms" {
		t.Fatalf("got %s %s", admin.method, admin.uri)
	}
	if admin.contentType != "application/json" {
		t.Fatalf("got content type %q", admin.contentType)
	}
	if !strings.Contains(admin.body, `"realm": "example.com"`) {
		t.Fatalf("the body does not name the realm: %s", admin.body)
	}
}

func TestCreateRealmReportsAnExistingRealm(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusConflict, "")
	err := fake.client().CreateRealm("example.com")
	var exists RealmExists
	if !errors.As(err, &exists) || exists.Realm != "example.com" {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRealmReportsOtherStatusesAsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusInternalServerError, "")
	err := fake.client().CreateRealm("example.com")
	var refused AdminRefused
	if !errors.As(err, &refused) || refused.Status != http.StatusInternalServerError {
		t.Fatalf("got %v", err)
	}
}

func TestForbiddenCreateOfAnExistingRealmReportsAnExistingRealm(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusForbidden, "")
	fake.discovery = http.StatusOK
	err := fake.client().CreateRealm("example.com")
	found := fake.discoveryRequest()
	var exists RealmExists
	if !errors.As(err, &exists) || exists.Realm != "example.com" {
		t.Fatalf("got %v", err)
	}
	if found.uri != "/idp/realms/example.com/.well-known/openid-configuration" {
		t.Fatalf("got %q", found.uri)
	}
}

func TestForbiddenCreateOfAMissingRealmIsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusForbidden, "")
	err := fake.client().CreateRealm("example.com")
	var refused AdminRefused
	var exists RealmExists
	if !errors.As(err, &refused) || refused.Status != http.StatusForbidden || errors.As(err, &exists) {
		t.Fatalf("got %v", err)
	}
}

func TestForbiddenCreateWithAFailedDiscoveryIsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusForbidden, "")
	fake.discovery = http.StatusInternalServerError
	err := fake.client().CreateRealm("example.com")
	var refused AdminRefused
	if !errors.As(err, &refused) || refused.Status != http.StatusForbidden || !strings.Contains(err.Error(), "500") {
		t.Fatalf("got %v", err)
	}
}

func TestRefusedTokenStopsBeforeTheAdminCall(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusUnauthorized, http.StatusCreated, "")
	err := fake.client().CreateRealm("example.com")
	_, _, requests := fake.snapshot()
	var refused AdminRefused
	if !errors.As(err, &refused) || refused.Status != http.StatusUnauthorized {
		t.Fatalf("got %v", err)
	}
	if requests != 1 {
		t.Fatalf("got %d requests", requests)
	}
}

func TestEmptyAccessTokenIsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusCreated, "")
	fake.tokenBody = `{"access_token":""}`
	err := fake.client().CreateRealm("example.com")
	var refused AdminRefused
	if !errors.As(err, &refused) {
		t.Fatalf("got %v", err)
	}
}

func TestTokenAnswerThatIsNotJsonFails(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusCreated, "")
	fake.tokenBody = `not json`
	err := fake.client().CreateRealm("example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestMissingSettingsMakeNoRequest(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusCreated, "")
	settings := fake.settings()
	settings.ClientSecret = ""
	err := NewAdmin(settings, fake.server.Client()).CreateRealm("example.com")
	_, _, requests := fake.snapshot()
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
	if requests != 0 {
		t.Fatalf("got %d requests", requests)
	}
}

func TestMissingBaseUrlFailsEveryCall(t *testing.T) {
	admin := NewAdmin(AdminSettings{ClientId: "mesail-directory", ClientSecret: "example-secret"}, http.DefaultClient)
	_, existsError := admin.RealmExists("example.com")
	deleteError := admin.DeleteRealm("example.com")
	if !errors.As(existsError, &NotConfigured{}) || !errors.As(deleteError, &NotConfigured{}) {
		t.Fatalf("got %v, %v", existsError, deleteError)
	}
}

func TestTokenRequestIsFormEncodedWithClientCredentials(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusNoContent, "")
	err := fake.client().DeleteRealm("example.com")
	token, _, _ := fake.snapshot()
	form, parseError := url.ParseQuery(token.body)
	if err != nil || parseError != nil {
		t.Fatalf("got %v, %v", err, parseError)
	}
	if token.method != http.MethodPost || token.contentType != "application/x-www-form-urlencoded" {
		t.Fatalf("got %s with %q", token.method, token.contentType)
	}
	if form.Get("grant_type") != "client_credentials" || form.Get("client_id") != "mesail-directory" || form.Get("client_secret") != "example-secret" {
		t.Fatalf("got form %v", form)
	}
}

func TestAdminCallCarriesTheBearerToken(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusNoContent, "")
	err := fake.client().DeleteRealm("example.com")
	_, admin, _ := fake.snapshot()
	if err != nil || admin.authorization != "Bearer example-token" {
		t.Fatalf("got %q, %v", admin.authorization, err)
	}
}

func TestRealmExistsWhenDiscoveryFindsIt(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	fake.discovery = http.StatusOK
	exists, err := fake.client().RealmExists("example.com")
	found := fake.discoveryRequest()
	_, _, requests := fake.snapshot()
	if err != nil || !exists {
		t.Fatalf("got %v, %v", exists, err)
	}
	if found.method != http.MethodGet || found.uri != "/idp/realms/example.com/.well-known/openid-configuration" {
		t.Fatalf("got %s %s", found.method, found.uri)
	}
	if requests != 1 {
		t.Fatalf("got %d requests", requests)
	}
}

func TestDiscoveryCarriesNoAuthorization(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	fake.discovery = http.StatusOK
	_, err := fake.client().RealmExists("example.com")
	found := fake.discoveryRequest()
	if err != nil || found.authorization != "" {
		t.Fatalf("got %q, %v", found.authorization, err)
	}
}

func TestRealmExistsNeedsOnlyTheBaseUrl(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	fake.discovery = http.StatusOK
	exists, err := NewAdmin(AdminSettings{BaseUrl: fake.server.URL + "/idp"}, fake.server.Client()).RealmExists("example.com")
	if err != nil || !exists {
		t.Fatalf("got %v, %v", exists, err)
	}
}

func TestRealmDoesNotExistWhenDiscoveryAnswersNotFound(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	exists, err := fake.client().RealmExists("example.com")
	if err != nil || exists {
		t.Fatalf("got %v, %v", exists, err)
	}
}

func TestRealmExistsReportsOtherStatusesAsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	fake.discovery = http.StatusInternalServerError
	_, err := fake.client().RealmExists("example.com")
	var refused AdminRefused
	if !errors.As(err, &refused) || refused.Status != http.StatusInternalServerError {
		t.Fatalf("got %v", err)
	}
}

func TestRealmNameIsEscapedInTheDiscoveryPath(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	_, err := fake.client().RealmExists("a b/c?d")
	found := fake.discoveryRequest()
	if err != nil || found.uri != "/idp/realms/a%20b%2Fc%3Fd/.well-known/openid-configuration" {
		t.Fatalf("got %q, %v", found.uri, err)
	}
}

func TestRealmNameIsEscapedInTheAdminPath(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusNoContent, "")
	err := fake.client().DeleteRealm("a b/c?d")
	_, admin, _ := fake.snapshot()
	if err != nil || admin.uri != "/idp/admin/realms/a%20b%2Fc%3Fd" {
		t.Fatalf("got %q, %v", admin.uri, err)
	}
}

func TestDeleteRealmSucceedsWithNoContent(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusNoContent, "")
	err := fake.client().DeleteRealm("example.com")
	_, admin, _ := fake.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if admin.method != http.MethodDelete || admin.uri != "/idp/admin/realms/example.com" {
		t.Fatalf("got %s %s", admin.method, admin.uri)
	}
}

func TestDeleteRealmTreatsNotFoundAsGone(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusNotFound, "")
	err := fake.client().DeleteRealm("example.com")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRealmReportsOtherStatusesAsRefused(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusInternalServerError, "")
	err := fake.client().DeleteRealm("example.com")
	var refused AdminRefused
	if !errors.As(err, &refused) || refused.Status != http.StatusInternalServerError {
		t.Fatalf("got %v", err)
	}
}

func TestForbiddenDeleteIsARealmTheDirectoryDoesNotManage(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusForbidden, "")
	err := fake.client().DeleteRealm("example.com")
	var notManaged RealmNotManaged
	if !errors.As(err, &notManaged) || notManaged.Realm != "example.com" {
		t.Fatalf("got %v", err)
	}
}

func TestUnreachableKeycloakWrapsTheTransportErrorWithoutTheSecret(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	admin := fake.client()
	fake.server.Close()
	err := admin.DeleteRealm("example.com")
	if err == nil || errors.Unwrap(err) == nil {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "example-secret") {
		t.Fatalf("the error shows the secret: %v", err)
	}
}

func TestInvalidBaseUrlFails(t *testing.T) {
	admin := NewAdmin(AdminSettings{BaseUrl: "http://bad host", ClientId: "mesail-directory", ClientSecret: "example-secret"}, http.DefaultClient)
	err := admin.DeleteRealm("example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestInvalidBaseUrlFailsTheDiscovery(t *testing.T) {
	admin := NewAdmin(AdminSettings{BaseUrl: "http://bad host"}, http.DefaultClient)
	_, err := admin.RealmExists("example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestUnreachableKeycloakFailsTheDiscovery(t *testing.T) {
	fake := newFakeKeycloak(t, http.StatusOK, http.StatusOK, "")
	admin := fake.client()
	fake.server.Close()
	_, err := admin.RealmExists("example.com")
	if err == nil || errors.Unwrap(err) == nil {
		t.Fatalf("got %v", err)
	}
}

func TestErrorMessagesNameTheProblem(t *testing.T) {
	exists := RealmExists{Realm: "example.com"}.Error()
	refused := AdminRefused{Status: http.StatusForbidden, Action: "delete the realm"}.Error()
	missing := NotConfigured{}.Error()
	notManaged := RealmNotManaged{Realm: "example.com"}.Error()
	if !strings.Contains(exists, "example.com") || !strings.Contains(refused, "403") || missing == "" {
		t.Fatalf("got %q, %q, %q", exists, refused, missing)
	}
	if !strings.Contains(notManaged, "example.com") || !strings.Contains(notManaged, "cannot delete") {
		t.Fatalf("got %q", notManaged)
	}
}
