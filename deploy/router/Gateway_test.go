package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
)

func TestRealtimeSocketAndStreamGoToTheGatewayAndTheRestToYon(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("gateway " + request.URL.Path))
	}))
	defer gateway.Close()
	yon := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("yon " + request.URL.Path))
	}))
	defer yon.Close()
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Upstreams["realtime"] = yon.URL
	settings.Passthrough = map[string]string{"/api/v1/realtime/socket": gateway.URL, "/api/v1/realtime/stream": gateway.URL}
	settings.Blocked = []string{"/api/v1/realtime/gateway/"}
	handler := handlerFor(t, settings)
	socket := call(t, handler, http.MethodGet, "/api/v1/realtime/socket", "")
	stream := call(t, handler, http.MethodGet, "/api/v1/realtime/stream?ticket=1", "")
	tickets := call(t, handler, http.MethodPost, "/api/v1/realtime/tickets", "")
	blocked := call(t, handler, http.MethodPost, "/api/v1/realtime/gateway/connect", "")
	if socket.Body.String() != "gateway /api/v1/realtime/socket" || stream.Body.String() != "gateway /api/v1/realtime/stream" || tickets.Body.String() != "yon /api/v1/realtime/tickets" || blocked.Code != http.StatusNotFound {
		t.Fatalf("socket %q, stream %q, tickets %q, blocked %d", socket.Body.String(), stream.Body.String(), tickets.Body.String(), blocked.Code)
	}
}

func TestStreamEventsArriveBeforeTheStreamEnds(t *testing.T) {
	release := make(chan struct{})
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("event: hello\n\n"))
		writer.(http.Flusher).Flush()
		<-release
	}))
	defer gateway.Close()
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Passthrough = map[string]string{"/api/v1/realtime/stream": gateway.URL}
	router := httptest.NewServer(handlerFor(t, settings))
	defer router.Close()
	response, err := http.Get(router.URL + "/api/v1/realtime/stream")
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	close(release)
	_ = response.Body.Close()
	if err != nil || line != "event: hello\n" {
		t.Fatalf("got %q, %v", line, err)
	}
}

func TestBadPassthroughAddressStopsTheAssembly(t *testing.T) {
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Passthrough = map[string]string{"/api/v1/realtime/socket": "http://[::1"}
	if _, err := (Assembly{}).Handler(settings); err == nil {
		t.Fatal("want an error")
	}
}

func TestSignedBlobLinksWorkThroughTheRouter(t *testing.T) {
	folder := t.TempDir()
	secret := strings.Repeat("s", 32)
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Blobs = BlobSettings{Folder: folder, Base: "http://mesail.test/blobs", Secret: secret}
	handler := handlerFor(t, settings)
	store := blobs.NewLocalFolder(folder, "http://mesail.test/blobs", blobs.NewLocalSigner([]byte(secret)))
	key := blobs.Key{DomainId: "example.com", Name: "file-1"}
	upload, err := store.SignUpload(key, blobs.UploadTerms{MediaType: "text/plain", Size: 5}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRequest(http.MethodPut, pathOf(t, upload), strings.NewReader("hello"))
	put.Header.Set("Content-Type", "text/plain")
	putReply := httptest.NewRecorder()
	handler.ServeHTTP(putReply, put)
	download, err := store.SignDownload(key, blobs.DownloadTerms{FileName: "a.txt", MediaType: "text/plain"}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	getReply := call(t, handler, http.MethodGet, pathOf(t, download), "")
	unsigned := call(t, handler, http.MethodGet, "/blobs/example.com/file-1", "")
	deleted := call(t, handler, http.MethodDelete, pathOf(t, download), "")
	body, _ := io.ReadAll(getReply.Body)
	if putReply.Code >= 300 || string(body) != "hello" || unsigned.Code < 400 || deleted.Code != http.StatusMethodNotAllowed {
		t.Fatalf("put %d %s, get %d %q, unsigned %d, delete %d", putReply.Code, putReply.Body.String(), getReply.Code, body, unsigned.Code, deleted.Code)
	}
}

func pathOf(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.RequestURI()
}

func TestBlobsAreMissingWithoutAFolder(t *testing.T) {
	reply := call(t, handlerFor(t, nativeSettings(t, "http://127.0.0.1:1")), http.MethodGet, "/blobs/example.com/file-1", "")
	if reply.Code != http.StatusNotFound {
		t.Fatalf("got %d", reply.Code)
	}
}

func TestBlobSettingsNeedABaseAndALongSecret(t *testing.T) {
	values := map[string]string{"MESAIL_ROUTER_UPSTREAMS": "identity=http://x", "MESAIL_ROUTER_BLOBS_FOLDER": "/tmp/blobs", "MESAIL_ROUTER_BLOBS_SECRET": "short"}
	_, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err == nil || !strings.Contains(err.Error(), "MESAIL_ROUTER_BLOBS_FOLDER") {
		t.Fatalf("got %v", err)
	}
}

func TestSettingsReadPassthroughAndBlockedPaths(t *testing.T) {
	values := map[string]string{
		"MESAIL_ROUTER_UPSTREAMS":   "realtime=http://127.0.0.1:8106",
		"MESAIL_ROUTER_PASSTHROUGH": "/api/v1/realtime/socket=http://127.0.0.1:8116,/api/v1/realtime/stream=http://127.0.0.1:8116",
	}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err != nil || settings.Passthrough["/api/v1/realtime/stream"] != "http://127.0.0.1:8116" || settings.Blocked[0] != "/api/v1/realtime/gateway/" {
		t.Fatalf("got %+v, %v", settings, err)
	}
}

func TestWebSocketUpgradeReachesTheGateway(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, buffer, err := http.NewResponseController(writer).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		_, _ = buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\nhello")
		_ = buffer.Flush()
	}))
	defer gateway.Close()
	settings := nativeSettings(t, "http://127.0.0.1:1")
	settings.Passthrough = map[string]string{"/api/v1/realtime/socket": gateway.URL}
	router := httptest.NewServer(handlerFor(t, settings))
	defer router.Close()
	connection, err := net.Dial("tcp", strings.TrimPrefix(router.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = connection.Write([]byte("GET /api/v1/realtime/socket HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"))
	reader := bufio.NewReader(connection)
	status, _ := reader.ReadString('\n')
	rest, _ := io.ReadAll(io.LimitReader(reader, 200))
	if !strings.HasPrefix(status, "HTTP/1.1 101") || !strings.HasSuffix(string(rest), "hello") {
		t.Fatalf("got %q %q", status, rest)
	}
}

func TestPassthroughRoutesPutTheLongestPrefixFirst(t *testing.T) {
	targets := map[string]string{
		"/api/v1/":                     "http://127.0.0.1:1",
		"/api/v1/realtime/":            "http://127.0.0.1:2",
		"/api/v1/realtime/socket":      "http://127.0.0.1:3",
		"/api/v1/realtime/stream":      "http://127.0.0.1:3",
		"/api/v1/realtime/socket/long": "http://127.0.0.1:4",
	}
	passthroughs, err := Passthroughs{}.Of(targets)
	if err != nil || len(passthroughs.Routes) != 5 || passthroughs.Routes[0].Prefix != "/api/v1/realtime/socket/long" || passthroughs.Routes[3].Prefix != "/api/v1/realtime/" || passthroughs.Routes[4].Prefix != "/api/v1/" {
		t.Fatalf("got %+v, %v", passthroughs.Routes, err)
	}
}

func TestRealtimePassthroughsInLambdaModeBeatTheFunctions(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("gateway " + request.URL.Path))
	}))
	defer gateway.Close()
	yon := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("yon " + request.URL.Path))
	}))
	defer yon.Close()
	fake := newFakeAws(t)
	settings := settingsFor(fake, "")
	settings.Passthrough = map[string]string{"/api/v1/realtime/": yon.URL, "/api/v1/realtime/socket": gateway.URL, "/api/v1/realtime/stream": gateway.URL}
	settings.Blocked = []string{"/api/v1/realtime/gateway/"}
	handler := handlerFor(t, settings)
	socket := call(t, handler, http.MethodGet, "/api/v1/realtime/socket", "")
	stream := call(t, handler, http.MethodGet, "/api/v1/realtime/stream?ticket=1", "")
	health := call(t, handler, http.MethodGet, "/api/v1/realtime/health", "")
	blocked := call(t, handler, http.MethodPost, "/api/v1/realtime/gateway/connect", "")
	if socket.Body.String() != "gateway /api/v1/realtime/socket" || stream.Body.String() != "gateway /api/v1/realtime/stream" || health.Body.String() != "yon /api/v1/realtime/health" || blocked.Code != http.StatusNotFound || len(fake.functions) != 0 {
		t.Fatalf("socket %q, stream %q, health %q, blocked %d, functions %v", socket.Body.String(), stream.Body.String(), health.Body.String(), blocked.Code, fake.functions)
	}
}

func TestLambdaSettingsNeedNoRealtimeServiceForTheRealtimePassthrough(t *testing.T) {
	values := map[string]string{
		"MESAIL_ROUTER_SERVICES":        "identity,directory",
		"MESAIL_ROUTER_LAMBDA_ENDPOINT": "http://floci:4566",
		"AWS_ACCESS_KEY_ID":             "test",
		"AWS_SECRET_ACCESS_KEY":         "test",
		"MESAIL_ROUTER_PASSTHROUGH":     "/api/v1/realtime/socket=http://realtime:8116,/api/v1/realtime/stream=http://realtime:8116,/api/v1/realtime/=http://realtime:8106",
	}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err != nil || settings.Passthrough["/api/v1/realtime/"] != "http://realtime:8106" || settings.Passthrough["/api/v1/realtime/socket"] != "http://realtime:8116" || len(settings.Services) != 2 || settings.Blocked[0] != "/api/v1/realtime/gateway/" {
		t.Fatalf("got %+v, %v", settings, err)
	}
}
