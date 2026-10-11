package bridgeclient

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/fylo"
	"github.com/d31ma/caduceus/internal/platform/fylobridge"
)

type echo struct{ requests []map[string]any }

func (caller *echo) Call(_ context.Context, request map[string]any) (fylo.Response, error) {
	caller.requests = append(caller.requests, request)
	result, _ := json.Marshal(map[string]any{"op": request["op"]})
	return fylo.Response{ProtocolVersion: 1, OK: true, Operation: request["op"].(string), Result: result}, nil
}

// TestFramesAreAcceptedByTheBridge signs frames the real fylobridge verifies.
func TestFramesAreAcceptedByTheBridge(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	caller := &echo{}
	bridge := fylobridge.New(fylobridge.Options{
		Address: "127.0.0.1:0", Secret: secret, MaxFrameBytes: 1 << 20, RequestTimeout: 5 * time.Second,
		IncomingRoot: t.TempDir(), DataRoot: t.TempDir(),
	}, caller)
	listener, err := bridge.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = bridge.Serve(ctx, listener) }()

	client := New(listener.Addr().String(), secret)
	result, err := client.Call(ctx, map[string]any{"op": "createCollection", "collection": "emails", "kind": "document"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"op":"createCollection"}` || caller.requests[0]["collection"] != "emails" {
		t.Fatalf("result = %s requests = %v", result, caller.requests)
	}
	wrong := New(listener.Addr().String(), "ffffffffffffffffffffffffffffffff")
	if _, err := wrong.Call(ctx, map[string]any{"op": "handshake"}); err == nil {
		t.Fatal("bad signature accepted")
	} else if bridgeErr, ok := err.(*Error); !ok || bridgeErr.Code != "EAUTH" {
		t.Fatalf("err = %v", err)
	}
	if _, err := New("10.0.0.1:9876", secret).Call(ctx, map[string]any{"op": "handshake"}); err == nil {
		t.Fatal("non-loopback address accepted")
	}
}
