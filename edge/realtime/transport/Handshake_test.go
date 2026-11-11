package transport_test

import (
	"strings"
	"testing"
	"time"
)

func TestSocketSaysHelloFirst(t *testing.T) {
	harness := newHarness(t, defaults)
	connection, _ := harness.open()
	if frame := harness.readRaw(connection); frame != helloFrame {
		t.Fatalf("first frame %q", frame)
	}
}

func TestEveryNewSocketSaysHello(t *testing.T) {
	harness := newHarness(t, defaults)
	first, _ := harness.open()
	harness.readRaw(first)
	second, _ := harness.open()
	if frame := harness.readRaw(second); frame != helloFrame {
		t.Fatalf("second socket's first frame %q", frame)
	}
}

func TestSocketAsksForReauthBeforeTheDeadline(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(400 * time.Millisecond)
	connection, _ := harness.open()
	harness.readRaw(connection)
	started := time.Now()
	harness.sendAndWait(connection, `{"type":"auth","token":"t"}`)
	if frame := harness.readRaw(connection); frame != reauthFrame {
		t.Fatalf("expected reauth, got %q", frame)
	}
	if waited := time.Since(started); waited > 380*time.Millisecond {
		t.Fatalf("reauth came after %s, too close to the deadline", waited)
	}
}

func TestRenewalAfterReauthKeepsTheSocket(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(300 * time.Millisecond)
	connection, _ := harness.open()
	harness.readRaw(connection)
	harness.sendAndWait(connection, `{"type":"auth","token":"t"}`)
	harness.readRaw(connection)
	harness.backend.acceptFor(5 * time.Second)
	harness.sendAndWait(connection, `{"type":"auth","token":"fresh"}`)
	time.Sleep(500 * time.Millisecond)
	connectionId := harness.backend.connectsSnapshot()[0].ConnectionId
	if status := harness.push(connectionId, `{"type":"message.created"}`); status != 200 {
		t.Fatalf("push after renewal answered %d", status)
	}
}

func TestStreamSaysHelloFirst(t *testing.T) {
	harness := newHarness(t, defaults)
	stream := harness.openStream("?ticket=abc")
	stream.waitFor("data: " + helloFrame)
}

func TestWarmCompilesTheBackendWithAHarmlessConnect(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.gateway.Warm()
	connects := harness.backend.connectsSnapshot()
	if len(connects) != 1 || connects[0].ConnectionId != "gateway-warmup" || connects[0].Kind != "socket" {
		t.Fatalf("connects %+v", connects)
	}
	if !strings.Contains(harness.logs.String(), `"msg":"backend.warmed","ready":true`) {
		t.Fatalf("logs %s", harness.logs.String())
	}
}

func TestWarmLogsAFailedBackend(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.setConnectStatus(500)
	harness.gateway.Warm()
	if !strings.Contains(harness.logs.String(), `"ready":false`) || len(harness.backend.connectsSnapshot()) != 12 {
		t.Fatalf("logs %s", harness.logs.String())
	}
}
