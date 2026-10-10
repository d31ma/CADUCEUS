package smtpd

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/smtp"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestingCertificate answers a throwaway self-signed certificate.
func testingCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

type recorder struct {
	mu       sync.Mutex
	messages []string
	sessions []Session
}

func (r *recorder) onData(_ context.Context, raw []byte, exceeded bool, session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, base64.StdEncoding.EncodeToString(raw))
	r.sessions = append(r.sessions, *session)
	if exceeded {
		return &Reply{Code: 552, Message: "Message exceeds maximum permitted size"}
	}
	return nil
}

func configured(t *testing.T, name string, rec *recorder) Options {
	t.Helper()
	switch name {
	case "inbound":
		return Options{
			Name: "mx.test", Banner: "mx.test ESMTP CADUCEUS", Size: 64, AuthOptional: true,
			DisabledCommands: []string{"AUTH"}, HideSTARTTLS: true, DisableReverseLookup: true, OnData: rec.onData,
		}
	case "submission":
		return Options{
			Name: "smtp.test", Banner: "smtp.test ESMTP CADUCEUS submission", Size: 64,
			AuthMethods: []string{"PLAIN", "LOGIN"}, AllowInsecureAuth: true, DisableReverseLookup: true,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{testingCertificate(t)}},
			OnAuth: func(_ context.Context, auth Auth, _ *Session) (string, error) {
				return "", &Reply{Message: fmt.Sprintf("refused %s %s %s", auth.Method, auth.Username, auth.Password)}
			},
			OnData: rec.onData,
		}
	}
	t.Fatalf("unknown config %s", name)
	return Options{}
}

func start(t *testing.T, options Options) (*Server, string) {
	t.Helper()
	server := New(options)
	listener, err := server.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})
	return server, listener.Addr().String()
}

var replyEnd = regexp.MustCompile(`^\d{3}( |$)`)

func readReply(reader *bufio.Reader) []string {
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if line != "" {
				lines = append(lines, line)
			}
			return append(lines, "<closed>")
		}
		line = strings.TrimSuffix(line, "\r\n")
		lines = append(lines, line)
		if replyEnd.MatchString(line) {
			return lines
		}
	}
}

type exchange struct {
	Send  json.RawMessage `json:"send"`
	Reply []string        `json:"reply"`
}

type scenario struct {
	Config    string     `json:"config"`
	Name      string     `json:"name"`
	Early     bool       `json:"early"`
	Exchanges []exchange `json:"exchanges"`
	Messages  []string   `json:"messages"`
}

// TestMatchesNodeSMTPServer replays sessions recorded against the Node
// smtp-server package with the options the JavaScript CADUCEUS servers used
// (testdata/smtp-server-golden.json) and requires the same replies.
func TestMatchesNodeSMTPServer(t *testing.T) {
	encoded, err := os.ReadFile("testdata/smtp-server-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []scenario
	if err := json.Unmarshal(encoded, &scenarios); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scenarios {
		t.Run(sc.Config+"/"+sc.Name, func(t *testing.T) {
			rec := &recorder{}
			_, address := start(t, configured(t, sc.Config, rec))
			conn, err := net.Dial("tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			reader := bufio.NewReader(conn)
			for index, step := range sc.Exchanges {
				if string(step.Send) != "null" {
					var text string
					var raw struct {
						Raw string `json:"raw"`
					}
					if json.Unmarshal(step.Send, &text) == nil {
						text += "\r\n"
					} else if err := json.Unmarshal(step.Send, &raw); err == nil {
						text = raw.Raw
					}
					if _, err := io.WriteString(conn, text); err != nil {
						t.Fatalf("step %d: write: %v", index, err)
					}
				}
				got := readReply(reader)
				if !reflect.DeepEqual(got, step.Reply) {
					t.Fatalf("step %d (%s): reply %q, want %q", index, step.Send, got, step.Reply)
				}
			}
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if len(rec.messages) != len(sc.Messages) {
				t.Fatalf("messages = %d, want %d", len(rec.messages), len(sc.Messages))
			}
			for index, message := range sc.Messages {
				decoded, _ := base64.StdEncoding.DecodeString(message)
				if len(decoded) > 64 {
					continue // over the limit: the size reply is what counts
				}
				if rec.messages[index] != message {
					t.Fatalf("message %d = %q, want %q", index, rec.messages[index], message)
				}
			}
		})
	}
}

func TestParseAddressCommandMatchesNode(t *testing.T) {
	cases := []struct {
		name, command, address string
		args                   map[string]Arg
	}{
		{"mail from", "MAIL FROM: <a@xn--bcher-kva.test> SIZE=10 BODY=8BITMIME SMTPUTF8", "a@bücher.test",
			map[string]Arg{"SIZE": {Value: "10"}, "BODY": {Value: "8BITMIME"}, "SMTPUTF8": {Flag: true}}},
		{"rcpt to", "RCPT TO:<c@[IPv6:2001:db8:0::1]>", "c@[IPv6:2001:db8::1]", nil},
		{"mail from", "MAIL FROM:<>", "", nil},
		{"mail from", "MAIL FROM:<a@b.test> =x X=1=2 ENVID=a+2Bb", "a@b.test", map[string]Arg{"X": {Value: "1=2"}, "ENVID": {Value: "a+b"}}},
		{"rcpt to", "RCPT TO:<User@Example.TEST> NOTIFY=never", "User@Example.TEST", map[string]Arg{"NOTIFY": {Value: "never"}}},
	}
	for _, c := range cases {
		parsed, ok := parseAddressCommand(c.name, c.command)
		if !ok || parsed.Address != c.address || !reflect.DeepEqual(parsed.Args, c.args) {
			t.Fatalf("%s: got %+v %v", c.command, parsed, ok)
		}
	}
}

func TestEnvelopeAndDotUnstuffing(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "inbound", rec)
	options.Size = 1 << 20
	_, address := start(t, options)
	client, err := smtp.Dial(address)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Hello("Sender.Example"); err != nil {
		t.Fatal(err)
	}
	if err := client.Mail("from@sender.example"); err != nil {
		t.Fatal(err)
	}
	for _, rcpt := range []string{"one@d.test", "TWO@d.test", "two@d.test"} {
		if err := client.Rcpt(rcpt); err != nil {
			t.Fatal(err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(writer, "Subject: x\r\n\r\n.leading dot\r\nend\r\n")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_ = client.Quit()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	raw, _ := base64.StdEncoding.DecodeString(rec.messages[0])
	if string(raw) != "Subject: x\r\n\r\n.leading dot\r\nend\r\n" {
		t.Fatalf("raw = %q", raw)
	}
	session := rec.sessions[0]
	if session.HostNameAppearsAs != "sender.example" || session.Envelope.MailFrom.Address != "from@sender.example" {
		t.Fatalf("session = %+v", session)
	}
	if len(session.Envelope.RcptTo) != 2 || session.Envelope.RcptTo[1].Address != "two@d.test" {
		t.Fatalf("rcpt = %+v", session.Envelope.RcptTo)
	}
}

func TestStartTLSResetsSessionAndHidesFeature(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "submission", rec)
	_, address := start(t, options)
	client, err := smtp.Dial(address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		t.Fatal("STARTTLS not offered")
	}
	if err := client.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	// net/smtp sends a fresh EHLO after STARTTLS.
	if ok, _ := client.Extension("STARTTLS"); ok {
		t.Fatal("STARTTLS offered inside TLS")
	}
	if ok, params := client.Extension("AUTH"); !ok || params != "PLAIN LOGIN" {
		t.Fatalf("AUTH = %v %q", ok, params)
	}
	err = client.Auth(smtp.PlainAuth("", "user", "pass", "127.0.0.1"))
	if err == nil || !strings.Contains(err.Error(), "535") || !strings.Contains(err.Error(), "refused PLAIN user pass") {
		t.Fatalf("auth err = %v", err)
	}
}

func TestStartTLSDiscardsPipelinedPlaintext(t *testing.T) {
	rec := &recorder{}
	_, address := start(t, configured(t, "submission", rec))
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	readReply(reader)
	_, _ = io.WriteString(conn, "EHLO c.test\r\n")
	readReply(reader)
	// The injected NOOP rides along with STARTTLS and must never be answered.
	_, _ = io.WriteString(conn, "STARTTLS\r\nNOOP\r\n")
	if got := readReply(reader); got[0] != "220 Ready to start TLS" {
		t.Fatalf("reply = %q", got)
	}
	secure := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	if err := secure.Handshake(); err != nil {
		t.Fatal(err)
	}
	secureReader := bufio.NewReader(secure)
	_, _ = io.WriteString(secure, "MAIL FROM:<a@b.test>\r\n")
	if got := readReply(secureReader); got[0] != "503 Error: send HELO/EHLO first" {
		t.Fatalf("reply after TLS = %q", got)
	}
}

func TestImplicitTLS(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "submission", rec)
	options.Secure = true
	options.AllowInsecureAuth = false
	_, address := start(t, options)
	conn, err := tls.Dial("tcp", address, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Hello("c.test"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		t.Fatal("STARTTLS offered on implicit TLS")
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		t.Fatal("AUTH not offered")
	}
}

func TestAuthRequiresTLSWithoutInsecureAuth(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "submission", rec)
	options.AllowInsecureAuth = false
	_, address := start(t, options)
	conn, _ := net.Dial("tcp", address)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	readReply(reader)
	_, _ = io.WriteString(conn, "EHLO c.test\r\nAUTH PLAIN AHVzZXIAcGFzcw==\r\n")
	readReply(reader)
	if got := readReply(reader); got[0] != "538 Error: Must issue a STARTTLS command first" {
		t.Fatalf("reply = %q", got)
	}
}

func TestPartialLongLineAndIdleTimeout(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "inbound", rec)
	options.SocketTimeout = 300 * time.Millisecond
	_, address := start(t, options)

	conn, _ := net.Dial("tcp", address)
	reader := bufio.NewReader(conn)
	readReply(reader)
	_, _ = io.WriteString(conn, strings.Repeat("A", 5000))
	if got := readReply(reader); got[0] != "421 Error: Command line too long" {
		t.Fatalf("reply = %q", got)
	}
	conn.Close()

	idle, _ := net.Dial("tcp", address)
	defer idle.Close()
	idleReader := bufio.NewReader(idle)
	readReply(idleReader)
	if got := readReply(idleReader); got[0] != "421 Timeout - closing connection" {
		t.Fatalf("reply = %q", got)
	}
}

func TestCloseAnswersShuttingDown(t *testing.T) {
	rec := &recorder{}
	options := configured(t, "inbound", rec)
	options.CloseTimeout = 200 * time.Millisecond
	server := New(options)
	listener, err := server.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	conn, _ := net.Dial("tcp", listener.Addr().String())
	defer conn.Close()
	reader := bufio.NewReader(conn)
	readReply(reader)
	started := time.Now()
	if err := server.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := readReply(reader); got[0] != "421 Server shutting down" {
		t.Fatalf("reply = %q", got)
	}
	if time.Since(started) < 150*time.Millisecond {
		t.Fatal("close did not wait for the close timeout")
	}
	if err := <-served; err != nil {
		t.Fatalf("serve = %v", err)
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), 200*time.Millisecond); err == nil {
		t.Fatal("listener still accepting")
	}
}

func TestListenReportsAddressInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if _, err := New(Options{}).Listen(busy.Addr().String()); err == nil {
		t.Fatal("second bind succeeded")
	}
}

func TestPunycode(t *testing.T) {
	for input, want := range map[string]string{"bcher-kva": "bücher", "mnchen-3ya": "münchen", "wgv71a119e": "日本語"} {
		if got, ok := punycodeDecode(input); !ok || got != want {
			t.Fatalf("%s = %q %v", input, got, ok)
		}
	}
}
