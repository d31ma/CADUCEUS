package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"mesail.dev/packages/apierrors"
)

type numberedCase struct {
	name    string
	problem Problem
	status  int
	code    string
	message string
}

func numberedCases() []numberedCase {
	return []numberedCase{
		{name: "unexpected", problem: Problems{}.Unexpected(), status: http.StatusBadGateway, code: "function-error", message: "[MS5500]: Something went wrong."},
		{name: "function failed", problem: Problems{}.FunctionFailed(), status: http.StatusBadGateway, code: "function-error", message: "[MS5501]: The service failed to answer. Try again later."},
		{name: "function busy", problem: Problems{}.FunctionBusy(), status: http.StatusTooManyRequests, code: "function-busy", message: "[MS5502]: The service is busy. Try again later."},
		{name: "function throttled", problem: Problems{}.FunctionThrottled(), status: http.StatusTooManyRequests, code: "function-busy", message: "[MS5503]: The service is busy. Try again later."},
		{name: "too many requests", problem: Problems{}.TooManyRequests(), status: http.StatusTooManyRequests, code: "too-many-requests", message: "[MS5504]: Too many attempts. Wait and try again."},
		{name: "unknown service", problem: Problems{}.UnknownService(), status: http.StatusNotFound, code: "unknown-service", message: "[MS5505]: No service has this name."},
		{name: "body too large", problem: Problems{}.BodyTooLarge(), status: http.StatusRequestEntityTooLarge, code: "body-too-large", message: "[MS5506]: The request body is too large."},
		{name: "storage failed", problem: Problems{}.StorageFailed(), status: http.StatusBadGateway, code: "upstream-error", message: "[MS5507]: The storage failed to answer. Try again later."},
		{name: "missing file", problem: Problems{}.MissingFile(), status: http.StatusNotFound, code: "not-found", message: "[MS5508]: Nothing is at this path."},
		{name: "missing object", problem: Problems{}.MissingObject(), status: http.StatusNotFound, code: "not-found", message: "[MS5509]: Nothing is at this path."},
		{name: "not found", problem: Problems{}.NotFound(), status: http.StatusNotFound, code: "not-found", message: "[MS5510]: Nothing is at this path."},
		{name: "bad path", problem: Problems{}.BadPath(), status: http.StatusBadRequest, code: "bad-path", message: "[MS5511]: This path is not allowed."},
		{name: "only get and head", problem: Problems{}.OnlyGetAndHead(), status: http.StatusMethodNotAllowed, code: "method-not-allowed", message: "[MS5512]: Only GET and HEAD are allowed here."},
		{name: "only get", problem: Problems{}.OnlyGet(), status: http.StatusMethodNotAllowed, code: "method-not-allowed", message: "[MS5513]: Only GET is allowed here."},
		{name: "only get and put", problem: Problems{}.OnlyGetAndPut(), status: http.StatusMethodNotAllowed, code: "method-not-allowed", message: "[MS5514]: Only GET and PUT are allowed here."},
		{name: "upstream failed", problem: Problems{}.UpstreamFailed(), status: http.StatusBadGateway, code: "upstream-error", message: "[MS5515]: The service failed to answer. Try again later."},
		{name: "passthrough failed", problem: Problems{}.PassthroughFailed(), status: http.StatusBadGateway, code: "upstream-error", message: "[MS5516]: Live updates are not available now. Try again later."},
		{name: "keycloak failed", problem: Problems{}.KeycloakFailed(), status: http.StatusBadGateway, code: "upstream-error", message: "[MS5517]: Sign-in is not available now. Try again later."},
	}
}

func wireProblem(t *testing.T, body []byte) map[string]string {
	t.Helper()
	var wire map[string]string
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	return wire
}

func TestEveryRouterAnswerCarriesItsNumber(t *testing.T) {
	for _, testCase := range numberedCases() {
		t.Run(testCase.name, func(t *testing.T) {
			reply := Problems{}.Reply(testCase.problem)
			wire := wireProblem(t, reply.Body)
			if reply.Status != testCase.status || len(wire) != 2 || wire["code"] != testCase.code || wire["message"] != testCase.message {
				t.Fatalf("got %d %s", reply.Status, reply.Body)
			}
		})
	}
}

func TestRouterNumbersAreListedOnceInTheRouterBlock(t *testing.T) {
	numbers := []int{}
	for _, testCase := range numberedCases() {
		numbers = append(numbers, testCase.problem.Number())
	}
	file, err := apierrors.ErrorsFile{}.Nearest(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Check(5500, numbers); err != nil {
		t.Fatal(err)
	}
}

func TestAPlainErrorIsTheUnexpectedNumber(t *testing.T) {
	reply := Problems{}.Reply(http.ErrHandlerTimeout)
	if reply.Status != http.StatusBadGateway || wireProblem(t, reply.Body)["message"] != "[MS5500]: Something went wrong." {
		t.Fatalf("got %d %s", reply.Status, reply.Body)
	}
}

func TestUnreachableNativeServiceIsNumbered(t *testing.T) {
	reply := call(t, handlerFor(t, nativeSettings(t, "http://127.0.0.1:1")), http.MethodGet, "/api/v1/identity/health", "")
	if reply.Code != http.StatusBadGateway || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5515]: The service failed to answer. Try again later." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestUnreachablePassthroughIsNumbered(t *testing.T) {
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Passthrough = map[string]string{"/api/v1/realtime/socket": "http://127.0.0.1:1"}
	reply := call(t, handlerFor(t, settings), http.MethodGet, "/api/v1/realtime/socket", "")
	if reply.Code != http.StatusBadGateway || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5516]: Live updates are not available now. Try again later." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestUnreachableKeycloakIsNumbered(t *testing.T) {
	fake := newFakeAws(t)
	reply := call(t, handlerFor(t, settingsFor(fake, "http://127.0.0.1:1")), http.MethodGet, "/idp/realms/example.com/.well-known/openid-configuration", "")
	if reply.Code != http.StatusBadGateway || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5517]: Sign-in is not available now. Try again later." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestUnreachableLambdaIsTheUnexpectedNumber(t *testing.T) {
	fake := newFakeAws(t)
	settings := settingsFor(fake, "")
	settings.LambdaEndpoint = "http://127.0.0.1:1"
	reply := call(t, handlerFor(t, settings), http.MethodGet, "/api/v1/identity/health", "")
	if reply.Code != http.StatusBadGateway || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5500]: Something went wrong." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func numberedHandler(t *testing.T) http.Handler {
	t.Helper()
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Blobs = BlobSettings{Folder: t.TempDir(), Base: "http://mesail.test/blobs", Secret: "0123456789abcdef0123456789abcdef"}
	settings.DevDns = devDnsSettings()
	settings.Blocked = []string{"/api/v1/realtime/gateway/"}
	return handlerFor(t, settings)
}

func TestRoutedAnswersCarryTheirNumbers(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		want   string
	}{
		{name: "missing file", method: http.MethodGet, target: "/missing.js", want: "[MS5508]: Nothing is at this path."},
		{name: "blocked path", method: http.MethodPost, target: "/api/v1/realtime/gateway/connect", want: "[MS5510]: Nothing is at this path."},
		{name: "bad path", method: http.MethodGet, target: "/a/..%2f..%2fsecret", want: "[MS5511]: This path is not allowed."},
		{name: "static post", method: http.MethodPost, target: "/", want: "[MS5512]: Only GET and HEAD are allowed here."},
		{name: "dev dns post", method: http.MethodPost, target: "/_dev/dns-query?name=example.test&type=MX", want: "[MS5513]: Only GET is allowed here."},
		{name: "blob delete", method: http.MethodDelete, target: "/blobs/example.com/file-1", want: "[MS5514]: Only GET and PUT are allowed here."},
	}
	handler := numberedHandler(t)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			reply := call(t, handler, testCase.method, testCase.target, "")
			if wireProblem(t, reply.Body.Bytes())["message"] != testCase.want {
				t.Fatalf("got %d %s", reply.Code, reply.Body.String())
			}
		})
	}
}

func TestMissingStorageObjectIsNumbered(t *testing.T) {
	fake := newFakeAws(t)
	reply := call(t, handlerFor(t, settingsFor(fake, "")), http.MethodGet, "/missing.js", "")
	if reply.Code != http.StatusNotFound || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5509]: Nothing is at this path." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestRateLimitedAnswerIsNumbered(t *testing.T) {
	fake := newFakeAws(t)
	settings := settingsFor(fake, "")
	settings.Limited = []string{"POST /api/v1/identity/session"}
	settings.RateLimit = 1
	handler := handlerFor(t, settings)
	call(t, handler, http.MethodPost, "/api/v1/identity/session", "")
	reply := call(t, handler, http.MethodPost, "/api/v1/identity/session", "")
	if reply.Code != http.StatusTooManyRequests || wireProblem(t, reply.Body.Bytes())["message"] != "[MS5504]: Too many attempts. Wait and try again." {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}
