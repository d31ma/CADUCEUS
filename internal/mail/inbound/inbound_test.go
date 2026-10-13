package inbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/mail/smtpd"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

type delivery struct {
	Recipient   string `json:"recipient"`
	Sender      string `json:"sender"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	SpamVerdict string `json:"spamVerdict"`
	SpamScore   int    `json:"spamScore"`
	raw         string
}

type webhook struct {
	mu         sync.Mutex
	deliveries []delivery
	answer     func(delivery) (int, string)
}

func (hook *webhook) start(t *testing.T, secret string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.URL.Path != "/inbound/webhook" || request.Header.Get("X-Caduceus-Signature") != tickworker.Sign(secret, body) {
			http.Error(writer, "bad signature", http.StatusUnauthorized)
			return
		}
		var got delivery
		_ = json.Unmarshal(body, &got)
		got.raw = string(body)
		hook.mu.Lock()
		hook.deliveries = append(hook.deliveries, got)
		hook.mu.Unlock()
		status, answer := 200, `{"accepted":true}`
		if hook.answer != nil {
			status, answer = hook.answer(got)
		}
		writer.WriteHeader(status)
		_, _ = io.WriteString(writer, answer)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func session(rcpts ...string) *smtpd.Session {
	s := &smtpd.Session{RemoteAddress: "192.0.2.1", ClientHostname: "[192.0.2.1]", HostNameAppearsAs: "mx.sender.test"}
	s.Envelope.MailFrom = &smtpd.Address{Address: "bounce@sender.test"}
	for _, rcpt := range rcpts {
		s.Envelope.RcptTo = append(s.Envelope.RcptTo, smtpd.Address{Address: rcpt})
	}
	return s
}

const message = "From: Alice <alice@sender.test>\r\nSubject: =?UTF-8?Q?Caf=C3=A9?=\r\n\r\nhello\r\n"

func TestFansOutAndSigns(t *testing.T) {
	hook := &webhook{answer: func(d delivery) (int, string) {
		if strings.HasSuffix(d.Recipient, "@elsewhere.test") {
			return 200, `{"accepted":false,"reason":"domain not configured"}`
		}
		return 200, `{"accepted":true}`
	}}
	url := hook.start(t, "secret")
	server := &Server{options: Options{APIURL: url, Secret: "secret", Hostname: "mx.test", Client: http.DefaultClient, Stderr: io.Discard}}
	failure, err := server.ProcessMessage(context.Background(), []byte(message), session("a@hosted.test", "b@elsewhere.test"), false)
	if err != nil || failure != nil {
		t.Fatalf("failure = %v, err = %v", failure, err)
	}
	if len(hook.deliveries) != 2 {
		t.Fatalf("deliveries = %d", len(hook.deliveries))
	}
	first := hook.deliveries[0]
	if first.Sender != "bounce@sender.test" || first.Subject != "Café" || first.Body != message {
		t.Fatalf("delivery = %+v", first)
	}
	if !strings.HasPrefix(first.raw, `{"recipient":"a@hosted.test","sender":"bounce@sender.test","subject":"Café","body":`) || strings.Contains(first.raw, "spam") {
		t.Fatalf("payload = %s", first.raw)
	}
}

func TestRefusesUnhostedAndDefersFailures(t *testing.T) {
	hook := &webhook{answer: func(delivery) (int, string) {
		return 200, `{"accepted":false,"reason":"domain not configured"}`
	}}
	url := hook.start(t, "secret")
	server := &Server{options: Options{APIURL: url, Secret: "secret", Client: http.DefaultClient, Stderr: io.Discard}}
	failure, _ := server.ProcessMessage(context.Background(), []byte(message), session("x@elsewhere.test"), false)
	if failure == nil || failure.Code != 550 || failure.Message != "No such user here" {
		t.Fatalf("failure = %+v", failure)
	}
	// A rejected (but hosted) recipient still counts as hosted.
	hook.answer = func(delivery) (int, string) { return 200, `{"accepted":false,"reason":"spam"}` }
	if failure, _ := server.ProcessMessage(context.Background(), []byte(message), session("x@hosted.test"), false); failure != nil {
		t.Fatalf("failure = %+v", failure)
	}
	hook.answer = func(delivery) (int, string) { return 500, `oops` }
	failure, _ = server.ProcessMessage(context.Background(), []byte(message), session("x@hosted.test"), false)
	if failure == nil || failure.Code != 451 {
		t.Fatalf("failure = %+v", failure)
	}
	hook.answer = func(delivery) (int, string) { return 413, `Request body is too large.` }
	failure, _ = server.ProcessMessage(context.Background(), []byte(message), session("x@hosted.test"), false)
	if failure == nil || failure.Code != 552 {
		t.Fatalf("a webhook 413 must refuse permanently: %+v", failure)
	}
	failure, _ = server.ProcessMessage(context.Background(), []byte(message), session("x@hosted.test"), true)
	if failure == nil || failure.Code != 552 || failure.Message != "Message exceeds maximum permitted size" {
		t.Fatalf("failure = %+v", failure)
	}
}

func TestAuthenticationResultsAndSpamSignal(t *testing.T) {
	hook := &webhook{}
	url := hook.start(t, "secret")
	var input AuthInput
	outcome := &AuthOutcome{Headers: "Authentication-Results: mx.test;\r\n dmarc=fail\r\n", SPF: "pass", DMARC: "fail"}
	server := &Server{options: Options{
		APIURL: url, Secret: "secret", Hostname: "mx.test", Client: http.DefaultClient, Stderr: io.Discard, VerifyAuth: true,
		Authenticate: func(_ context.Context, _ []byte, in AuthInput) (*AuthOutcome, error) {
			input = in
			return outcome, nil
		},
	}}
	sess := session("a@hosted.test")
	if failure, err := server.ProcessMessage(context.Background(), []byte(message), sess, false); failure != nil || err != nil {
		t.Fatal(failure, err)
	}
	got := hook.deliveries[0]
	if got.SpamVerdict != "fail" || got.SpamScore != 6 || !strings.HasPrefix(got.Body, outcome.Headers+"From: Alice") {
		t.Fatalf("delivery = %+v", got)
	}
	if input != (AuthInput{IP: "192.0.2.1", Helo: "mx.sender.test", Sender: "bounce@sender.test", MTA: "mx.test"}) {
		t.Fatalf("input = %+v", input)
	}

	outcome = &AuthOutcome{SPF: "fail"}
	_, _ = server.ProcessMessage(context.Background(), []byte(message), sess, false)
	if got := hook.deliveries[1]; got.SpamVerdict != "" || got.SpamScore != 3 {
		t.Fatalf("delivery = %+v", got)
	}

	var logs bytes.Buffer
	server.options.Stderr = &logs
	server.options.Authenticate = func(context.Context, []byte, AuthInput) (*AuthOutcome, error) {
		return nil, errors.New("DNS timeout")
	}
	_, _ = server.ProcessMessage(context.Background(), []byte(message), sess, false)
	if got := hook.deliveries[2]; got.Body != message || got.SpamScore != 0 {
		t.Fatalf("delivery = %+v", got)
	}
	if !strings.Contains(logs.String(), "[smtp-in] mailauth verification failed (delivering unverified): DNS timeout") {
		t.Fatalf("logs = %q", logs.String())
	}
}

// TestSMTPSmoke starts the MX on a high port and delivers one message with
// net/smtp end to end.
func TestSMTPSmoke(t *testing.T) {
	hook := &webhook{}
	url := hook.start(t, "smoke-secret")
	var stdout bytes.Buffer
	server, err := Start(Options{
		Port: 0, Host: "127.0.0.1", APIURL: url, Secret: "smoke-secret", Hostname: "mx.smoke.test",
		MaxSize: 1 << 20, Stdout: &stdout, Stderr: io.Discard,
		SMTP: func(options *smtpd.Options) { options.DisableReverseLookup = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Close(ctx)
	}()
	if !strings.Contains(stdout.String(), "[smtp-in] inbound SMTP server listening on 127.0.0.1:0 → "+url+"/inbound/webhook (STARTTLS off)") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	err = smtp.SendMail(server.Addr().String(), nil, "alice@sender.test", []string{"bob@hosted.test"},
		[]byte("From: alice@sender.test\r\nTo: bob@hosted.test\r\nSubject: smoke\r\n\r\nhello over SMTP\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	hook.mu.Lock()
	defer hook.mu.Unlock()
	if len(hook.deliveries) != 1 {
		t.Fatalf("deliveries = %d", len(hook.deliveries))
	}
	got := hook.deliveries[0]
	if got.Recipient != "bob@hosted.test" || got.Sender != "alice@sender.test" || got.Subject != "smoke" || !strings.HasSuffix(got.Body, "hello over SMTP\r\n") {
		t.Fatalf("delivery = %+v", got)
	}
}

func TestSecretRequired(t *testing.T) {
	t.Setenv("INBOUND_WEBHOOK_SECRET", "")
	if _, err := FromSettings(""); err == nil || err.Error() != "INBOUND_WEBHOOK_SECRET is required to run the inbound SMTP server" {
		t.Fatalf("err = %v", err)
	}
}
