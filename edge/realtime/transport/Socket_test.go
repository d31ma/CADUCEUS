package transport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"mesail.dev/edge/realtime/connections"
)

const authExpiredFrame = `{"type":"auth.expired","conversationId":"","messageId":"","state":"4401"}`

func TestHealthAnswersOk(t *testing.T) {
	harness := newHarness(t, defaults)
	response, err := http.Get(harness.public.URL + "/api/v1/realtime/gateway-health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := map[string]string{}
	_ = json.NewDecoder(response.Body).Decode(&body)
	if response.StatusCode != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health: %d %v", response.StatusCode, body)
	}
}

func TestSocketRefusesUnknownOrigin(t *testing.T) {
	harness := newHarness(t, defaults)
	_, response, err := harness.dial("https://evil.example")
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %v %v", response, err)
	}
	if len(harness.backend.connectsSnapshot()) != 0 {
		t.Fatalf("connect must not be called for a refused origin")
	}
}

func TestSocketAcceptsListedOrigin(t *testing.T) {
	harness := newHarness(t, defaults)
	connection, _, err := harness.dial(allowedOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = connection.CloseNow()
}

func TestPlainRequestOnSocketPathIsRefused(t *testing.T) {
	harness := newHarness(t, defaults)
	response, err := http.Get(harness.public.URL + "/api/v1/realtime/socket")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode < 400 {
		t.Fatalf("expected a refusal, got %d", response.StatusCode)
	}
}

func TestSocketConnectSendsIdAndKind(t *testing.T) {
	harness := newHarness(t, defaults)
	_, connectionId := harness.open()
	connect := harness.backend.connectsSnapshot()[0]
	if connect.Kind != "socket" || len(connectionId) != 22 {
		t.Fatalf("connect: %+v", connect)
	}
	if harness.backend.badSecretCount() != 0 {
		t.Fatalf("backend calls must carry the secret")
	}
}

func TestSocketWithoutAuthSendsExpiredFrameThenCloses4401(t *testing.T) {
	harness := newHarness(t, defaults)
	connection, connectionId := harness.open()
	if frame := harness.read(connection); frame != authExpiredFrame {
		t.Fatalf("frame: %s", frame)
	}
	if code := harness.closeCode(connection); code != 4401 {
		t.Fatalf("close code %d", code)
	}
	if disconnect := harness.disconnectFor(connectionId); disconnect.DomainId != "" {
		t.Fatalf("disconnect: %+v", disconnect)
	}
}

func TestSocketForwardsFramesWithDomainId(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(time.Minute)
	connection, connectionId := harness.open()
	harness.sendAndWait(connection, `{"type":"auth","token":"secret-token"}`)
	harness.sendAndWait(connection, `{"type":"typing","conversationId":"c1"}`)
	messages := harness.backend.messagesSnapshot()
	if messages[0].ConnectionId != connectionId || messages[0].DomainId != "" || messages[0].Body != `{"type":"auth","token":"secret-token"}` {
		t.Fatalf("first message: %+v", messages[0])
	}
	if messages[1].DomainId != "domain-1" || messages[1].Body != `{"type":"typing","conversationId":"c1"}` {
		t.Fatalf("second message: %+v", messages[1])
	}
}

func TestAcceptedAuthStaysOpenUntilExpiresAt(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(900 * time.Millisecond)
	connection, connectionId := harness.open()
	harness.sendAndWait(connection, `{"type":"auth","token":"t"}`)
	time.Sleep(500 * time.Millisecond)
	if status := harness.push(connectionId, `{"type":"typing"}`); status != http.StatusOK {
		t.Fatalf("push status %d", status)
	}
	if frame := harness.read(connection); frame != `{"type":"typing"}` {
		t.Fatalf("frame: %s", frame)
	}
	if frame := harness.read(connection); frame != authExpiredFrame {
		t.Fatalf("frame: %s", frame)
	}
	if code := harness.closeCode(connection); code != 4401 {
		t.Fatalf("close code %d", code)
	}
	if disconnect := harness.disconnectFor(connectionId); disconnect.DomainId != "domain-1" {
		t.Fatalf("disconnect: %+v", disconnect)
	}
}

func TestRenewalMovesTheDeadline(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(700 * time.Millisecond)
	connection, connectionId := harness.open()
	harness.sendAndWait(connection, `{"type":"auth","token":"first"}`)
	time.Sleep(450 * time.Millisecond)
	harness.sendAndWait(connection, `{"type":"auth","token":"second"}`)
	time.Sleep(450 * time.Millisecond)
	if status := harness.push(connectionId, `{"type":"still-open"}`); status != http.StatusOK {
		t.Fatalf("push status %d", status)
	}
	if frame := harness.read(connection); frame != `{"type":"still-open"}` {
		t.Fatalf("frame: %s", frame)
	}
	if code := harness.closeCode(connection); code != 4401 {
		t.Fatalf("close code %d", code)
	}
}

func TestPushReachesOnlyTheTargetConnection(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(time.Minute)
	first, firstId := harness.open()
	second, secondId := harness.open()
	harness.sendAndWait(first, `{"type":"auth"}`)
	harness.sendAndWait(second, `{"type":"auth"}`)
	if status := harness.push(firstId, `{ "type" : "for-first" }`); status != http.StatusOK {
		t.Fatalf("push status %d", status)
	}
	if status := harness.push(secondId, `{"type":"for-second"}`); status != http.StatusOK {
		t.Fatalf("push status %d", status)
	}
	if frame := harness.read(first); frame != `{"type":"for-first"}` {
		t.Fatalf("first got %s", frame)
	}
	if frame := harness.read(second); frame != `{"type":"for-second"}` {
		t.Fatalf("second got %s", frame)
	}
}

func TestUnknownConnectionAnswersGone(t *testing.T) {
	harness := newHarness(t, defaults)
	if status := harness.push("unknown", `{"type":"x"}`); status != http.StatusGone {
		t.Fatalf("push status %d", status)
	}
	if status := harness.remove("unknown", ""); status != http.StatusGone {
		t.Fatalf("delete status %d", status)
	}
}

func TestDeleteClosesWithTheRequestedCode(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, connectionId := harness.open()
	if status := harness.remove(connectionId, "?code=4401"); status != http.StatusNoContent {
		t.Fatalf("delete status %d", status)
	}
	if code := harness.closeCode(connection); code != 4401 {
		t.Fatalf("close code %d", code)
	}
}

func TestDeleteWithoutCodeClosesNormally(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, connectionId := harness.open()
	if status := harness.remove(connectionId, ""); status != http.StatusNoContent {
		t.Fatalf("delete status %d", status)
	}
	if code := harness.closeCode(connection); code != 1000 {
		t.Fatalf("close code %d", code)
	}
}

func TestDeleteRefusesCodeOutsideTheAllowedRange(t *testing.T) {
	harness := newHarness(t, defaults)
	_, connectionId := harness.open()
	if status := harness.remove(connectionId, "?code=1013"); status != http.StatusBadRequest {
		t.Fatalf("status %d", status)
	}
}

func TestDeleteRefusesCodeThatIsNotANumber(t *testing.T) {
	harness := newHarness(t, defaults)
	if status := harness.remove("any", "?code=soon"); status != http.StatusBadRequest {
		t.Fatalf("status %d", status)
	}
}

func TestManagementRefusesWrongSecret(t *testing.T) {
	harness := newHarness(t, defaults)
	if status := harness.manage(http.MethodPost, "/@connections/any", `{}`, "wrong-secret"); status != http.StatusUnauthorized {
		t.Fatalf("status %d", status)
	}
}

func TestManagementRefusesMissingSecret(t *testing.T) {
	harness := newHarness(t, defaults)
	if status := harness.manage(http.MethodDelete, "/@connections/any", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("status %d", status)
	}
}

func TestPushRefusesBodyThatIsNotJson(t *testing.T) {
	harness := newHarness(t, defaults)
	if status := harness.push("any", "not json"); status != http.StatusBadRequest {
		t.Fatalf("status %d", status)
	}
}

func TestPushRefusesEventOver64KiB(t *testing.T) {
	harness := newHarness(t, defaults)
	event := `{"text":"` + strings.Repeat("a", 65536) + `"}`
	if status := harness.push("any", event); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", status)
	}
}

func TestBinaryFrameClosesWith1003(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, connectionId := harness.open()
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	_ = connection.Write(ctx, websocket.MessageBinary, []byte("binary"))
	if code := harness.closeCode(connection); code != 1003 {
		t.Fatalf("close code %d", code)
	}
	harness.disconnectFor(connectionId)
}

func TestFrameOver16KiBClosesWith1009(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, _ := harness.open()
	harness.send(connection, strings.Repeat("a", 16385))
	if code := harness.closeCode(connection); code != 1009 {
		t.Fatalf("close code %d", code)
	}
	if len(harness.backend.messagesSnapshot()) != 0 {
		t.Fatalf("an oversized frame must not be forwarded")
	}
}

func TestFrameOfExactly16KiBIsForwarded(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, _ := harness.open()
	harness.sendAndWait(connection, strings.Repeat("a", 16384))
	if body := harness.backend.messagesSnapshot()[0].Body; len(body) != 16384 {
		t.Fatalf("body length %d", len(body))
	}
}

func TestSlowReaderClosesWith1013(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) {
		settings.AuthWindow = time.Minute
		settings.PingInterval = time.Minute
		settings.WriteTimeout = 30 * time.Second
		settings.QueueLimit = 4
	})
	connection, connectionId := harness.open()
	event := `{"text":"` + strings.Repeat("a", 60000) + `"}`
	pushes := 0
	for harness.push(connectionId, event) == http.StatusOK {
		pushes++
		if pushes > 2000 {
			t.Fatalf("the queue never filled")
		}
	}
	if code := harness.closeCode(connection); code != 1013 {
		t.Fatalf("close code %d", code)
	}
}

func TestConnectFailureClosesSocketWith1011(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.setConnectStatus(http.StatusInternalServerError)
	connection, connectionId := harness.open()
	if code := harness.closeCode(connection); code != 1011 {
		t.Fatalf("close code %d", code)
	}
	harness.disconnectFor(connectionId)
}

func TestShutdownClosesWith1001AndDisconnects(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	connection, connectionId := harness.open()
	codes := make(chan int, 1)
	go func() { codes <- harness.closeCode(connection) }()
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	if err := harness.gateway.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if code := <-codes; code != 1001 {
		t.Fatalf("close code %d", code)
	}
	if len(harness.backend.disconnectsSnapshot()) != 1 || harness.backend.disconnectsSnapshot()[0].ConnectionId != connectionId {
		t.Fatalf("disconnects: %+v", harness.backend.disconnectsSnapshot())
	}
}

func TestConnectionAfterShutdownIsClosedAtOnce(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	_ = harness.gateway.Close(ctx)
	connection, _ := harness.open()
	if code := harness.closeCode(connection); code != 1001 {
		t.Fatalf("close code %d", code)
	}
}

func TestLogsLeaveOutSecretsAndFrames(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(time.Minute)
	connection, connectionId := harness.open()
	harness.sendAndWait(connection, `{"type":"auth","token":"very-private-token"}`)
	_ = harness.remove(connectionId, "")
	_ = harness.closeCode(connection)
	harness.disconnectFor(connectionId)
	harness.eventually("closed log line", func() bool { return strings.Contains(harness.logs.String(), "connection.closed") })
	logs := harness.logs.String()
	if strings.Contains(logs, "very-private-token") || strings.Contains(logs, testSecret) {
		t.Fatalf("logs leak private values: %s", logs)
	}
	if !strings.Contains(logs, connectionId) {
		t.Fatalf("logs must name the connection: %s", logs)
	}
}

func TestBackendFailureIsLoggedWithoutTheFrame(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.server.Close()
	connection, _, err := harness.dial("")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if code := harness.closeCode(connection); code != 1011 {
		t.Fatalf("close code %d", code)
	}
	harness.eventually("failure log line", func() bool { return strings.Contains(harness.logs.String(), "backend.failed") })
}
