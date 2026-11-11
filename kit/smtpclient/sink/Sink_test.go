package sink

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"sync"
	"testing"
)

func startSink(t *testing.T, options Options) *Sink {
	t.Helper()
	server, err := Start(options)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	return server
}

func dial(t *testing.T, server *Sink) *smtp.Client {
	t.Helper()
	client, err := smtp.Dial(server.Address())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func send(client *smtp.Client, from string, recipients []string, body string) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		return err
	}
	return writer.Close()
}

func replyCode(t *testing.T, err error) int {
	t.Helper()
	var protocolError *textproto.Error
	if !errors.As(err, &protocolError) {
		t.Fatalf("want a protocol error, got %v", err)
	}
	return protocolError.Code
}

func selfSigned(t *testing.T) SelfSigned {
	t.Helper()
	certificates, err := NewSelfSigned()
	if err != nil {
		t.Fatalf("certificates: %v", err)
	}
	return certificates
}

func TestPlainDeliveryIsRecorded(t *testing.T) {
	server := startSink(t, Options{})
	client := dial(t, server)
	body := "Subject: hi\r\n\r\n.leading dot\r\nbody\r\n"
	if err := send(client, "a@example.test", []string{"b@example.test", "c@example.test"}, body); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := client.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	messages := server.Messages()
	if len(messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(messages))
	}
	got := messages[0]
	if got.From != "a@example.test" || strings.Join(got.Recipients, ",") != "b@example.test,c@example.test" {
		t.Fatalf("envelope: %+v", got)
	}
	if string(got.Data) != body {
		t.Fatalf("data: %q", got.Data)
	}
	if got.Tls || got.AuthUser != "" {
		t.Fatalf("want plain unauthenticated, got %+v", got)
	}
}

func TestPortMatchesAddress(t *testing.T) {
	server := startSink(t, Options{})
	_, port, _ := net.SplitHostPort(server.Address())
	if port == "" || !strings.HasSuffix(server.Address(), ":"+port) || server.Port() == 0 {
		t.Fatalf("address %s port %d", server.Address(), server.Port())
	}
}

func TestStartTlsAndAuth(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, Options{Tls: certificates.Server, Username: "relay", Password: "secret"})
	client := dial(t, server)
	if found, _ := client.Extension("AUTH"); found {
		t.Fatalf("AUTH must not be offered before TLS")
	}
	if found, _ := client.Extension("STARTTLS"); !found {
		t.Fatalf("STARTTLS must be offered")
	}
	if err := client.StartTLS(certificates.Client); err != nil {
		t.Fatalf("starttls: %v", err)
	}
	if found, _ := client.Extension("STARTTLS"); found {
		t.Fatalf("STARTTLS must not be offered twice")
	}
	if err := client.Auth(smtp.PlainAuth("", "relay", "secret", "127.0.0.1")); err != nil {
		t.Fatalf("auth: %v", err)
	}
	if err := send(client, "a@example.test", []string{"b@example.test"}, "x\r\n"); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := server.Messages()[0]
	if !got.Tls || got.AuthUser != "relay" {
		t.Fatalf("want TLS and auth user, got %+v", got)
	}
}

func TestWrongPasswordIsRefused(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, Options{Tls: certificates.Server, Username: "relay", Password: "secret"})
	client := dial(t, server)
	if err := client.StartTLS(certificates.Client); err != nil {
		t.Fatalf("starttls: %v", err)
	}
	err := client.Auth(smtp.PlainAuth("", "relay", "wrong", "127.0.0.1"))
	if replyCode(t, err) != 535 {
		t.Fatalf("want 535, got %v", err)
	}
}

func TestMailWithoutAuthIsRefused(t *testing.T) {
	server := startSink(t, Options{Username: "relay", Password: "secret"})
	text := rawSession(t, server)
	if code := command(t, text, "MAIL FROM:<a@example.test>"); code != 530 {
		t.Fatalf("want 530, got %d", code)
	}
}

func rawSession(t *testing.T, server *Sink) *textproto.Conn {
	t.Helper()
	network, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	text := textproto.NewConn(network)
	t.Cleanup(func() { text.Close() })
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("greeting: %v", err)
	}
	return text
}

func command(t *testing.T, text *textproto.Conn, line string) int {
	t.Helper()
	if err := text.PrintfLine("%s", line); err != nil {
		t.Fatalf("write: %v", err)
	}
	code, _, err := text.ReadResponse(0)
	if err != nil && code == 0 {
		t.Fatalf("read: %v", err)
	}
	return code
}

func TestAuthWithoutTlsIsRefused(t *testing.T) {
	server := startSink(t, Options{Username: "relay", Password: "secret"})
	text := rawSession(t, server)
	if code := command(t, text, "AUTH PLAIN AHJlbGF5AHNlY3JldA=="); code != 503 {
		t.Fatalf("want 503, got %d", code)
	}
}

func TestStartTlsWithoutConfigIsRefused(t *testing.T) {
	server := startSink(t, Options{})
	text := rawSession(t, server)
	if code := command(t, text, "STARTTLS"); code != 454 {
		t.Fatalf("want 454, got %d", code)
	}
}

func TestCommandOrderAndUnknownCommands(t *testing.T) {
	server := startSink(t, Options{})
	text := rawSession(t, server)
	steps := []struct {
		line string
		code int
	}{
		{"HELO client.test", 250},
		{"NOOP", 250},
		{"VRFY someone", 500},
		{"RCPT TO:<b@example.test>", 503},
		{"DATA", 503},
		{"MAIL FROM:<>", 250},
		{"DATA", 503},
		{"RSET", 250},
		{"RCPT TO:<b@example.test>", 503},
		{"QUIT", 221},
	}
	for _, step := range steps {
		if code := command(t, text, step.line); code != step.code {
			t.Fatalf("%s: want %d, got %d", step.line, step.code, code)
		}
	}
}

func TestRejectedRecipientGetsChosenReply(t *testing.T) {
	reject := func(address string) (int, string) {
		if address == "full@example.test" {
			return 452, "4.2.2 Mailbox full"
		}
		return 0, ""
	}
	server := startSink(t, Options{RejectRecipient: reject})
	client := dial(t, server)
	if err := client.Mail("a@example.test"); err != nil {
		t.Fatalf("mail: %v", err)
	}
	if replyCode(t, client.Rcpt("full@example.test")) != 452 {
		t.Fatalf("want 452")
	}
	if err := client.Rcpt("ok@example.test"); err != nil {
		t.Fatalf("rcpt: %v", err)
	}
}

func TestTooLargeMessageIsRefused(t *testing.T) {
	server := startSink(t, Options{MaxSize: 10})
	client := dial(t, server)
	err := send(client, "a@example.test", []string{"b@example.test"}, strings.Repeat("x", 40)+"\r\n")
	if replyCode(t, err) != 552 {
		t.Fatalf("want 552, got %v", err)
	}
	if len(server.Messages()) != 0 {
		t.Fatalf("message must not be stored")
	}
}

func TestDisconnectDuringDataStoresNothing(t *testing.T) {
	server := startSink(t, Options{})
	text := rawSession(t, server)
	command(t, text, "MAIL FROM:<a@example.test>")
	command(t, text, "RCPT TO:<b@example.test>")
	if code := command(t, text, "DATA"); code != 354 {
		t.Fatalf("want 354, got %d", code)
	}
	text.PrintfLine("partial")
	text.Close()
	server.Close()
	if len(server.Messages()) != 0 {
		t.Fatalf("message must not be stored")
	}
}

func TestFailedHandshakeClosesConnection(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, Options{Tls: certificates.Server})
	text := rawSession(t, server)
	if code := command(t, text, "STARTTLS"); code != 220 {
		t.Fatalf("want 220, got %d", code)
	}
	text.PrintfLine("not a tls hello")
	if _, err := bufio.NewReader(text.R).ReadString('\n'); err == nil {
		t.Fatalf("want closed connection")
	}
}

func TestCloseEndsIdleConnections(t *testing.T) {
	server, err := Start(Options{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	text := rawSession(t, server)
	if err := server.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := text.ReadLine(); err == nil {
		t.Fatalf("want closed connection")
	}
	if _, err := net.Dial("tcp", server.Address()); err == nil {
		t.Fatalf("want refused dial after close")
	}
}

func TestConcurrentDeliveries(t *testing.T) {
	server := startSink(t, Options{})
	var group sync.WaitGroup
	for index := 0; index < 8; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := smtp.SendMail(server.Address(), nil, "a@example.test", []string{"b@example.test"}, []byte("x\r\n")); err != nil {
				t.Errorf("send: %v", err)
			}
		}()
	}
	group.Wait()
	if len(server.Messages()) != 8 {
		t.Fatalf("want 8 messages, got %d", len(server.Messages()))
	}
}

func TestClientTlsConfigVerifiesSink(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, Options{Tls: certificates.Server})
	client := dial(t, server)
	if err := client.StartTLS(&tls.Config{ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}); err == nil {
		t.Fatalf("an unknown certificate must fail verification")
	}
}

func TestImplicitTls(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, Options{Tls: certificates.Server, ImplicitTls: true})
	network, err := tls.Dial("tcp", server.Address(), certificates.Client)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client, err := smtp.NewClient(network, "127.0.0.1")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()
	if found, _ := client.Extension("STARTTLS"); found {
		t.Fatalf("STARTTLS must not be offered on implicit TLS")
	}
	if err := send(client, "a@example.test", []string{"b@example.test"}, "x\r\n"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !server.Messages()[0].Tls {
		t.Fatalf("want TLS recorded")
	}
}
