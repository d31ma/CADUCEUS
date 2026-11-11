package protocol

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestExtraConnectionsAreRefusedWith421(t *testing.T) {
	server := newHarness(t, func(settings *Settings) { settings.MaxConnections = 1 })
	first := dialRaw(t, server.address)
	first.expect("NOOP", 250, "2.0.0")
	second := dialRaw(t, server.address)
	if !strings.HasPrefix(second.greeting, "4.3.2") {
		t.Fatalf("want 421 4.3.2, got %q", second.greeting)
	}
}

func TestShutdownStopsAcceptingAndWaitsForSessions(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("QUIT", 221, "2.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := net.DialTimeout("tcp", server.address, time.Second); err == nil {
		t.Fatal("the listener still accepts connections")
	}
}

func TestShutdownClosesIdleSessionsAfterTheGracePeriod(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("NOOP", 250, "2.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := server.server.Shutdown(ctx); err == nil {
		t.Fatal("want the grace period error")
	}
	if _, err := client.text.ReadLine(); err == nil {
		t.Fatal("the idle session is still open")
	}
}

func TestServeReportsListenerErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(Settings{MaxConnections: 1})
	listener.Close()
	if err := server.Serve(failingListener{listener}); err == nil {
		t.Fatal("want an accept error")
	}
}

type failingListener struct {
	net.Listener
}

func (failingListener) Accept() (net.Conn, error) {
	return nil, &net.OpError{Op: "accept", Err: context.DeadlineExceeded}
}
