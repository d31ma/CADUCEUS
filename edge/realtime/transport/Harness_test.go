package transport_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"mesail.dev/edge/realtime/backend"
	"mesail.dev/edge/realtime/connections"
	"mesail.dev/edge/realtime/transport"
)

const testSecret = "a-test-secret-that-is-long-enough-1234"
const allowedOrigin = "https://mail.example"
const waitLimit = 5 * time.Second
const helloFrame = `{"type":"hello","conversationId":"","messageId":"","state":""}`
const reauthFrame = `{"type":"reauth","conversationId":"","messageId":"","state":""}`

type FakeBackend struct {
	mutex         sync.Mutex
	server        *httptest.Server
	connects      []backend.ConnectRequest
	messages      []backend.MessageRequest
	disconnects   []backend.DisconnectRequest
	badSecrets    int
	connectStatus int
	reply         func(backend.MessageRequest) backend.Reply
}

func newFakeBackend(t *testing.T) *FakeBackend {
	fake := &FakeBackend{connectStatus: http.StatusNoContent}
	fake.reply = func(backend.MessageRequest) backend.Reply { return backend.Reply{} }
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *FakeBackend) serve(writer http.ResponseWriter, request *http.Request) {
	fake.mutex.Lock()
	if request.Header.Get("X-Realtime-Gateway") != testSecret {
		fake.badSecrets++
	}
	fake.mutex.Unlock()
	switch request.URL.Path {
	case "/api/v1/realtime/gateway/connect":
		fake.serveConnect(writer, request)
	case "/api/v1/realtime/gateway/message":
		fake.serveMessage(writer, request)
	case "/api/v1/realtime/gateway/disconnect":
		fake.serveDisconnect(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (fake *FakeBackend) serveConnect(writer http.ResponseWriter, request *http.Request) {
	payload := backend.ConnectRequest{}
	_ = json.NewDecoder(request.Body).Decode(&payload)
	fake.mutex.Lock()
	fake.connects = append(fake.connects, payload)
	status := fake.connectStatus
	fake.mutex.Unlock()
	writer.WriteHeader(status)
}

func (fake *FakeBackend) serveMessage(writer http.ResponseWriter, request *http.Request) {
	payload := backend.MessageRequest{}
	_ = json.NewDecoder(request.Body).Decode(&payload)
	fake.mutex.Lock()
	fake.messages = append(fake.messages, payload)
	reply := fake.reply
	fake.mutex.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(reply(payload))
}

func (fake *FakeBackend) serveDisconnect(writer http.ResponseWriter, request *http.Request) {
	payload := backend.DisconnectRequest{}
	_ = json.NewDecoder(request.Body).Decode(&payload)
	fake.mutex.Lock()
	fake.disconnects = append(fake.disconnects, payload)
	fake.mutex.Unlock()
	writer.WriteHeader(http.StatusNoContent)
}

func (fake *FakeBackend) setReply(reply func(backend.MessageRequest) backend.Reply) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.reply = reply
}

func (fake *FakeBackend) setConnectStatus(status int) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.connectStatus = status
}

func (fake *FakeBackend) acceptFor(lifetime time.Duration) {
	fake.setReply(func(backend.MessageRequest) backend.Reply {
		return backend.Reply{DomainId: "domain-1", ExpiresAt: time.Now().Add(lifetime).Format(time.RFC3339Nano)}
	})
}

func (fake *FakeBackend) connectsSnapshot() []backend.ConnectRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]backend.ConnectRequest(nil), fake.connects...)
}

func (fake *FakeBackend) messagesSnapshot() []backend.MessageRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]backend.MessageRequest(nil), fake.messages...)
}

func (fake *FakeBackend) disconnectsSnapshot() []backend.DisconnectRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]backend.DisconnectRequest(nil), fake.disconnects...)
}

func (fake *FakeBackend) badSecretCount() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.badSecrets
}

type SafeBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (safe *SafeBuffer) Write(data []byte) (int, error) {
	safe.mutex.Lock()
	defer safe.mutex.Unlock()
	return safe.buffer.Write(data)
}

func (safe *SafeBuffer) String() string {
	safe.mutex.Lock()
	defer safe.mutex.Unlock()
	return safe.buffer.String()
}

type Harness struct {
	t          *testing.T
	backend    *FakeBackend
	gateway    transport.Gateway
	public     *httptest.Server
	management *httptest.Server
	logs       *SafeBuffer
}

func testSessionSettings() connections.SessionSettings {
	return connections.SessionSettings{
		AuthWindow:        300 * time.Millisecond,
		PingInterval:      50 * time.Millisecond,
		KeepAliveInterval: 50 * time.Millisecond,
		WriteTimeout:      2 * time.Second,
		QueueLimit:        64,
		ReauthLead:        100 * time.Millisecond,
		WarmAttempts:      3,
		WarmPause:         time.Millisecond,
	}
}

func newHarness(t *testing.T, change func(*connections.SessionSettings)) *Harness {
	fake := newFakeBackend(t)
	session := testSessionSettings()
	change(&session)
	logs := &SafeBuffer{}
	settings := transport.GatewaySettings{
		BackendUrl:     fake.server.URL,
		BackendTimeout: 5 * time.Second,
		BackendRetry:   backend.RetryPolicy{Delays: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}},
		Secret:         testSecret,
		Origins:        []string{allowedOrigin, "https://other.example"},
		Session:        session,
	}
	gateway := transport.NewGateway(settings, connections.NewEventLog(logs))
	harness := &Harness{
		t:          t,
		backend:    fake,
		gateway:    gateway,
		public:     httptest.NewServer(gateway.Public()),
		management: httptest.NewServer(gateway.Management()),
		logs:       logs,
	}
	t.Cleanup(harness.stop)
	return harness
}

func defaults(*connections.SessionSettings) {}

func (harness *Harness) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	_ = harness.gateway.Close(ctx)
	harness.public.Close()
	harness.management.Close()
}

func (harness *Harness) socketUrl() string {
	return "ws" + strings.TrimPrefix(harness.public.URL, "http") + "/api/v1/realtime/socket"
}

func (harness *Harness) dial(origin string) (*websocket.Conn, *http.Response, error) {
	headers := http.Header{}
	if origin != "" {
		headers.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	return websocket.Dial(ctx, harness.socketUrl(), &websocket.DialOptions{HTTPHeader: headers})
}

func (harness *Harness) open() (*websocket.Conn, string) {
	before := len(harness.backend.connectsSnapshot())
	connection, _, err := harness.dial("")
	if err != nil {
		harness.t.Fatalf("dial: %v", err)
	}
	connection.SetReadLimit(1 << 22)
	harness.t.Cleanup(func() { _ = connection.CloseNow() })
	harness.eventually("connect call", func() bool { return len(harness.backend.connectsSnapshot()) > before })
	return connection, harness.backend.connectsSnapshot()[before].ConnectionId
}

func (harness *Harness) send(connection *websocket.Conn, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	err := connection.Write(ctx, websocket.MessageText, []byte(text))
	if err != nil {
		harness.t.Fatalf("write: %v", err)
	}
}

func (harness *Harness) sendAndWait(connection *websocket.Conn, text string) {
	before := len(harness.backend.messagesSnapshot())
	harness.send(connection, text)
	harness.eventually("message call", func() bool { return len(harness.backend.messagesSnapshot()) > before })
	time.Sleep(20 * time.Millisecond)
}

func (harness *Harness) read(connection *websocket.Conn) string {
	for {
		frame := harness.readRaw(connection)
		if frame != helloFrame && frame != reauthFrame {
			return frame
		}
	}
}

func (harness *Harness) readRaw(connection *websocket.Conn) string {
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	_, data, err := connection.Read(ctx)
	if err != nil {
		harness.t.Fatalf("read: %v", err)
	}
	return string(data)
}

func (harness *Harness) closeCode(connection *websocket.Conn) int {
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	for {
		_, _, err := connection.Read(ctx)
		if err != nil {
			return int(websocket.CloseStatus(err))
		}
	}
}

func (harness *Harness) manage(method string, path string, body string, secret string) int {
	request, err := http.NewRequest(method, harness.management.URL+path, strings.NewReader(body))
	if err != nil {
		harness.t.Fatalf("request: %v", err)
	}
	if secret != "" {
		request.Header.Set("X-Realtime-Gateway", secret)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		harness.t.Fatalf("manage: %v", err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}

func (harness *Harness) push(connectionId string, body string) int {
	return harness.manage(http.MethodPost, "/@connections/"+connectionId, body, testSecret)
}

func (harness *Harness) remove(connectionId string, query string) int {
	return harness.manage(http.MethodDelete, "/@connections/"+connectionId+query, "", testSecret)
}

func (harness *Harness) eventually(what string, condition func() bool) {
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	harness.t.Fatalf("timed out waiting for %s", what)
}

func (harness *Harness) disconnectFor(connectionId string) backend.DisconnectRequest {
	found := backend.DisconnectRequest{}
	harness.eventually("disconnect call", func() bool {
		for _, request := range harness.backend.disconnectsSnapshot() {
			if request.ConnectionId == connectionId {
				found = request
				return true
			}
		}
		return false
	})
	return found
}

type StreamClient struct {
	t        *testing.T
	response *http.Response
	lines    chan string
}

func (harness *Harness) openStream(query string) *StreamClient {
	response, err := http.Get(harness.public.URL + "/api/v1/realtime/stream" + query)
	if err != nil {
		harness.t.Fatalf("stream: %v", err)
	}
	client := &StreamClient{t: harness.t, response: response, lines: make(chan string, 1024)}
	harness.t.Cleanup(func() { _ = response.Body.Close() })
	go client.scan()
	return client
}

func (client *StreamClient) scan() {
	scanner := bufio.NewScanner(client.response.Body)
	for scanner.Scan() {
		client.lines <- scanner.Text()
	}
	close(client.lines)
}

func (client *StreamClient) next() string {
	timeout := time.After(waitLimit)
	for {
		select {
		case line, open := <-client.lines:
			if !open {
				client.t.Fatalf("stream ended")
			}
			if line != "" && line != ": keep-alive" && line != "data: "+helloFrame && line != "data: "+reauthFrame {
				return line
			}
		case <-timeout:
			client.t.Fatalf("timed out reading the stream")
		}
	}
}

func (client *StreamClient) waitFor(wanted string) {
	timeout := time.After(waitLimit)
	for {
		select {
		case line, open := <-client.lines:
			if !open {
				client.t.Fatalf("stream ended before %q", wanted)
			}
			if line == wanted {
				return
			}
		case <-timeout:
			client.t.Fatalf("timed out waiting for %q", wanted)
		}
	}
}
