package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (buffer *lockedBuffer) Write(p []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.Write(p)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.String()
}

// TestInboundSMTPCommandDeliversOneMessage starts the CLI's inbound listener
// on a high port and delivers one message with net/smtp.
func TestInboundSMTPCommandDeliversOneMessage(t *testing.T) {
	delivered := make(chan map[string]any, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Header.Get("X-Caduceus-Signature") != tickworker.Sign("cli-secret", body) {
			http.Error(w, "unsigned", http.StatusUnauthorized)
			return
		}
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		delivered <- payload
		_, _ = io.WriteString(w, `{"accepted":true}`)
	}))
	defer webhook.Close()
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	t.Setenv("INBOUND_WEBHOOK_SECRET", "cli-secret")
	t.Setenv("CADUCEUS_API_URL", webhook.URL)
	t.Setenv("SMTP_INBOUND_HOST", "127.0.0.1")
	t.Setenv("SMTP_INBOUND_PORT", fmt.Sprint(port))
	t.Setenv("MAIL_AUTH_VERIFY", "off")
	t.Setenv("MAIL_HOSTNAME", "mx.cli.test")
	t.Setenv("MAIL_TLS_KEY", "")
	t.Setenv("MAIL_TLS_CERT", "")
	t.Setenv("MAIL_TLS_KEY_FILE", "")
	t.Setenv("MAIL_TLS_CERT_FILE", "")

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr := &lockedBuffer{}, &lockedBuffer{}
	code := make(chan int, 1)
	go func() { code <- Run(ctx, []string{"inbound-smtp"}, stdout, stderr, buildinfo.Info{}) }()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(stdout.String(), "inbound SMTP server listening") {
		if time.Now().After(deadline) {
			t.Fatalf("listener did not start: %s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := smtp.SendMail(address, nil, "a@sender.test", []string{"b@hosted.test"}, []byte("Subject: cli\r\n\r\nhi\r\n")); err != nil {
		t.Fatal(err)
	}
	payload := <-delivered
	if payload["recipient"] != "b@hosted.test" || payload["subject"] != "cli" {
		t.Fatalf("payload = %v", payload)
	}
	cancel()
	if got := <-code; got != ExitSuccess {
		t.Fatalf("exit = %d: %s", got, stderr.String())
	}
}

func TestEntryCommandHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"entry", "help"}, &stdout, &stderr, buildinfo.Info{}); code != ExitSuccess || !strings.Contains(stdout.String(), "admin:create") {
		t.Fatalf("code = %d stdout = %q", code, stdout.String())
	}
}
