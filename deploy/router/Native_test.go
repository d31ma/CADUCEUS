package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nativeSettings(t *testing.T, upstream string) Settings {
	t.Helper()
	web := t.TempDir()
	admin := t.TempDir()
	writeFile(t, filepath.Join(web, "index.html"), "web home")
	writeFile(t, filepath.Join(web, "app.wasm"), "wasm")
	writeFile(t, filepath.Join(admin, "index.html"), "admin home")
	if err := os.Mkdir(filepath.Join(web, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Settings{
		Timeout:       5 * time.Second,
		BodyLimit:     16,
		Upstreams:     map[string]string{"identity": upstream},
		StaticFolders: map[string]string{"web": web, "admin": admin},
	}
}

func writeFile(t *testing.T, name string, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNativeApiCallsAreProxiedToTheServiceServer(t *testing.T) {
	var seen *http.Request
	service := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = request
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"service":"identity","status":"ok"}`))
	}))
	defer service.Close()
	reply := call(t, handlerFor(t, nativeSettings(t, service.URL)), http.MethodGet, "/api/v1/identity/health?x=1", "")
	if reply.Code != http.StatusOK || reply.Body.String() != `{"service":"identity","status":"ok"}` || seen.URL.Path != "/api/v1/identity/health" || seen.URL.RawQuery != "x=1" || seen.Header.Get("X-Forwarded-Host") != "example.com" {
		t.Fatalf("got %d %s, service saw %+v", reply.Code, reply.Body.String(), seen)
	}
}

func TestNativeUnknownServiceIsMissing(t *testing.T) {
	reply := call(t, handlerFor(t, nativeSettings(t, "http://127.0.0.1:1")), http.MethodGet, "/api/v1/search/health", "")
	if reply.Code != http.StatusNotFound || !strings.Contains(reply.Body.String(), "unknown-service") {
		t.Fatalf("got %d %s", reply.Code, reply.Body.String())
	}
}

func TestNativeStaticFiles(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   int
		body   string
		kind   string
	}{
		{name: "web index", target: "/", want: http.StatusOK, body: "web home", kind: "text/html; charset=utf-8"},
		{name: "wasm", target: "/app.wasm", want: http.StatusOK, body: "wasm", kind: "application/wasm"},
		{name: "page falls back", target: "/conversations/1", want: http.StatusOK, body: "web home"},
		{name: "folder falls back", target: "/folder", want: http.StatusOK, body: "web home"},
		{name: "admin", target: "/admin/", want: http.StatusOK, body: "admin home"},
		{name: "missing asset", target: "/missing.js", want: http.StatusNotFound},
		{name: "unknown kind", target: "/missing.unknownkind", want: http.StatusNotFound},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			reply := call(t, handlerFor(t, nativeSettings(t, "http://127.0.0.1:1")), http.MethodGet, testCase.target, "")
			if reply.Code != testCase.want || !strings.HasPrefix(reply.Body.String(), testCase.body) || (testCase.kind != "" && reply.Header().Get("Content-Type") != testCase.kind) {
				t.Fatalf("got %d %q %s", reply.Code, reply.Header().Get("Content-Type"), reply.Body.String())
			}
		})
	}
}

func TestFolderStoreGivesUnknownKindsAsBytes(t *testing.T) {
	folder := t.TempDir()
	writeFile(t, filepath.Join(folder, "data.unknownkind"), "x")
	reply, err := FolderStore{Folders: map[string]string{"web": folder}}.Get(t.Context(), "web/data.unknownkind")
	if err != nil || reply.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("got %+v, %v", reply, err)
	}
}

func TestFolderStoreReportsAMissingFolder(t *testing.T) {
	_, err := FolderStore{Folders: map[string]string{}}.Get(t.Context(), "web/index.html")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestNativeSettings(t *testing.T) {
	values := map[string]string{
		"MESAIL_ROUTER_UPSTREAMS":      "identity=http://127.0.0.1:8101, mail-in=http://127.0.0.1:8109",
		"MESAIL_ROUTER_STATIC_FOLDERS": "web=/tmp/web,admin=/tmp/admin",
		"MESAIL_ROUTER_KEYCLOAK_URL":   "http://127.0.0.1:18180/",
	}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err != nil || settings.Upstreams["mail-in"] != "http://127.0.0.1:8109" || settings.StaticFolders["admin"] != "/tmp/admin" || settings.KeycloakUrl != "http://127.0.0.1:18180" || settings.LambdaEndpoint != "" {
		t.Fatalf("got %+v, %v", settings, err)
	}
}

func TestNativeSettingsRefuseBadNames(t *testing.T) {
	_, err := SettingsReader{Lookup: lookupIn(map[string]string{"MESAIL_ROUTER_UPSTREAMS": "Bad=http://x"})}.Read()
	if err == nil || !strings.Contains(err.Error(), "MESAIL_ROUTER_UPSTREAMS") {
		t.Fatalf("got %v", err)
	}
}

func TestNativeSettingsRefuseBadFolderNames(t *testing.T) {
	_, err := SettingsReader{Lookup: lookupIn(map[string]string{"MESAIL_ROUTER_UPSTREAMS": "identity=http://x", "MESAIL_ROUTER_STATIC_FOLDERS": "Web=/x"})}.Read()
	if err == nil || !strings.Contains(err.Error(), "MESAIL_ROUTER_STATIC_FOLDERS") {
		t.Fatalf("got %v", err)
	}
}

func TestNativeSettingsRefuseABadTimeout(t *testing.T) {
	_, err := SettingsReader{Lookup: lookupIn(map[string]string{"MESAIL_ROUTER_UPSTREAMS": "identity=http://x", "MESAIL_ROUTER_TIMEOUT_SECONDS": "0"})}.Read()
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestBadUpstreamAddressStopsTheAssembly(t *testing.T) {
	settings := nativeSettings(t, "http://[::1")
	if _, err := (Assembly{}).Handler(settings); err == nil {
		t.Fatal("want an error")
	}
}

func TestProgramStopsOnABadUpstream(t *testing.T) {
	t.Setenv("MESAIL_ROUTER_UPSTREAMS", "identity=http://[::1")
	if (Program{}).Run() != 1 {
		t.Fatal("want exit code 1")
	}
}
