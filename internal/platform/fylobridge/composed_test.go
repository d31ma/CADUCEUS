package fylobridge

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/d31ma/caduceus/internal/platform/fylo"
)

func TestAccountsBoundaryUsesItsOwnAllowlistAndDispatcher(t *testing.T) {
	dispatcher := &recordingDispatcher{result: map[string]bool{"ok": true}}
	bridge := New(Options{}, nil).WithAccounts(dispatcher, []string{"accounts.list"})
	response := callComposed(t, bridge, "accounts.list")
	if !response.OK || dispatcher.operation != "accounts.list" || string(dispatcher.payload) != `{"token":"valid"}` {
		t.Fatalf("accounts response = %#v dispatcher=%#v", response, dispatcher)
	}
	denied := callComposed(t, bridge, "accounts.unknown")
	if denied.OK || denied.Error == nil || denied.Error.Code != "EFORBIDDEN" {
		t.Fatalf("unknown accounts response = %#v", denied)
	}
}

func callComposed(t *testing.T, bridge *Bridge, operation string) fylo.Response {
	t.Helper()
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		bridge.serveAccounts(context.Background(), server, operation, map[string]any{
			"op": operation, "params": map[string]any{"token": "valid"},
		})
		_ = server.Close()
	}()
	line, err := bufio.NewReader(client).ReadBytes('\n')
	_ = client.Close()
	<-done
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var response fylo.Response
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

type recordingDispatcher struct {
	operation string
	payload   json.RawMessage
	result    any
}

func (dispatcher *recordingDispatcher) Dispatch(_ context.Context, operation string, payload json.RawMessage) (any, error) {
	dispatcher.operation, dispatcher.payload = operation, payload
	return dispatcher.result, nil
}
