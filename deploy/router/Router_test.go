package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAws struct {
	server      *httptest.Server
	invocations []HttpEvent
	functions   []string
	objects     map[string]string
	answer      string
	failure     string
	status      int
	storageCode int
}

func newFakeAws(t *testing.T) *fakeAws {
	t.Helper()
	fake := &fakeAws{objects: map[string]string{}, status: http.StatusOK}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fakeAws) serve(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test/") {
		writer.WriteHeader(http.StatusForbidden)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/2015-03-31/functions/") {
		fake.invoke(writer, request)
		return
	}
	fake.object(writer, request)
}

func (fake *fakeAws) invoke(writer http.ResponseWriter, request *http.Request) {
	var event HttpEvent
	body, _ := io.ReadAll(request.Body)
	_ = json.Unmarshal(body, &event)
	fake.invocations = append(fake.invocations, event)
	fake.functions = append(fake.functions, strings.Split(request.URL.Path, "/")[3])
	if fake.failure != "" {
		writer.Header().Set("X-Amz-Function-Error", fake.failure)
	}
	writer.WriteHeader(fake.status)
	_, _ = writer.Write([]byte(fake.answer))
}

func (fake *fakeAws) object(writer http.ResponseWriter, request *http.Request) {
	if fake.storageCode != 0 {
		writer.WriteHeader(fake.storageCode)
		return
	}
	content, found := fake.objects[strings.TrimPrefix(request.URL.Path, "/mesail-static/")]
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "text/html")
	writer.Header().Set("Etag", `"abc"`)
	_, _ = writer.Write([]byte(content))
}

func settingsFor(fake *fakeAws, keycloak string) Settings {
	return Settings{
		Listen:         ":0",
		Services:       []string{"identity", "mail-in"},
		FunctionPrefix: "mesail-",
		LambdaEndpoint: fake.server.URL,
		S3Endpoint:     fake.server.URL,
		StaticBucket:   "mesail-static",
		KeycloakUrl:    keycloak,
		Credentials:    Credentials{AccessKey: "test", SecretKey: "test", Region: "us-east-1"},
		Timeout:        5 * time.Second,
		BodyLimit:      16,
	}
}

func call(t *testing.T, handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Cookie", "a=1; b=2")
	request.Header.Set("Connection", "close")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestReadyAnswersWithoutBackend(t *testing.T) {
	fake := newFakeAws(t)
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodGet, "/_router/ready", "")
	if reply.Code != http.StatusOK || reply.Body.String() != `{"status":"ok"}` || len(fake.functions) != 0 {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestApiCallInvokesTheServiceFunctionWithAVersionTwoEvent(t *testing.T) {
	fake := newFakeAws(t)
	fake.answer = `{"statusCode":201,"headers":{"content-type":"application/json"},"multiValueHeaders":{"x-list":["a","b"]},"cookies":["session=1"],"body":"eyJvayI6dHJ1ZX0=","isBase64Encoded":true}`
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodPost, "/api/v1/mail-in/messages?folder=inbox&folder=sent", "hello")
	event := fake.invocations[0]
	body, _ := base64.StdEncoding.DecodeString(event.Body)
	checks := map[string]bool{
		"status":          reply.Code == http.StatusCreated,
		"body":            reply.Body.String() == `{"ok":true}`,
		"content type":    reply.Header().Get("Content-Type") == "application/json",
		"multiple values": strings.Join(reply.Header().Values("X-List"), ",") == "a,b",
		"cookie":          reply.Header().Get("Set-Cookie") == "session=1",
		"function":        fake.functions[0] == "mesail-mail-in",
		"version":         event.Version == "2.0" && event.RouteKey == "$default",
		"path":            event.RawPath == "/api/v1/mail-in/messages" && event.RequestContext.Http.Method == http.MethodPost,
		"query":           event.QueryStringParameters["folder"] == "inbox,sent" && event.RawQueryString == "folder=inbox&folder=sent",
		"event body":      string(body) == "hello" && event.IsBase64Encoded,
		"cookies":         strings.Join(event.Cookies, "|") == "a=1|b=2",
		"host":            event.Headers["host"] == "localhost" && event.Headers["x-forwarded-host"] == "example.com",
		"hop by hop":      event.Headers["connection"] == "" && event.Headers["cookie"] == "",
		"source":          event.RequestContext.Http.SourceIp == "192.0.2.1" && len(event.RequestContext.RequestId) == 32,
	}
	for name, passed := range checks {
		if !passed {
			t.Errorf("%s check failed: reply %d %s, event %+v", name, reply.Code, reply.Body.String(), event)
		}
	}
}

func TestPlainPayloadBecomesAJsonReply(t *testing.T) {
	fake := newFakeAws(t)
	fake.answer = `{"service":"identity","status":"ok"}`
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodGet, "/api/v1/identity/health", "")
	if reply.Code != http.StatusOK || reply.Body.String() != fake.answer || reply.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestPlainTextBodyIsKept(t *testing.T) {
	fake := newFakeAws(t)
	fake.answer = `{"statusCode":200,"body":"plain"}`
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodGet, "/api/v1/identity/health", "")
	if reply.Code != http.StatusOK || reply.Body.String() != "plain" {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestApiProblems(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		body    string
		failure string
		status  int
		answer  string
		want    int
		code    string
	}{
		{name: "unknown service", target: "/api/v1/billing/x", want: http.StatusNotFound, code: "unknown-service"},
		{name: "empty service", target: "/api/v1/", want: http.StatusNotFound, code: "unknown-service"},
		{name: "body too large", target: "/api/v1/identity/x", body: strings.Repeat("a", 17), want: http.StatusRequestEntityTooLarge, code: "body-too-large"},
		{name: "function error", target: "/api/v1/identity/x", failure: "Unhandled", status: http.StatusOK, want: http.StatusBadGateway, code: "function-error"},
		{name: "invoke refused", target: "/api/v1/identity/x", status: http.StatusTooManyRequests, want: http.StatusBadGateway, code: "function-error"},
		{name: "broken result", target: "/api/v1/identity/x", status: http.StatusOK, answer: `{"statusCode":"x"}`, want: http.StatusBadGateway, code: "function-error"},
		{name: "broken base64", target: "/api/v1/identity/x", status: http.StatusOK, answer: `{"statusCode":200,"body":"%%%","isBase64Encoded":true}`, want: http.StatusBadGateway, code: "function-error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := newFakeAws(t)
			fake.failure = testCase.failure
			fake.status = testCase.status
			fake.answer = testCase.answer
			reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodPost, testCase.target, testCase.body)
			var problem Problem
			_ = json.Unmarshal(reply.Body.Bytes(), &problem)
			if reply.Code != testCase.want || problem.Code != testCase.code {
				t.Fatalf("got %d %s", reply.Code, reply.Body.String())
			}
		})
	}
}

func TestUnreachableLambdaIsABadGateway(t *testing.T) {
	fake := newFakeAws(t)
	settings := settingsFor(fake, "")
	settings.LambdaEndpoint = "http://127.0.0.1:1"
	reply := call(t, Assembly{}.Handler(settings), http.MethodGet, "/api/v1/identity/health", "")
	if reply.Code != http.StatusBadGateway {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestStaticFiles(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		accept string
		want   int
		body   string
	}{
		{name: "web index", method: http.MethodGet, target: "/", want: http.StatusOK, body: "web home"},
		{name: "web asset", method: http.MethodGet, target: "/app.js", want: http.StatusOK, body: "web script"},
		{name: "web route falls back", method: http.MethodGet, target: "/conversations/42", want: http.StatusOK, body: "web home"},
		{name: "admin bare", method: http.MethodGet, target: "/admin", want: http.StatusOK, body: "admin home"},
		{name: "admin index", method: http.MethodGet, target: "/admin/", want: http.StatusOK, body: "admin home"},
		{name: "admin route falls back", method: http.MethodGet, target: "/admin/domains/example.com/", want: http.StatusOK, body: "admin home"},
		{name: "page with a dot falls back for browsers", method: http.MethodGet, target: "/admin/domains/example.com", accept: "text/html,*/*", want: http.StatusOK, body: "admin home"},
		{name: "admin asset", method: http.MethodGet, target: "/admin/admin.css", want: http.StatusOK, body: "admin styles"},
		{name: "missing asset", method: http.MethodGet, target: "/missing.js", want: http.StatusNotFound},
		{name: "head", method: http.MethodHead, target: "/app.js", want: http.StatusOK},
		{name: "post refused", method: http.MethodPost, target: "/", want: http.StatusMethodNotAllowed},
		{name: "parent folder refused", method: http.MethodGet, target: "/a/..%2f..%2fsecret", want: http.StatusBadRequest},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := newFakeAws(t)
			fake.objects["web/index.html"] = "web home"
			fake.objects["web/app.js"] = "web script"
			fake.objects["admin/index.html"] = "admin home"
			fake.objects["admin/admin.css"] = "admin styles"
			request := httptest.NewRequest(testCase.method, testCase.target, nil)
			request.Header.Set("Accept", testCase.accept)
			reply := httptest.NewRecorder()
			Assembly{}.Handler(settingsFor(fake, "")).ServeHTTP(reply, request)
			if reply.Code != testCase.want || !strings.HasPrefix(reply.Body.String(), testCase.body) {
				t.Fatalf("got %d %s", reply.Code, reply.Body.String())
			}
			if reply.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(reply.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatalf("security headers missing: %v", reply.Header())
			}
		})
	}
}

func TestStaticStorageFailureIsABadGateway(t *testing.T) {
	fake := newFakeAws(t)
	fake.storageCode = http.StatusInternalServerError
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodGet, "/", "")
	if reply.Code != http.StatusBadGateway {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestStaticStorageUnreachable(t *testing.T) {
	fake := newFakeAws(t)
	settings := settingsFor(fake, "")
	settings.S3Endpoint = "http://127.0.0.1:1"
	reply := call(t, Assembly{}.Handler(settings), http.MethodGet, "/", "")
	if reply.Code != http.StatusBadGateway {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestKeycloakPathsAreProxiedWithTheOriginalHost(t *testing.T) {
	var seen *http.Request
	keycloak := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = request
		_, _ = writer.Write([]byte("realm"))
	}))
	defer keycloak.Close()
	fake := newFakeAws(t)
	reply := call(t, Assembly{}.Handler(settingsFor(fake, keycloak.URL)), http.MethodGet, "/realms/example.com/.well-known/openid-configuration", "")
	if reply.Code != http.StatusOK || reply.Body.String() != "realm" || seen.Host != "example.com" || seen.Header.Get("X-Forwarded-Host") != "example.com" || seen.Header.Get("X-Forwarded-Proto") != "http" {
		t.Fatalf("got %d %s, keycloak saw %+v", reply.Code, reply.Body.String(), seen)
	}
}

func TestKeycloakPathsAreMissingWithoutKeycloak(t *testing.T) {
	fake := newFakeAws(t)
	reply := call(t, Assembly{}.Handler(settingsFor(fake, "")), http.MethodGet, "/resources/x", "")
	if reply.Code != http.StatusNotFound {
		t.Fatalf("got %d", reply.Code)
	}
}

func TestSignatureMatchesTheAwsTestSuite(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://example.amazonaws.com/", nil)
	request.Header = http.Header{}
	signer := Signer{Credentials: Credentials{AccessKey: "AKIDEXAMPLE", SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", Region: "us-east-1"}, Service: "service"}
	signer.Sign(request, nil, time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31"
	if request.Header.Get("Authorization") != want {
		t.Fatalf("got %s", request.Header.Get("Authorization"))
	}
}

func TestSignatureForStorageAddsContentHashAndSessionToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://floci:4566/bucket/key?b=2&a=1", nil)
	request.Header = http.Header{}
	signer := Signer{Credentials: Credentials{AccessKey: "test", SecretKey: "test", SessionToken: "token", Region: "us-east-1"}, Service: "s3"}
	signer.Sign(request, nil, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	authorization := request.Header.Get("Authorization")
	if !strings.Contains(authorization, "SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token") || request.Header.Get("X-Amz-Security-Token") != "token" {
		t.Fatalf("got %s", authorization)
	}
}

func TestSettings(t *testing.T) {
	complete := map[string]string{
		"MESAIL_ROUTER_SERVICES":        "identity,mail-in",
		"MESAIL_ROUTER_LAMBDA_ENDPOINT": "http://floci:4566/",
		"AWS_ACCESS_KEY_ID":             "test",
		"AWS_SECRET_ACCESS_KEY":         "test",
	}
	settings, err := SettingsReader{Lookup: lookupIn(complete)}.Read()
	if err != nil || settings.Listen != ":8080" || settings.S3Endpoint != "http://floci:4566" || settings.Timeout != 70*time.Second || settings.Credentials.Region != "us-east-1" || len(settings.Services) != 2 {
		t.Fatalf("got %+v, %v", settings, err)
	}
	cases := []struct {
		name   string
		change map[string]string
		want   string
	}{
		{name: "no services", change: map[string]string{"MESAIL_ROUTER_SERVICES": ""}, want: "MESAIL_ROUTER_SERVICES is required"},
		{name: "bad service", change: map[string]string{"MESAIL_ROUTER_SERVICES": "identity,Bad"}, want: `bad service name "Bad"`},
		{name: "no endpoint", change: map[string]string{"MESAIL_ROUTER_LAMBDA_ENDPOINT": ""}, want: "MESAIL_ROUTER_LAMBDA_ENDPOINT is required"},
		{name: "no access key", change: map[string]string{"AWS_ACCESS_KEY_ID": ""}, want: "AWS_ACCESS_KEY_ID is required"},
		{name: "no secret", change: map[string]string{"AWS_SECRET_ACCESS_KEY": ""}, want: "AWS_SECRET_ACCESS_KEY is required"},
		{name: "bad timeout", change: map[string]string{"MESAIL_ROUTER_TIMEOUT_SECONDS": "soon"}, want: "whole number of seconds"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			values := map[string]string{}
			for name, value := range complete {
				values[name] = value
			}
			for name, value := range testCase.change {
				values[name] = value
			}
			_, err := SettingsReader{Lookup: lookupIn(values)}.Read()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func lookupIn(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}
}

func TestProgramStopsWithAnErrorWhenSettingsAreMissing(t *testing.T) {
	t.Setenv("MESAIL_ROUTER_SERVICES", "")
	if (Program{}).Run() != 1 {
		t.Fatal("want exit code 1")
	}
}

func TestProgramServesUntilStopped(t *testing.T) {
	fake := newFakeAws(t)
	t.Setenv("MESAIL_ROUTER_SERVICES", "identity")
	t.Setenv("MESAIL_ROUTER_LAMBDA_ENDPOINT", fake.server.URL)
	t.Setenv("MESAIL_ROUTER_LISTEN", "127.0.0.1:0")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	server := &http.Server{Addr: "127.0.0.1:0", Handler: ReadyHandler{}}
	failures := make(chan error, 1)
	go Program{}.listen(server, failures)
	time.Sleep(100 * time.Millisecond)
	if err := (Program{}).shutdown(server); err != nil {
		t.Fatal(err)
	}
	if err := <-failures; err != nil {
		t.Fatal(err)
	}
}

func TestProgramReportsAListenFailure(t *testing.T) {
	t.Setenv("MESAIL_ROUTER_SERVICES", "identity")
	t.Setenv("MESAIL_ROUTER_LAMBDA_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("MESAIL_ROUTER_LISTEN", "256.0.0.1:99999")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	if (Program{}).Run() != 1 {
		t.Fatal("want exit code 1")
	}
}
