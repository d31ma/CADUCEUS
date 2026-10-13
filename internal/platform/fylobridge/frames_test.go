package fylobridge

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/bridgeclient"
	"github.com/d31ma/caduceus/internal/platform/fylo"
)

type handshakeCaller struct{}

func (handshakeCaller) Call(_ context.Context, request map[string]any) (fylo.Response, error) {
	op, _ := request["op"].(string)
	return fylo.Response{ProtocolVersion: 1, OK: true, Operation: op, Result: json.RawMessage(`{"owner":"frames-test"}`)}, nil
}

func startFrameBridge(t *testing.T) (address, secret string) {
	t.Helper()
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address = "127.0.0.1:" + strconv.Itoa(reservation.Addr().(*net.TCPAddr).Port)
	_ = reservation.Close()
	secret = "frames-test-secret-0123456789abcdef"
	dir := t.TempDir()
	bridge := New(Options{
		Address: address, Secret: secret, MaxFrameBytes: 1 << 20,
		RequestTimeout: 5 * time.Second, IncomingRoot: dir, DataRoot: dir,
	}, handshakeCaller{})
	listener, err := bridge.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = bridge.Serve(ctx, listener); close(done) }()
	t.Cleanup(func() { cancel(); _ = listener.Close(); <-done })
	return address, secret
}

// The bridge accepts the frames of both senders: internal/platform/bridgeclient
// (the entry point and the operator commands) and the Go server's
// server/shared/boundary, which Tachyon compiles as package main and so cannot
// share code with internal/. This keeps the two implementations in step.
func TestBothBridgeClientsSendFramesTheBridgeAccepts(t *testing.T) {
	address, secret := startFrameBridge(t)

	result, err := bridgeclient.New(address, secret).Call(context.Background(), map[string]any{"op": "handshake"})
	if err != nil || !strings.Contains(string(result), `"frames-test"`) {
		t.Fatalf("bridgeclient handshake = %s, %v", result, err)
	}

	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH; cannot build the server boundary")
	}
	root := filepath.Join("..", "..", "..", "server", "shared")
	dir := t.TempDir()
	for _, source := range []string{"boundary/Boundary.go", "boundary/BoundaryError.go", "runtime/Settings.go"} {
		data, err := os.ReadFile(filepath.Join(root, source))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(source)), append([]byte("package main\n\n"), data...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	program := `package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	result, err := Boundary{}.FromSettings().Call(map[string]any{"op": "handshake"}, 1<<20, 5*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(string(result))
}
`
	files := map[string]string{"go.mod": "module boundarycheck\n\ngo 1.26\n", "main.go": program}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(goBinary, "run", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "FYLO_BRIDGE_ADDRESS="+address, "FYLO_BRIDGE_SECRET="+secret)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), `"frames-test"`) {
		t.Fatalf("server Boundary handshake: %v\n%s", err, output)
	}
}
